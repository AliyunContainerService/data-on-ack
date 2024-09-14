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

package job_controller

import (
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubelet/eviction"
)

// initializeReplicaStatuses initializes the ReplicaStatuses for replica.
func initializeReplicaStatuses(jobStatus *apiv1.JobStatus, rtype apiv1.ReplicaType) {
	if jobStatus.ReplicaStatuses == nil {
		jobStatus.ReplicaStatuses = make(map[apiv1.ReplicaType]*apiv1.ReplicaStatus)
	}

	jobStatus.ReplicaStatuses[rtype] = &apiv1.ReplicaStatus{}
}

// updateJobReplicaStatuses updates the JobReplicaStatuses according to the pod.
func updateJobReplicaStatuses(jobStatus *apiv1.JobStatus, rtype apiv1.ReplicaType, pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		jobStatus.ReplicaStatuses[rtype].Active++
	case corev1.PodSucceeded:
		jobStatus.ReplicaStatuses[rtype].Succeeded++
	case corev1.PodFailed:
		jobStatus.ReplicaStatuses[rtype].Failed++
		if eviction.PodIsEvicted(pod.Status) {
			jobStatus.ReplicaStatuses[rtype].Evicted++
		}
	}
}
