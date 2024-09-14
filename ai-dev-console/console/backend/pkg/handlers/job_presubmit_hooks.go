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
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/k8sutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

type preSubmitHook func(job runtime.Object)

func tfJobPreSubmitAutoConvertReplicas(job runtime.Object) {
	if job == nil {
		return
	}
	tfJob, ok := job.(*training.TFJob)
	if !ok {
		return
	}

	totalReplicas := k8sutil.GetTotalReplicas(tfJob.Spec.TFReplicaSpecs)
	if tbSpec, ok := tfJob.Spec.TFReplicaSpecs[apiv1.ReplicaTypeTensorBoard]; ok {
		if tbSpec.Replicas != nil {
			totalReplicas -= *tbSpec.Replicas
		} else {
			totalReplicas -= 1
		}
	}
	_, workerExist := tfJob.Spec.TFReplicaSpecs[training.TFReplicaTypeWorker]
	_, chiefExist := tfJob.Spec.TFReplicaSpecs[training.TFReplicaTypeChief]
	if totalReplicas == 1 && workerExist && !chiefExist {
		workerSpec := tfJob.Spec.TFReplicaSpecs[training.TFReplicaTypeWorker]
		tfJob.Spec.TFReplicaSpecs[training.TFReplicaTypeChief] = workerSpec.DeepCopy()
		delete(tfJob.Spec.TFReplicaSpecs, training.TFReplicaTypeWorker)
	}
}

func pytorchJobPreSubmitAutoConvertReplicas(job runtime.Object) {
	if job == nil {
		return
	}
	pytorchJob, ok := job.(*training.PyTorchJob)
	if !ok {
		return
	}

	var (
		workerExist, masterExist       bool
		workerReplicas, masterReplicas int32
	)

	_, workerExist = pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeWorker]
	_, masterExist = pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeMaster]

	if workerExist && pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeWorker].Replicas != nil {
		workerReplicas = *pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeWorker].Replicas
	}
	if masterExist && pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeMaster].Replicas != nil {
		masterReplicas = *pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeMaster].Replicas
	}

	if masterReplicas == 0 && workerReplicas > 0 {
		workerSpec := pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeWorker]
		pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeMaster] = workerSpec.DeepCopy()
		pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeMaster].Replicas = pointer.Int32Ptr(1)
		workerReplicas = workerReplicas - 1
	}

	if workerReplicas <= 0 {
		delete(pytorchJob.Spec.PyTorchReplicaSpecs, training.PyTorchReplicaTypeWorker)
	} else if workerReplicas > 0 {
		pytorchJob.Spec.PyTorchReplicaSpecs[training.PyTorchReplicaTypeWorker].Replicas = pointer.Int32Ptr(workerReplicas)
	}

	return
}

func getMainContainerImage(spec *apiv1.ReplicaSpec, main string) string {
	if spec == nil {
		return ""
	}
	for i := range spec.Template.Spec.Containers {
		if spec.Template.Spec.Containers[i].Name == main {
			return spec.Template.Spec.Containers[i].Image
		}
	}
	return ""
}
