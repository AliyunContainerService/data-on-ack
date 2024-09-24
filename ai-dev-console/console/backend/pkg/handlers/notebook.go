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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	utils2 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo/converters"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/proxy"
	"github.com/ghodss/yaml"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
	"os"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strings"
	"time"
)

type ConditionsSorted []corev1.PodCondition

func (c ConditionsSorted) Len() int      { return len(c) }
func (c ConditionsSorted) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c ConditionsSorted) Less(i, j int) bool {
	return c[i].LastTransitionTime.Time.Before(c[j].LastTransitionTime.Time)
}

type NotebookHandler struct {
	client         client.Client
	routeCache     *proxy.SyncMapCache
	storageBackend backends.ObjectStorageBackend
	dynamicClient  dynamic.Interface
}

const (
	Notebook_Type_Jupyter = "Jupyter"
	Notebook_Type_VSCOde  = "VSCode"

	Notebook_Type_Label = "notebook-type"
)

func NewNotebookHandler(objStorage string) (*NotebookHandler, error) {
	objBackend := registry.GetObjectBackend(objStorage)
	if objBackend == nil {
		return nil, fmt.Errorf("no object backend storage named: %s", objStorage)
	}
	err := objBackend.Initialize()

	if err != nil {
		return nil, err
	}
	return &NotebookHandler{
		storageBackend: objBackend,
		client:         clientmgr.GetCtrlClient(),
		routeCache:     proxy.NewSyncMapCache(reflect.TypeOf("")),
		dynamicClient:  dynamic.NewForConfigOrDie(ctrl.GetConfigOrDie()),
	}, nil
}

type NotebookMessage struct {
	Name       string   `json:"name"`
	Namespace  string   `json:"namespace"`
	Image      string   `json:"image"`
	Volumes    []string `json:"volumes"`
	Age        string   `json:"age"`
	AccessPath string   `json:"accessPath"`
	Cpus       string   `json:"cpus"`
	Gpus       string   `json:"gpus"`
	Memory     string   `json:"memory"`
	Status     string   `json:"status"`
	Event      string   `json:"event"`
	Token      string   `json:"token"`
	ErrMessage string   `json:"errMessage"`
}

type NotebookMessages []NotebookMessage

func (nb NotebookMessages) Len() int {
	return len(nb)
}

func (nb NotebookMessages) Swap(i, j int) {
	nb[i], nb[j] = nb[j], nb[i]
}

func (nb NotebookMessages) Less(i, j int) bool {
	return nb[i].Name < nb[j].Name
}

func (nh *NotebookHandler) GetNotebookServiceConnection(namespace, name string) (string, error) {
	cacheKey := utils2.GetCacheKey(namespace, name, utils2.NotebookSvc)

	value, ok := nh.routeCache.Get(cacheKey)
	if ok {
		return value.(string), nil
	} else {
		svc := &corev1.Service{}
		err := nh.client.Get(context.Background(), client.ObjectKey{
			Name:      name,
			Namespace: namespace,
		}, svc)
		if err != nil {
			klog.Errorf("Get notebook svc from apiServer err : %s", err.Error())
			return "", err
		}

		err = nh.routeCache.Store(cacheKey, svc.Spec.ClusterIP)
		if err != nil {
			klog.Errorf("The value type is invalid : %s", err.Error())
			return "", err
		}

		return svc.Spec.ClusterIP, nil
	}
}

func (nh *NotebookHandler) GetNotebookPodConnection(namespace, name string) (string, error) {
	cacheKey := utils2.GetCacheKey(namespace, name, utils2.NotebookPod)

	value, ok := nh.routeCache.Get(cacheKey)
	if ok {
		return value.(string), nil
	} else {
		pod := &corev1.Pod{}
		err := nh.client.Get(context.Background(), client.ObjectKey{
			Name:      fmt.Sprintf("%s-0", name),
			Namespace: namespace,
		}, pod)
		if err != nil {
			klog.Errorf("Get notebook pod from apiServer err : %s", err.Error())
			return "", err
		}

		err = nh.routeCache.Store(cacheKey, pod.Status.PodIP)
		if err != nil {
			klog.Errorf("The value type is invalid : %s", err.Error())
			return "", err
		}

		return pod.Status.PodIP, nil
	}
}

type NotebookStatus string

