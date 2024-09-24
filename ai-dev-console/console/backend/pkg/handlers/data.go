/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
 */

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"sort"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/resource_utils"

	prommodel "github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/fields"
	apitypes "k8s.io/apimachinery/pkg/types"
	resources "k8s.io/apiserver/pkg/quota/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtime "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ResourceGPU   = "nvidia.com/gpu"
	IndexNodeName = "spec.nodeName"
	IndexPhase    = "status.phase"
	GPUType       = "aliyun.accelerator/nvidia_name"
)

func NewDataHandler() *DataHandler {
	clientmgr.IndexField(&corev1.Pod{}, IndexNodeName, func(obj runtime.Object) []string {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return []string{}
		}
		if len(pod.Spec.NodeName) == 0 {
			return []string{}
		}
		return []string{pod.Spec.NodeName}
	})

	clientmgr.IndexField(&corev1.Pod{}, IndexPhase, func(obj runtime.Object) []string {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			return []string{}
		}
		return []string{string(pod.Status.Phase)}
	})

	return &DataHandler{client: clientmgr.GetCtrlClient()}
}

type DataHandler struct {
	client client.Client
}

// sum all nodes allocatable resource(cpu/memory/gpu)
func (ov *DataHandler) GetClusterTotalResource() (model.ClusterTotalResource, error) {
	nodes := &corev1.NodeList{}
	err := ov.client.List(context.TODO(), nodes)
	if err != nil {
		klog.Errorf("GetClusterTotalResource Failed to list nodes")
		return model.ClusterTotalResource{}, err
	}
	totalResource := corev1.ResourceList{}
	for _, node := range nodes.Items {
		if strings.HasPrefix(node.Name, "virtual-kubelet") {
			continue
		}
		allocatable := node.Status.Allocatable.DeepCopy()
		totalResource = resources.Add(totalResource, allocatable)
	}
	clusterTotal := model.ClusterTotalResource{
		TotalCPU:    totalResource.Cpu().MilliValue(),
		TotalMemory: totalResource.Memory().Value(),
		TotalGPU:    getGpu(totalResource).MilliValue()}
	return clusterTotal, nil
}

// sum all pods request resource(cpu/memory/gpu)
func (ov *DataHandler) GetClusterRequestResource(podPhase string) (model.ClusterRequestResource, error) {
	namespaces := &corev1.NamespaceList{}
	err := ov.client.List(context.TODO(), namespaces)
	if err != nil {
		klog.Errorf("GetClusterRequestResource Failed to list namespaces")
		return model.ClusterRequestResource{}, err
	}
	totalRequest := corev1.ResourceList{}
	for _, namespace := range namespaces.Items {
		// query pod list in namespace
		podList := &corev1.PodList{}
		err = ov.client.List(context.TODO(), podList, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(IndexPhase, podPhase),
			Namespace:     namespace.Name})
		if err != nil {
			klog.Errorf("GetClusterRequestResource Failed to get pod list on node: %v error: %v", namespace.Name, err)
			return model.ClusterRequestResource{}, err
		}
		totalRequest = resources.Add(totalRequest, getPodRequest(podList, corev1.PodPhase(podPhase)))
	}
	clusterRequest := model.ClusterRequestResource{
		RequestCPU:    totalRequest.Cpu().MilliValue(),
		RequestMemory: totalRequest.Memory().Value(),
		RequestGPU:    getGpu(totalRequest).MilliValue()}
	return clusterRequest, nil
}

// sum all pods allocatable and request resource(cpu/memory/gpu) in a node
func (ov *DataHandler) GetClusterNodeInfos(searchParam string) (model.ClusterNodeInfoList, error) {
	nodes := &corev1.NodeList{}
	listOptions := client.ListOptions{}

	if searchParam != "" {
		searchLabel := make(map[string]string)
		if strings.Contains(searchParam, ":") {
			arr := strings.Split(searchParam, ":")
			searchLabel[arr[0]] = arr[1]
			listOptions.LabelSelector = labels.SelectorFromSet(searchLabel)
		} else {
			searchLabel["kubernetes.io/hostname"] = searchParam
			//listOptions.FieldSelector = fields.OneTermEqualSelector("metadata.name", searchParam)
		}
		listOptions.LabelSelector = labels.SelectorFromSet(searchLabel)
	}

	err := ov.client.List(context.TODO(), nodes, &listOptions)
	if err != nil {
		klog.Errorf("GetClusterTotalResource Failed to list nodes")
		return model.ClusterNodeInfoList{}, err
	}
	var clusterNodeInfos []model.ClusterNodeInfo
	for _, node := range nodes.Items {
		// query pod list in node
		podList := &corev1.PodList{}
		err = ov.client.List(context.TODO(), podList, &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(IndexNodeName, node.Name),
		})
		if err != nil {
			klog.Errorf("GetClusterNodeInfos Failed to get pod list on node: %v error: %v", node.Name, err)
			return model.ClusterNodeInfoList{}, err
		}
		podsRequest := getPodRequest(podList, corev1.PodRunning)
		allocatable := node.Status.Allocatable.DeepCopy()

		clusterNodeInfo := model.ClusterNodeInfo{
			NodeName:      node.Name,
			InstanceType:  getInstanceType(&node),
			GPUType:       node.Labels[GPUType],
			TotalCPU:      allocatable.Cpu().MilliValue(),
			TotalMemory:   allocatable.Memory().Value(),
			TotalGPU:      getGpu(allocatable).MilliValue(),
			RequestCPU:    podsRequest.Cpu().MilliValue(),
			RequestMemory: podsRequest.Memory().Value(),
			RequestGPU:    getGpu(podsRequest).MilliValue(),
		}
		clusterNodeInfos = append(clusterNodeInfos, clusterNodeInfo)
	}

	// sort by node name
	sort.SliceStable(clusterNodeInfos, func(i, j int) bool {
		return clusterNodeInfos[i].NodeName > clusterNodeInfos[j].NodeName
	})

	return model.ClusterNodeInfoList{Items: clusterNodeInfos}, nil
}

