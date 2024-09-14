// Copyright 2018 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"strings"

	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

// setDefaults_TFJobPort sets the default ports for tensorflow container.
func setDefaults_TFJobPort(spec *corev1.PodSpec) {
	index := 0
	for i, container := range spec.Containers {
		if container.Name == TFJobDefaultContainerName {
			index = i
			break
		}
	}

	hasTFJobPort := false
	for _, port := range spec.Containers[index].Ports {
		if port.Name == TFJobDefaultPortName {
			hasTFJobPort = true
			break
		}
	}
	if !hasTFJobPort {
		spec.Containers[index].Ports = append(spec.Containers[index].Ports, corev1.ContainerPort{
			Name:          TFJobDefaultPortName,
			ContainerPort: TFJobDefaultPort,
		})
	}
}

func setDefaultReplicas(spec *v1.ReplicaSpec) {
	if spec.Replicas == nil {
		spec.Replicas = pointer.Int32Ptr(1)
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = TFJobDefaultRestartPolicy
	}
}

// setTypeNames_TFJob sets the name of all replica types from any case to correct case.
func setTypeNames_TFJob(tfJob *TFJob) {
	setTypeName_TFJob(tfJob, TFReplicaTypePS)
	setTypeName_TFJob(tfJob, TFReplicaTypeWorker)
	setTypeName_TFJob(tfJob, TFReplicaTypeChief)
	setTypeName_TFJob(tfJob, TFReplicaTypeMaster)
	setTypeName_TFJob(tfJob, TFReplicaTypeEval)
}

// setTypeName_TFJob sets the name of the replica type from any case to correct case.
// E.g. from ps to PS; from WORKER to Worker.
func setTypeName_TFJob(tfJob *TFJob, typ v1.ReplicaType) {
	for t := range tfJob.Spec.TFReplicaSpecs {
		if strings.EqualFold(string(t), string(typ)) && t != typ {
			spec := tfJob.Spec.TFReplicaSpecs[t]
			delete(tfJob.Spec.TFReplicaSpecs, t)
			tfJob.Spec.TFReplicaSpecs[typ] = spec
			return
		}
	}
}

func setDefaults_TFJobReplicas(spec *v1.ReplicaSpec) {
	if spec.Replicas == nil {
		spec.Replicas = pointer.Int32Ptr(1)
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = TFJobDefaultRestartPolicy
	}
}

// SetDefaults_TFJob sets any unspecified values to defaults.
func SetDefaults_TFJob(tfjob *TFJob) {
	// Set default cleanpod policy to Running.
	if tfjob.Spec.CleanPodPolicy == nil {
		running := v1.CleanPodPolicyRunning
		tfjob.Spec.CleanPodPolicy = &running
	}

	// Set default success policy to "".
	if tfjob.Spec.SuccessPolicy == nil {
		defaultPolicy := v1.SuccessPolicyAllWorkers
		tfjob.Spec.SuccessPolicy = &defaultPolicy
	}

	// Update the key of TFReplicaSpecs to camel case.
	setTypeNames_TFJob(tfjob)

	for _, spec := range tfjob.Spec.TFReplicaSpecs {
		// Set default replicas to 1.
		setDefaults_TFJobReplicas(spec)
		// Set default port to tensorFlow container.
		setDefaults_TFJobPort(&spec.Template.Spec)
	}
}
