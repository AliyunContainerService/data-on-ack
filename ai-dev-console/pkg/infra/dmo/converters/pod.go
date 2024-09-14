/*
Copyright 2020 The Alibaba Authors.

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
	"errors"
	"fmt"
	"time"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/k8sutil"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/resource_utils"

	"github.com/kubeflow/arena/pkg/apis/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	quotav1 "k8s.io/kubernetes/pkg/quota/v1"
)

var (
	ErrNoDependentOwner   = errors.New("object has no dependent owner")
	ErrNoReplicaTypeLabel = fmt.Errorf("object has no replica type label [%s]", apiv1.ReplicaTypeLabel)
)

// ConvertPodToDMOPod converts a native pod object to dmo pod.
func ConvertPodToDMOPod(pod *v1.Pod) (*dmo.Pod, error) {
	klog.V(5).Infof("[ConvertPodToDMOPod] pod: %s/%s", pod.Namespace, pod.Name)
	dmoPod := &dmo.Pod{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		UID:         string(pod.UID),
		EtcdVersion: pod.ResourceVersion,
		GmtCreated:  pod.CreationTimestamp.Time,
	}

	owner := k8sutil.ResolveDependentOwner(pod)
	if owner == nil {
		return nil, ErrNoDependentOwner
	}
	dmoPod.JobUID = string(owner.UID)
	dmoPod.JobName = owner.Name
	defaultContainerName := getDefaultContainerName(owner.Kind)
	rtype, ok := k8sutil.GetReplicaType(pod, owner.Kind)
	if !ok {
		return nil, ErrNoReplicaTypeLabel
	}
	dmoPod.ReplicaType = rtype

	if pod.Status.PodIP != "" {
		dmoPod.PodIP = &pod.Status.PodIP
	}
	if pod.Status.HostIP != "" {
		dmoPod.HostIP = &pod.Status.HostIP
	}

	if len(pod.Spec.Containers) == 0 {
		return dmoPod, nil
	}

	image := pod.Spec.Containers[0].Image
	for idx := 1; idx < len(pod.Spec.Containers); idx++ {
		if pod.Spec.Containers[idx].Name == defaultContainerName {
			image = pod.Spec.Containers[idx].Image
			break
		}
	}
	dmoPod.Image = image

	// Pod status Unknown defaulted.
	dmoPod.Status = v1.PodUnknown
	dmoPod.Status = pod.Status.Phase
	if len(pod.Status.ContainerStatuses) == 0 {
		return dmoPod, nil
	}

	containerStatus := pod.Status.ContainerStatuses[0]
	for idx := 1; idx < len(pod.Status.ContainerStatuses); idx++ {
		if pod.Status.ContainerStatuses[idx].Name == defaultContainerName {
			containerStatus = pod.Status.ContainerStatuses[idx]
			break
		}
	}

	switch pod.Status.Phase {
	case v1.PodPending:
		// Do nothing.
	case v1.PodRunning:
		if containerStatus.State.Running != nil {
			startedAt := containerStatus.State.Running.StartedAt
			dmoPod.GmtPodRunning = &startedAt.Time
		}
	case v1.PodSucceeded, v1.PodFailed:
		if dmoPod.GmtPodRunning == nil && containerStatus.State.Running != nil {
			dmoPod.GmtPodRunning = &containerStatus.State.Running.StartedAt.Time
		}
		if containerStatus.State.Terminated != nil {
			finishedAt := containerStatus.State.Terminated.FinishedAt
			dmoPod.GmtPodFinished = &finishedAt.Time
			if dmoPod.Status == v1.PodFailed {
				extended := fmt.Sprintf("Reason: %v\nExitCode: %v\nMessage: %v",
					containerStatus.State.Terminated.Reason,
					containerStatus.State.Terminated.ExitCode,
					containerStatus.State.Terminated.Message)
				dmoPod.Extended = &extended
			}
		}
		if dmoPod.GmtPodFinished == nil || dmoPod.GmtPodFinished.IsZero() {
			dmoPod.GmtPodFinished = util.TimePtr(time.Now())
		}
	}

	// shallow copy before serialization and discard `status` filed.
	dump, _ := json.Marshal(&v1.Pod{TypeMeta: pod.TypeMeta, ObjectMeta: pod.ObjectMeta, Spec: pod.Spec})
	dmoPod.PodJson = string(dump)

	return dmoPod, nil
}

func computePodResources(podSpec *v1.PodSpec) (resources v1.ResourceRequirements) {
	initResources := resource_utils.MaximumContainersResources(podSpec.InitContainers)
	runtimeResources := resource_utils.SumUpContainersResources(podSpec.Containers)
	resources.Requests = quotav1.Max(initResources.Requests, runtimeResources.Requests)
	resources.Limits = quotav1.Max(initResources.Limits, runtimeResources.Limits)
	return resources
}

func getDefaultContainerName(kind string) string {
	switch kind {
	case training.TFJobKind:
		return training.TFJobDefaultContainerName
	case training.PyTorchJobKind:
		return training.PyTorchJobDefaultContainerName
	case training.XDLJobKind:
		return training.XDLJobDefaultContainerName
	case training.XGBoostJobKind:
		return training.XGBoostJobDefaultContainerName
	}
	return ""
}

// ConvertArenaInstanceToDMOPod converts a native pod object to dmo pod.
func ConvertArenaInstanceToDMOPod(job *types.TrainingJobInfo, ins *types.TrainingJobInstance) (*dmo.Pod, error) {
	dmoPod := &dmo.Pod{
		Name:       ins.Name,
		JobName:    job.Name,
		Namespace:  job.Namespace,
		GmtCreated: time.Unix(job.CreationTimestamp, 0),
	}
	if ins.IP != "" {
		dmoPod.PodIP = &ins.IP
	}
	if ins.NodeIP != "" {
		dmoPod.HostIP = &ins.NodeIP
	}
	dmoPod.Status = v1.PodPhase(ins.Status)
	dmoPod.GPU = ins.RequestGPUs

	return dmoPod, nil
}