const (
	Running  NotebookStatus = "Running"
	Stopped  NotebookStatus = "Stopped"
	Deleted  NotebookStatus = "Deleted"
	Starting NotebookStatus = "Starting"
)

type timeEventSlice []corev1.Event

func (p timeEventSlice) Len() int {
	return len(p)
}

func (p timeEventSlice) Less(i, j int) bool {
	return p[i].EventTime.Before(&p[j].EventTime)
}

func (p timeEventSlice) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (nh *NotebookHandler) ListEvents(namespace string) (map[string]string, error) {
	eventsList := &corev1.EventList{}
	err := nh.client.List(context.Background(), eventsList, client.InNamespace(namespace))
	if err != nil {
		klog.Errorf("List Events err : %s", err.Error())
		return nil, err
	}
	res := make(map[string]string)
	events := timeEventSlice(eventsList.Items)
	sort.Sort(events)
	for _, item := range events {
		res[item.InvolvedObject.Name] = res[item.InvolvedObject.Name] + "\n" + item.Message
	}
	return res, nil
}

func (nh *NotebookHandler) SyncNotebook(notebook *v1.Notebook) error {
	if len(notebook.Spec.Template.Spec.InitContainers) > 0 {
		envMap := make(map[string]string)

		for _, item := range notebook.Spec.Template.Spec.InitContainers[0].Env {
			envMap[item.Name] = item.Value
		}
		base64Code := envMap["BASE64CONFIG"]
		if base64Code != "" {
			configFile, err := base64.StdEncoding.DecodeString(base64Code)
			if err != nil {
				return nh.storageBackend.WriteNotebook(notebook)
			}

			configJson, err := yaml.YAMLToJSON(configFile)
			if err != nil {
				return nh.storageBackend.WriteNotebook(notebook)
			}

			kubeConfig := KubeConfig{}
			err = json.Unmarshal(configJson, &kubeConfig)
			if err != nil {
				return nh.storageBackend.WriteNotebook(notebook)
			}
			userName := kubeConfig.AuthInfos[0].Name

			gvr := schema.GroupVersionResource{
				Group:    "data.kubeai.alibabacloud.com",
				Version:  "v1",
				Resource: "users",
			}

			userData, err := nh.dynamicClient.Resource(gvr).Namespace("kube-ai").Get(context.TODO(), userName, metav1.GetOptions{})
			if err != nil {
				return nh.storageBackend.WriteNotebook(notebook)
			}

			data, err := userData.MarshalJSON()
			if err != nil {
				return nh.storageBackend.WriteNotebook(notebook)
			}

			userMessage := make(map[string]interface{})

			if err := json.Unmarshal(data, &userMessage); err != nil {
				return nh.storageBackend.WriteNotebook(notebook)
			}
			if notebook.Labels == nil {
				notebook.Labels = make(map[string]string)
			}

			notebook.Labels["userName"] = (userMessage["spec"].(map[string]interface{}))["userName"].(string)
		}
	}
	return nh.storageBackend.WriteNotebook(notebook)
}

func (nh *NotebookHandler) CompatibleNotebook(namespace string) error {
	notebookList := &v1.NotebookList{}
	err := nh.client.List(context.Background(), notebookList, client.InNamespace(namespace))
	if err != nil {
		klog.Errorf("List notebooks err : %s", err.Error())
		return err
	}
	for index, notebookItem := range notebookList.Items {
		_, err := nh.storageBackend.GetNotebook(notebookItem.Namespace, notebookItem.Name)
		if err != nil {
			if gorm.IsRecordNotFoundError(err) {
				err = nh.SyncNotebook(&notebookList.Items[index])
				if err != nil {
					return err
				}
				continue
			} else {
				return err
			}
		}
	}
	return nil
}

