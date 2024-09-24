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
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	clientregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/tenant"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDLCHandler() *DLCHandler {
	return &DLCHandler{client: clientmgr.GetCtrlClient()}
}

type DLCHandler struct {
	client client.Client
}

type DLCCommonConfig struct {
	Namespace        string   `json:"namespace"`
	TFCpuImages      []string `json:"default-tf-cpu-images"`
	TFGpuImages      []string `json:"default-tf-gpu-images"`
	PytorchGpuImages []string `json:"default-pytorch-gpu-images"`
	ClusterID        string   `json:"clusterId,omitempty"`
	Version          string   `json:"version,omitempty"`
}

func (h *DLCHandler) GetDLCConfig() (*DLCCommonConfig, error) {
	if constants.SystemConfigName == "" {
		return nil, errors.New("empty system configmap name")
	}
	if constants.SystemNamespace == "" {
		return nil, errors.New("empty system namespace")
	}

	cm := v1.ConfigMap{}
	if err := h.client.Get(context.Background(), types.NamespacedName{
		Name:      constants.SystemConfigName,
		Namespace: constants.SystemNamespace,
	}, &cm); err != nil {
		return nil, fmt.Errorf("failed to get common config, err: %v", err)
	}

	commonCfg := cm.Data["commonConfig"]
	dlcCommonCfg := DLCCommonConfig{}
	if err := json.Unmarshal([]byte(commonCfg), &dlcCommonCfg); err != nil {
		return nil, fmt.Errorf("failed to marshal dlc common config, err: %v", err)
	}

	return &dlcCommonCfg, nil
}

func (h *DLCHandler) ListAvailableNamespaces(userName string) ([]string, error) {
	ctrlClient, err := clientregistry.GetCtrlClient(userName)
	if err != nil {
		return nil, err
	}

	namespaces := v1.NamespaceList{}
	if err := ctrlClient.List(context.Background(), &namespaces); err != nil {
		return nil, err
	}

	preservedNS := []string{"kube-system"}

	available := make([]string, 0, len(namespaces.Items)-1)
	for i := range namespaces.Items {
		skip := false
		for _, preserved := range preservedNS {
			if namespaces.Items[i].Name == preserved {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		available = append(available, namespaces.Items[i].Name)
	}
	return available, nil
}

func (h *DLCHandler) DetectJobsInNS(userName, ns, kind string) bool {
	var (
		list     client.ObjectList
		detector func(object runtime.Object) bool
	)

	switch kind {
	case training.TFJobKind:
		list = &training.TFJobList{}
		detector = func(object runtime.Object) bool {
			tfJobs := object.(*training.TFJobList)
			return len(tfJobs.Items) > 0
		}
	case training.PyTorchJobKind:
		list = &training.PyTorchJobList{}
		detector = func(object runtime.Object) bool {
			pytorchJobs := object.(*training.PyTorchJobList)
			return len(pytorchJobs.Items) > 0
		}
	default:
		return false
	}

	ctrlClient, err := clientregistry.GetCtrlClient(userName)
	if err != nil {
		return false
	}

	if err := ctrlClient.List(context.Background(), list, &client.ListOptions{Namespace: ns}); err != nil {
		return false
	}
	return detector(list)
}