// query data from prometheus(arms)
// prometheus Range Vector Selectors https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
func (ov *DataHandler) QueryRange(query, start, end, step string) (prommodel.Value, error) {
	armsInfo, err := GetArmsInfo()
	if err != nil {
		return nil, err
	}
	if armsInfo.UserId == "" || armsInfo.ArmsUrl == "" || armsInfo.ArmsRegion == "" || armsInfo.ClusterId == "" {
		klog.Errorf("armsInfo property lack armsInfo:%v", armsInfo)
		return nil, nil
	}

	reqUrl := armsInfo.ArmsUrl + "api/v1/arms/" + armsInfo.UserId + "/" + armsInfo.ClusterId + "/" + armsInfo.ArmsRegion + "/api/v1/query_range?query=" + query + "&start=" + start + "&end=" + end + "&step=" + step
	klog.Infof("QueryRange url=%s", reqUrl)

	resp, err := http.Get(reqUrl)
	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()
	if err != nil || resp.StatusCode != http.StatusOK {
		klog.Errorf("QueryRange http err=%v,resp=%v,reqUrl=%s", err, resp, reqUrl)
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	result, err := getArmsValueFromResponse(body)
	if err != nil {
		return nil, err
	}

	return prommodel.Value(result), err
}

// get data from Arms http response
func getArmsValueFromResponse(body []byte) (prommodel.Matrix, error) {
	res := struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data,omitempty"`
		Error  string          `json:"error,omitempty"`
	}{}
	err := json.Unmarshal(body, &res)
	if err != nil {
		return nil, err
	}
	if res.Status != "success" {
		klog.Errorf("getArmsValueFromResponse err=%v", res.Error)
		return nil, errors.New("arms query error")
	}

	v := struct {
		Type   prommodel.ValueType `json:"resultType"`
		Result prommodel.Matrix    `json:"result"`
	}{}

	err = json.Unmarshal(res.Data, &v)
	if err != nil {
		return nil, err
	}

	return v.Result, err
}

// sum podlist request
func getPodRequest(podList *corev1.PodList, phase corev1.PodPhase) corev1.ResourceList {
	totalRequest := corev1.ResourceList{}
	for _, pod := range podList.Items {
		if pod.Status.Phase != phase {
			continue
		}
		totalRequest = resources.Add(totalRequest, resource_utils.ComputePodResourceRequest(&pod))
	}
	return totalRequest
}

// get node instance type ,get from labels compatible
func getInstanceType(node *corev1.Node) string {
	instanceType := node.Labels["node.kubernetes.io/instance-type"]
	if instanceType == "" {
		instanceType = node.Labels["beta.kubernetes.io/instance-type"]
	}
	return instanceType
}

// get gpu from custom resource map
func getGpu(resourceList corev1.ResourceList) *resource.Quantity {
	if val, ok := (resourceList)[ResourceGPU]; ok {
		return &val
	}
	return &resource.Quantity{Format: resource.DecimalSI}
}

// get arms config for query data
func GetArmsInfo() (model.ArmsInfo, error) {
	// Get config map.
	configMap := &v1.ConfigMap{}
	var err = clientmgr.GetCtrlClient().Get(context.TODO(),
		apitypes.NamespacedName{
			Namespace: constants.SystemNamespace,
			Name:      constants.SystemConfigName,
		}, configMap)
	if err != nil {
		klog.Errorf("oauth failed get oauth configMap, ns: %s, name: %s, err: %v", constants.SystemNamespace, configMap, err)
		return model.ArmsInfo{}, err
	}
	armsConfig := configMap.Data["armsConfig"]
	oauthConfig := configMap.Data["oauthConfig"]
	armsMap := map[string]string{}
	err = json.Unmarshal([]byte(armsConfig), &armsMap)
	if err != nil {
		klog.Errorf("GetArmsInfo json Unmarshal err, armsConfig: %s, err: %v", armsConfig, err)
		return model.ArmsInfo{}, err
	}
	oauthMap := map[string]string{}
	err = json.Unmarshal([]byte(oauthConfig), &oauthMap)
	if err != nil {
		klog.Errorf("GetArmsInfo json Unmarshal err, oauthConfig: %s, err: %v", oauthConfig, err)
		return model.ArmsInfo{}, err
	}
	return model.ArmsInfo{ClusterId: armsMap["clusterId"], ArmsUrl: armsMap["armsUrl"], ArmsRegion: armsMap["armsRegion"], UserId: oauthMap["aid"]}, nil
}