func (nh *NotebookHandler) ListNotebookFromStorage(namespace, userName, userId string, c *gin.Context) ([]NotebookMessage, error) {
	eventsMap, _ := nh.ListEvents(namespace)
	session := sessions.Default(c)
	var notebooksListFromStorage []*dmo.Notebook

	var err error
	if session.Get(auth.SessionKeyRole) == auth.SessionValueRoleAdmin {
		notebooksListFromStorage, err = nh.storageBackend.ListAllNotebook(&backends.NotebookQuery{
			Namespace: namespace,
			UserName:  userName,
			UID:       userId,
		})
	} else {
		notebooksListFromStorage, err = nh.storageBackend.ListNotebook(&backends.NotebookQuery{
			Namespace: namespace,
			UserName:  userName,
			UID:       userId,
		})
	}

	if err != nil {
		klog.Errorf("List notebooks err : %s", err.Error())
		return nil, err
	}
	if notebooksListFromStorage == nil {
		return []NotebookMessage{}, err
	}
	notebookMessageList := make([]NotebookMessage, 0, len(notebooksListFromStorage))

	for _, item := range notebooksListFromStorage {
		volumes := make([]converters.TempVolume, 0)
		_ = json.Unmarshal([]byte(item.Volumes), &volumes)
		volumesArr := make([]string, 0)
		for _, volume := range volumes {
			if volume.Name == "dshm" || volume.Name == "kube" {
				continue
			}
			volumesArr = append(volumesArr, volume.Name)
		}
		event := "Unknown state."
		if item.Status == string(Stopped) {
			if value, ok := eventsMap[item.Name]; ok {
				event = value
			}
		}
		age := fmtDuration(metav1.Now().Time.Sub(item.GmtCreated))
		tmpNotebook := v1.Notebook{}
		nh.client.Get(context.Background(), types.NamespacedName{
			Namespace: item.Namespace,
			Name:      item.Name,
		}, &tmpNotebook)

		var bathPath string
		notebookType := tmpNotebook.Labels[Notebook_Type_Label]
		if notebookType == strings.ToLower(Notebook_Type_VSCOde) {
			bathPath = fmt.Sprintf("/vscode/%s/%s/", item.Namespace, item.Name)
		} else {
			bathPath = fmt.Sprintf("/notebook/%s/%s/lab", item.Namespace, item.Name)
		}

		errMessage := "{}"
		if item.Status == string(Starting) {
			notebookPod := &corev1.Pod{}
			if err = nh.client.Get(context.Background(),
				types.NamespacedName{
					Name:      fmt.Sprintf("%s-0", item.Name),
					Namespace: item.Namespace}, notebookPod); err == nil {
				sort.Sort(ConditionsSorted(notebookPod.Status.Conditions))
				if len(notebookPod.Status.Conditions) > 0 {
					jsonMessageBytes, _ := json.Marshal(notebookPod.Status.Conditions[len(notebookPod.Status.Conditions)-1])
					errMessage = string(jsonMessageBytes)
				}
				item.Status = string(notebookPod.Status.Phase)
			}
		}

		notebookMessageList = append(notebookMessageList, NotebookMessage{
			Name:       item.Name,
			Namespace:  item.Namespace,
			Image:      item.Image,
			Volumes:    volumesArr,
			Age:        age,
			AccessPath: bathPath,
			Cpus:       item.Cpu,
			Memory:     item.Memory,
			Gpus:       item.Gpu,
			Status:     item.Status,
			Event:      event,
			Token:      item.Token,
			ErrMessage: errMessage,
		})
	}
	//sort.Sort(NotebookMessages(notebookMessageList))
	return notebookMessageList, nil
}

