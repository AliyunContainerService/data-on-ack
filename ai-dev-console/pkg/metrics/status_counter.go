/*
Copyright 2019 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"context"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func JobStatusCounter(kind string, reader client.Reader, filter func(status v1.JobStatus) bool) (result int32, err error) {
	var list runtime.Object
	if obj, ok := listObjectMap[kind]; ok {
		list = obj.DeepCopyObject()
	}
	err = reader.List(context.Background(), list)
	if err != nil {
		return 0, err
	}
	statuses := getJobStatusList(list, kind)
	result = int32(0)
	for _, status := range statuses {
		if filter(*status) {
			result++
		}
	}
	return result, nil
}

var (
	listObjectMap = map[string]runtime.Object{
		training.TFJobKind:      &training.TFJobList{},
		training.PyTorchJobKind: &training.PyTorchJobList{},
		training.XDLJobKind:     &training.XDLJobList{},
		training.XGBoostJobKind: &training.XGBoostJobList{},
	}
)

func getJobStatusList(obj runtime.Object, kind string) []*v1.JobStatus {
	statuses := make([]*v1.JobStatus, 0)
	switch kind {
	case training.TFJobKind:
		tfList := obj.(*training.TFJobList)
		for idx := range tfList.Items {
			statuses = append(statuses, &tfList.Items[idx].Status)
		}
	case training.PyTorchJobKind:
		pytorchList := obj.(*training.PyTorchJobList)
		for idx := range pytorchList.Items {
			statuses = append(statuses, &pytorchList.Items[idx].Status)
		}
	case training.XDLJobKind:
		xdlList := obj.(*training.XDLJobList)
		for idx := range xdlList.Items {
			statuses = append(statuses, &xdlList.Items[idx].Status)
		}
	case training.XGBoostJobKind:
		xgbList := obj.(*training.XGBoostJobList)
		for idx := range xgbList.Items {
			statuses = append(statuses, &xgbList.Items[idx].Status.JobStatus)
		}
	}
	return statuses
}
