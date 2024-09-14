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
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type EvaluateHandler struct {
	client         client.Client
	storageBackend backends.ObjectStorageBackend
	clientBackend  backends.ObjectClientBackend
}

func NewEvaluateHandler(objStorage string, clientTypeName string) (*EvaluateHandler, error) {
	objBackend := registry.GetObjectBackend(objStorage)
	if objBackend == nil {
		return nil, fmt.Errorf("no object backend storage named: %s", objStorage)
	}
	err := objBackend.Initialize()
	if err != nil {
		return nil, err
	}

	clientBackend := registry.GetActionBackend(clientTypeName)
	if clientBackend == nil {
		return nil, fmt.Errorf("no action backend named: %s", objStorage)
	}
	err = clientBackend.Initialize()
	if err != nil {
		return nil, err
	}

	return &EvaluateHandler{
		client:         clientmgr.GetCtrlClient(),
		storageBackend: objBackend,
		clientBackend:  clientBackend,
	}, nil
}

func (eh *EvaluateHandler) ListEvaluateJobsFromBackend(query *backends.EvaluateJobQuery) ([]model.EvaluateJobInfo, error) {
	evaluateJobs, err := eh.storageBackend.ListEvaluateJobs(query)
	if err != nil {
		return nil, err
	}
	evaluateJobsMessage := make([]model.EvaluateJobInfo, 0, len(evaluateJobs))
	for _, evaluateJob := range evaluateJobs {
		evaluateJobsMessage = append(evaluateJobsMessage, model.ConvertDMOEvaluateJobToEvaluateJobInfo(evaluateJob))
	}
	return evaluateJobsMessage, nil
}

func (eh *EvaluateHandler) SubmitEvaluateJob(userName string, data []byte) error {
	evaluateJob := &dmo.SubmitEvaluateJobInfo{}
	err := json.Unmarshal(data, &evaluateJob)
	if err != nil {
		return err
	}

	serviceList := &corev1.ServiceList{}
	mysqlIP := ""
	eh.client.List(context.TODO(), serviceList, &client.ListOptions{Namespace: "kube-ai"})
	for _, service := range serviceList.Items {
		if service.Name == "ack-mysql" {
			mysqlIP = service.Spec.ClusterIP
			break
		}
	}
	if mysqlIP == "" {
		return errors.New("No mysql server.")
	}

	evaluateJob.Envs = make(map[string]string)

	evaluateJob.Envs["MYSQL_HOST"] = mysqlIP
	evaluateJob.Envs["MYSQL_PORT"] = "3306"
	evaluateJob.Envs["MYSQL_USERNAME"] = "kubeai"
	evaluateJob.Envs["MYSQL_PASSWORD"] = "kubeai@ACK"
	evaluateJob.Envs["ENABLE_MYSQL"] = "True"

	return eh.clientBackend.UserName(userName).SubmitEvaluateJob(evaluateJob)
}

func (eh *EvaluateHandler) GetEvaluateJobData(ns, name, jobID string) (*model.EvaluateJobInfo, error) {
	dmoEvaluateJob, err := eh.storageBackend.GetEvaluateJob("", "", jobID)
	if err != nil {
		klog.Errorf("get job failed, err:%v", err)
		return nil, err
	}
	evaluateJobInfo := model.ConvertDMOEvaluateJobToEvaluateJobInfo(dmoEvaluateJob)
	return &evaluateJobInfo, nil
}

//func (eh *EvaluateHandler) DeleteEvaluateJobFromBackend() {
//
//}

func (eh *EvaluateHandler) DeleteEvaluateJobFromCluster(userName, ns, name string) error {
	return eh.clientBackend.UserName(userName).DeleteEvaluateJob(ns, name)
}

type SearchItem struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	ID        string `json:"id"`
}

type SearchArray []SearchItem

type MetricsItem struct {
	SearchItem
	Metrics map[string]interface{} `json:"metrics"`
}

type CompareData struct {
	Names      []string                 `json:"names"`
	Namespaces []string                 `json:"namespaces"`
	Metrics    map[string][]interface{} `json:"metrics"`
}

func (eh *EvaluateHandler) CompareEvaluateJobs(array SearchArray) (CompareData, error) {
	metricsItems := make([]MetricsItem, 0)
	metricsRecord := make(map[string]int)
	for _, item := range array {
		tempEvaluateJobInfo, err := eh.GetEvaluateJobData(item.Namespace, item.Name, item.ID)
		if err != nil {
			klog.Errorf("get job failed, err:%v", err)
			return CompareData{}, nil
		}

		var itemMetrics string
		if tempEvaluateJobInfo.Metrics == "" {
			itemMetrics = "{}"
		} else {
			itemMetrics = strings.Replace(tempEvaluateJobInfo.Metrics, "'", "\"", -1)
		}

		metrics := make(map[string]interface{})
		err = json.Unmarshal([]byte(itemMetrics), &metrics)
		if err != nil {
			klog.Errorf("get job failed, err:%v", err)
			fmt.Println("Unmarshal err:", err)
			return CompareData{}, err
		}

		for key, _ := range metrics {
			metricsRecord[key]++
		}

		itemMetricsItem := MetricsItem{
			SearchItem: SearchItem{
				Name:      item.Name,
				Namespace: item.Namespace,
			},
			Metrics: metrics,
		}
		metricsItems = append(metricsItems, itemMetricsItem)
	}

	intersection := make([]string, 0)
	length := len(array)

	metricsRecord["ROC"]--

	for key, value := range metricsRecord {
		if value == length {
			intersection = append(intersection, key)
		}
	}

	if len(intersection) < 1 {
		return CompareData{}, nil
	}

	result := CompareData{
		Names:      []string{},
		Namespaces: []string{},
		Metrics:    map[string][]interface{}{},
	}

	for _, item := range intersection {
		result.Metrics[item] = make([]interface{}, 0, len(metricsItems))
	}

	for index, _ := range metricsItems {
		result.Names = append(result.Names, metricsItems[index].Name)
		result.Namespaces = append(result.Names, metricsItems[index].Namespace)
		for _, item := range intersection {
			result.Metrics[item] = append(result.Metrics[item], metricsItems[index].Metrics[item])
		}
	}

	return result, nil
}