func (nh *NotebookHandler) ListNotebook(namespace string) ([]NotebookMessage, error) {
	eventsMap, _ := nh.ListEvents(namespace)
	notebookList := &v1.NotebookList{}
	err := nh.client.List(context.Background(), notebookList, client.InNamespace(namespace))
	if err != nil {
		klog.Errorf("List notebooks err : %s", err.Error())
		return nil, err
	}
	notebookMessageList := make([]NotebookMessage, 0, len(notebookList.Items))
	for _, item := range notebookList.Items {
		itemSpec := item.Spec.Template.Spec
		tempVolumesArray := make([]string, 0, len(itemSpec.Volumes))
		for _, volume := range itemSpec.Volumes {
			if volume.PersistentVolumeClaim != nil {
				tempVolumesArray = append(tempVolumesArray, volume.Name)
			}
		}
		creatTime := item.CreationTimestamp.Time
		nowTime := metav1.Now().Time
		age := fmtDuration(nowTime.Sub(creatTime))
		if 1 != len(itemSpec.Containers) {
			klog.Warningf("pod container size > 1=%d", len(itemSpec.Containers))
		}
		gpuLimits := itemSpec.Containers[0].Resources.Limits["nvidia.com/gpu"]
		aliyunGpu := itemSpec.Containers[0].Resources.Limits["aliyun.com/gpu"]
		gpuLimits.Add(aliyunGpu)
		event := "Unknown state."

		status := Stopped
		if len(item.Status.Conditions) < 1 {
			status = Starting
			if value, ok := eventsMap[item.Name]; ok {
				event = value
			}
		} else if item.Status.Conditions[0].Type == "Running" {
			status = Running
		} else if item.Status.Conditions[0].Type == "Waiting" {
			status = Starting
		}

		var bathPath string
		notebookType := item.Labels[Notebook_Type_Label]
		if notebookType == strings.ToLower(Notebook_Type_VSCOde) {
			bathPath = fmt.Sprintf("/vscode/%s/%s/", item.Namespace, item.Name)
		} else {
			bathPath = fmt.Sprintf("/notebook/%s/%s/lab", item.Namespace, item.Name)
		}

		errMessage := "{}"
		if status == Starting {
			notebookPod := &corev1.Pod{}
			if err = nh.client.Get(context.Background(),
				types.NamespacedName{
					Name:      fmt.Sprintf("%s-0", item.Name),
					Namespace: item.Namespace}, notebookPod); err == nil {
				sort.Sort(ConditionsSorted(notebookPod.Status.Conditions))
				if len(notebookPod.Status.Conditions) > 0 {
					jsonMessageBytes, _ := json.Marshal(notebookPod.Status.Conditions)
					errMessage = string(jsonMessageBytes)
				}
				status = NotebookStatus(notebookPod.Status.Phase)
			}
		}

		tempNotebookMessage := NotebookMessage{
			Name:       item.Name,
			Namespace:  namespace,
			Image:      itemSpec.Containers[0].Image,
			Volumes:    tempVolumesArray,
			Age:        age,
			AccessPath: bathPath,
			Cpus:       itemSpec.Containers[0].Resources.Requests.Cpu().String(),
			Memory:     itemSpec.Containers[0].Resources.Requests.Memory().String(),
			Gpus:       gpuLimits.String(),
			Status:     string(status),
			Event:      event,
			ErrMessage: errMessage,
		}
		notebookMessageList = append(notebookMessageList, tempNotebookMessage)
	}
	sort.Sort(NotebookMessages(notebookMessageList))
	return notebookMessageList, nil
}

