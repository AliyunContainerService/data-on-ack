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
	"encoding/json"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type ModelsHandler struct {
	client         client.Client
	storageBackend backends.ObjectStorageBackend
}

func NewModelsHandler(objStorage string) (*ModelsHandler, error) {
	objBackend := registry.GetObjectBackend(objStorage)
	if objBackend == nil {
		return nil, fmt.Errorf("no object backend storage named: %s", objStorage)
	}
	err := objBackend.Initialize()
	if err != nil {
		return nil, err
	}
	return &ModelsHandler{
		client:         clientmgr.GetCtrlClient(),
		storageBackend: objBackend,
	}, nil
}

func (mh *ModelsHandler) GetModelDetails(modelID string) (dmo.Model, error) {
	model, err := mh.storageBackend.GetModel(modelID)
	if err != nil {
		return dmo.Model{}, err
	}
	return *model, nil
}

func (mh *ModelsHandler) DeleteModel(modelID string) error {
	err := mh.storageBackend.DeleteModel(modelID)
	if err != nil {
		return err
	}
	return nil
}

func (mh *ModelsHandler) GetModelsList(query *backends.ModelsQuery) ([]dmo.Model, error) {
	models, err := mh.storageBackend.ListModels(query)
	if err != nil {
		return nil, err
	}
	result := make([]dmo.Model, 0, len(models))
	for _, model := range models {
		result = append(result, *model)
	}
	return result, err
}

func (mh *ModelsHandler) CreateModel(data []byte) error {
	model := &dmo.Model{}
	err := json.Unmarshal(data, model)
	if err != nil {
		return err
	}
	model.OSSPath = ""
	model.GmtCreated = time.Now()
	err = mh.storageBackend.WriteModel(model)
	if err != nil {
		return err
	}
	return nil
}

//func (mh *ModelsHandler) GetTFJobMessage(name, namespace string) (string, error) {
//	tfjob := training.TFJob{}
//	err := mh.client.Get(context.Background(), client.ObjectKey{
//		Name: name,
//		Namespace: namespace,
//	}, &tfjob)
//	if err != nil {
//		return "", err
//	}
//	if spec, ok := tfjob.Spec.TFReplicaSpecs["Chief"]; ok{
//		spec.Template.Spec.Containers[0].VolumeMounts
//	}else {
//		spec = tfjob.Spec.TFReplicaSpecs["Worker"]
//
//	}
//}
