/*
Copyright 2021 The Alibaba Authors.

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

package converters

import (
	"encoding/json"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	batch "k8s.io/api/batch/v1"
	"k8s.io/klog"
)

func ConvertEvaluateJobToDMOEvaluateJob(evaluateJob *batch.Job) *dmo.EvaluateJob {
	klog.V(5).Infof("[ConvertEvaluateJobToDMOEvaluateJob] evaluateJob: %s/%s", evaluateJob.Namespace, evaluateJob.Name)
	envMap := make(map[string]string)
	for _, item := range evaluateJob.Spec.Template.Spec.Containers[0].Env {
		envMap[item.Name] = item.Value
	}

	//modelID, err := strconv.ParseUint(envMap["MODEL_ID"], 10, 64)
	//if err != nil {
	//	modelID = 0
	//}

	modelName := envMap["MODEL_NAME"]
	modelVersion := envMap["MODEL_VERSION"]

	var totalCommand string
	for _, command := range evaluateJob.Spec.Template.Spec.Containers[0].Command {
		totalCommand += command + " "
	}
	code := ""
	if len(evaluateJob.Spec.Template.Spec.InitContainers) > 0 {
		codeMap := make(map[string]string)
		for _, item := range evaluateJob.Spec.Template.Spec.InitContainers[0].Env {
			codeMap[item.Name] = item.Value
		}
		codeJson, _ := json.Marshal(codeMap)
		code = string(codeJson)
	}

	dmoEvaluateJob := &dmo.EvaluateJob{
		Name:         evaluateJob.GetName(),
		Namespace:    evaluateJob.GetNamespace(),
		JobID:        envMap["JOB_ID"],
		UID:          string(evaluateJob.GetUID()),
		ModelName:    modelName,
		ModelVersion: modelVersion,
		Image:        evaluateJob.Spec.Template.Spec.Containers[0].Image,
		DatasetPath:  envMap["DATASET_DIR"],
		Code:         code,
		Command:      totalCommand,
		ReportPath:   envMap["METRICS_DIR"],
		GmtCreated:   evaluateJob.GetCreationTimestamp().Time,
	}

	dmoEvaluateJob.Status = v1.JobCreated
	evaluateJobStatus := &evaluateJob.Status
	if condLen := len(evaluateJobStatus.Conditions); condLen > 0 {
		dmoEvaluateJob.Status = v1.JobConditionType(evaluateJobStatus.Conditions[condLen-1].Type)
	}
	dmoEvaluateJob.IsDeleted = 0

	//if len(evaluateJobStatus.Conditions) > 0 {
	//	cond := evaluateJobStatus.Conditions[len(evaluateJobStatus.Conditions)-1]
	//	dmoEvaluateJob.ReasonCode = pointer.StringPtr(cond.Reason)
	//	dmoEvaluateJob.Reason = pointer.StringPtr(cond.Message)
	//}

	return dmoEvaluateJob
}