func (nh *NotebookHandler) DeleteNotebook(name, namespace string) error {
	notebook := &v1.Notebook{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if namespace == "" || name == "" {
		return errors.New("delete notebook with name or namespace empty")
	}

	nh.routeCache.Delete(utils2.GetCacheKey(namespace, name, utils2.NotebookSvc))
	nh.routeCache.Delete(utils2.GetCacheKey(namespace, name, utils2.NotebookPod))

	klog.Infof("DeleteNotebook route ok name:%s namespace:%s", name, namespace)
	err := nh.client.Delete(context.Background(), notebook)
	return err
}

type VolumeData struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type NotebookSubmitData struct {
	Name             string                        `json:"name"`
	Namespace        string                        `json:"namespace"`
	Image            string                        `json:"image"`
	Volumes          []VolumeData                  `json:"volumes"`
	ImagePullPolicy  string                        `json:"imagePullPolicy"`
	Cpus             string                        `json:"cpus"`
	Gpus             string                        `json:"gpus"`
	Memory           string                        `json:"memory"`
	UserId           string                        `json:"userId"`
	UserName         string                        `json:"userName"`
	Token            string                        `json:"token"`
	NotebookType     string                        `json:"NotebookType"`
	ImagePullSecrets []string                      `json:"imagePullSecrets"`
	NodeSelectors    map[string]string             `json:"nodeSelectors"`
	Annotations      map[string]string             `json:"annotations"`
	Labels           map[string]string             `json:"labels"`
	Tolerates        map[string]dmo.TolerationData `json:"tolerates"`
}

func (nh *NotebookHandler) SubmitNotebookByData(data NotebookSubmitData) error {
	tempNotebook, err := GetNotebookResource(data)
	if err != nil {
		klog.Errorf("Submit notebook err : %s", err.Error())
		return err
	}
	return nh.submitNotebook(tempNotebook)
}

func (nh *NotebookHandler) submitNotebook(notebook client.Object) error {
	err := nh.client.Create(context.Background(), notebook)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (nh *NotebookHandler) ListPVC(ns string) ([]PVCObject, error) {
	list := &corev1.PersistentVolumeClaimList{}
	if err := nh.client.List(context.TODO(), list, &client.ListOptions{Namespace: ns}); err != nil {
		klog.Errorf("List pvc err : %s", err.Error())
		return nil, err
	}
	ret := nh.PersistentVolumeClaimFilter(*list, ns)
	return ret, nil
}

func (nh *NotebookHandler) GetMaxGpuNumbers() (int64, error) {
	dataHandler := NewDataHandler()
	request, err := dataHandler.GetClusterRequestResource("Running")
	if err != nil {
		klog.Errorf("Get gpu used number err : %s", err.Error())
		return 0, err
	}
	total, err := dataHandler.GetClusterTotalResource()
	if err != nil {
		klog.Errorf("Get gpu total number err : %s", err.Error())
		return 0, err
	}
	return total.TotalGPU - request.RequestGPU, nil
}

type PVCObject struct {
	Name    string `json:"name"`
	Isbound bool   `json:"isBound"`
}

func (nh *NotebookHandler) PersistentVolumeClaimFilter(list corev1.PersistentVolumeClaimList, namespace string) []PVCObject {
	origin := list.Items
	res := make([]PVCObject, 0)
	notebookList, _ := nh.ListNotebook(namespace)
	isBound := make(map[string]bool)
	for _, notebook := range notebookList {
		for _, volume := range notebook.Volumes {
			isBound[volume] = true
		}
	}

	for _, item := range origin {
		if item.Status.Phase == "Bound" && len(item.Status.AccessModes) > 0 {
			if item.Status.AccessModes[0] == "ReadWriteOnce" || item.Status.AccessModes[0] == "ReadWriteMany" {
				PVCItem := PVCObject{
					Name:    item.Name,
					Isbound: isBound[item.Name],
				}
				res = append(res, PVCItem)
			}
		}
	}
	return res
}

func (nh *NotebookHandler) ListSVC(ns string) (map[string]string, error) {
	list := &corev1.ServiceList{}
	if err := nh.client.List(context.TODO(), list, &client.ListOptions{Namespace: ns}); err != nil {
		klog.Errorf("List svc number err : %s", err.Error())
		return nil, err
	}
	klog.Infof("list svc in ns:%s length:%d", ns, len(list.Items))
	notebookTarget := make(map[string]string)
	for _, svc := range list.Items {
		notebookTarget[svc.Name] = svc.Spec.ClusterIP + ":80"
	}
	return notebookTarget, nil
}

func GetNotebookResource(data NotebookSubmitData) (*v1.Notebook, error) {
	var notebookCommand string
	jupyterCommand := fmt.Sprintf("jupyter-lab --notebook-dir=/home/jovyan "+
		"--ip=0.0.0.0 --no-browser "+
		"--allow-root --port=8888 "+
		"--ServerApp.token='%s' --ServerApp.password='' --ServerApp.allow_origin='*' "+
		"--ServerApp.base_url=${NB_PREFIX} "+
		"--ServerApp.authenticate_prometheus=False", data.Token)

	vscodeCommand := fmt.Sprintf("code-server --bind-addr 0.0.0.0:8888 --auth none")

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	for _, volume := range data.Volumes {
		if volume.Name == "" {
			klog.Errorf("volume Name is empty")
			return &v1.Notebook{}, errors.New("volume Name is empty")
		}
		if volume.Path == "" {
			klog.Errorf("volume Path is empty")
			return &v1.Notebook{}, errors.New("volume Path is empty")
		}
		tempVolume := corev1.Volume{
			Name: volume.Name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: volume.Name,
				},
			},
		}
		tempVolumeMount := corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.Path,
		}
		volumes = append(volumes, tempVolume)
		volumeMounts = append(volumeMounts, tempVolumeMount)
	}

	commitAgentHostType := corev1.HostPathDirectoryOrCreate
	commitAgentVolume := corev1.Volume{
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/run/commit-agent",
				Type: &commitAgentHostType,
			},
		},
		Name: "agent-sock",
	}

	defaultVolume := corev1.Volume{
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: "Memory",
			},
		},
		Name: "dshm",
	}
	kubeconfigVolume := corev1.Volume{
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
		Name: "kube",
	}
	defaultVolumeMount := corev1.VolumeMount{
		Name:      "dshm",
		MountPath: "/dev/shm",
	}
	kubeconfigVolumeMount := corev1.VolumeMount{
		Name:      "kube",
		MountPath: "/mnt",
	}

	agentVolumeMount := corev1.VolumeMount{
		Name:      "agent-sock",
		MountPath: "/mnt/commit-agent",
	}

	volumes = append(volumes, defaultVolume, kubeconfigVolume, commitAgentVolume)
	volumeMounts = append(volumeMounts, defaultVolumeMount, kubeconfigVolumeMount, agentVolumeMount)
	if data.Namespace == "" {
		klog.Error("the Namespace is empty")
		return &v1.Notebook{}, errors.New("the Namespace is empty")
	}
	if data.Name == "" {
		klog.Error("the Name is empty")
		return &v1.Notebook{}, errors.New("the Name is empty")
	}
	if data.UserId == "" {
		klog.Error("the UserId is empty")
		return &v1.Notebook{}, errors.New("the UserId is empty")
	}
	configFileByte, _, err := utils.GenerateUserKubeConfig(data.UserName, data.Namespace)
	if err != nil {
		klog.Errorf("generate user config error:%s", err.Error())
		return &v1.Notebook{}, err
	}
	imagePullSecrets := make([]corev1.LocalObjectReference, 0)
	for _, secret := range data.ImagePullSecrets {
		secretItem := corev1.LocalObjectReference{
			Name: secret,
		}
		imagePullSecrets = append(imagePullSecrets, secretItem)
	}
	encodedMessage := base64.StdEncoding.EncodeToString(configFileByte)

	Tolerations := make([]corev1.Toleration, 0)
	for key, value := range data.Tolerates {
		Tolerations = append(Tolerations, corev1.Toleration{
			Key:      key,
			Operator: corev1.TolerationOperator(value.Operator),
			Value:    value.Value,
			Effect:   corev1.TaintEffect(value.Effect),
		})
	}

	labels := data.Labels
	labels["User"] = strings.Replace(data.UserName, "@", "-", -1)
	labels["arena.kubeflow.org/console-user"] = data.UserId
	labels["Token"] = data.Token
	labels[Notebook_Type_Label] = strings.ToLower(data.NotebookType)

	var initContainerImage string
	initContainerImage = os.Getenv("INIT_IMAGE")
	if initContainerImage == "" {
		initContainerImage = "registry.cn-beijing.aliyuncs.com/acs/busybox:v1.0.0-aliyun"
	}

	if data.NotebookType == Notebook_Type_Jupyter {
		notebookCommand = jupyterCommand
	} else {
		notebookCommand = vscodeCommand
	}

	notebook := &v1.Notebook{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   data.Namespace,
			Name:        data.Name,
			Labels:      labels,
			Annotations: data.Annotations,
		},
		Spec: v1.NotebookSpec{
			Template: v1.NotebookTemplateSpec{
				Spec: corev1.PodSpec{
					Tolerations:  Tolerations,
					NodeSelector: data.NodeSelectors,
					InitContainers: []corev1.Container{
						{
							Name:  "initscript",
							Image: initContainerImage,
							Command: []string{
								"sh",
								"-c",
								"echo $BASE64CONFIG | base64 -d > /tmp/kubeconfig",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "BASE64CONFIG",
									Value: string(encodedMessage),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kube",
									MountPath: "/tmp",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Command: []string{
								"sh",
								"-c",
								notebookCommand,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "KUBECONFIG",
									Value: "/mnt/kubeconfig",
								},
							},
							Image:           data.Image,
							ImagePullPolicy: corev1.PullPolicy(data.ImagePullPolicy),
							Name:            data.Name,
							VolumeMounts:    volumeMounts,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									ResourceGPU: resource.MustParse(data.Gpus),
								},
								Requests: corev1.ResourceList{
									"cpu":    resource.MustParse(data.Cpus),
									"memory": resource.MustParse(data.Memory),
								},
							},
						},
					},
					ImagePullSecrets: imagePullSecrets,
					Volumes:          volumes,
				},
			},
		},
	}
	return notebook, nil
}

const (
	Day = 24 * time.Hour
)

func fmtDuration(d time.Duration) string {
	d = d.Round(time.Second)
	day := d / Day
	d -= day * Day
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if day > 0 {
		return fmt.Sprintf("%dd%dh%dm%ds", day, h, m, s)
	} else if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
