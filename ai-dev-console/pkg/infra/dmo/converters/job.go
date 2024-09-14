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
	"fmt"
	"strings"
	"time"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util"
	"github.com/kubeflow/arena/pkg/apis/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"k8s.io/utils/pointer"
)

const (
	RemarkEnableTensorBoard = "EnableTensorBoard"
	ArenaConsoleUserLabel   = "arena.kubeflow.org/console-user"
)

// ConvertJobToDMOJob converts a native job object to dmo job.
func ConvertJobToDMOJob(job metav1.Object, kind string, specs map[v1.ReplicaType]*v1.ReplicaSpec, jobStatus *v1.JobStatus, region string, enableGPUTopo bool) (*dmo.Job, error) {
	klog.V(5).Infof("[ConvertJobToDMOJob] kind: %s, job: %s/%s", kind, job.GetNamespace(), job.GetName())
	dmoJob := dmo.Job{
		Name:            job.GetName(),
		Namespace:       job.GetNamespace(),
		UID:             string(job.GetUID()),
		EtcdVersion:     job.GetResourceVersion(),
		Kind:            kind,
		GmtJobSubmitted: job.GetCreationTimestamp().Time,
	}

	labels := job.GetLabels()
	if createdBy, ok := labels["createdBy"]; ok {
		dmoJob.CreatedBy = &createdBy
	}

	if uid, ok := labels[ArenaConsoleUserLabel]; ok {
		dmoJob.User = &uid
		//user, err := utils.GetUserByName(name)
		//if err == nil {
		//	dmoJob.User = &user.Spec.UserName
		//}
	} else if userId, err := util.GetUserIdFromAnnotations(job.GetAnnotations()); err == nil && userId != "" {
		dmoJob.User = &userId
	}

	if enableGPUTopo {
		enabled := int8(1)
		dmoJob.EnableGPUTopologyAware = &enabled
	}

	if region != "" {
		dmoJob.RegionID = &region
	}

	/*
		if tn, err := tenancy.GetTenancy(job); err == nil && tn != nil {
			dmoJob.Tenant = &tn.Tenant
			dmoJob.Group = &tn.Group
			dmoJob.User = &tn.User
			if dmoJob.RegionID == nil && tn.Region != "" {
				dmoJob.RegionID = &tn.Region
			}
			if dmoJob.ClusterID == nil && tn.ClusterID != "" {
				dmoJob.ClusterID = &tn.ClusterID
			}
		} else {
			dmoJob.Tenant = pointer.StringPtr("")
			dmoJob.User = pointer.StringPtr("")
		}

		serviceAccount := job.GetAnnotations()["arena.kubeflow.org/username"]
		if serviceAccount != "" {
			strArray := strings.Split(serviceAccount, ":")
			if len(strArray) == 4 {
				dmoJob.User = pointer.StringPtr(strArray[3])
			}
		}
	*/

	dmoJob.Status = v1.JobCreated
	if condLen := len(jobStatus.Conditions); condLen > 0 {
		dmoJob.Status = jobStatus.Conditions[condLen-1].Type
	}
	if runningCond := util.GetCondition(*jobStatus, v1.JobRunning); runningCond != nil {
		dmoJob.GmtJobRunning = &runningCond.LastTransitionTime.Time
	}
	if finishTime := jobStatus.CompletionTime; finishTime != nil {
		dmoJob.GmtJobFinished = &finishTime.Time
	}

	if len(jobStatus.Conditions) > 0 {
		cond := jobStatus.Conditions[len(jobStatus.Conditions)-1]
		dmoJob.ReasonCode = pointer.StringPtr(cond.Reason)
		dmoJob.Reason = pointer.StringPtr(cond.Message)
	}
	dmoJob.IsDeleted = util.IntPtr(0)
	dmoJob.IsInK8s = 1
	extends := make([]string, 0)

	if job.GetAnnotations()[v1.AnnotationTensorBoardConfig] != "" {
		extends = append(extends, RemarkEnableTensorBoard)
	}
	if len(extends) > 0 {
		dmoJob.Extended = pointer.StringPtr(strings.Join(extends, ","))
	}

	dump, _ := json.Marshal(job)
	dmoJob.JobJson = string(dump)

	resources := computeJobResources(specs)
	resourcesBytes, err := json.Marshal(&resources)
	if err != nil {
		return nil, err
	}
	dmoJob.Resources = string(resourcesBytes)

	jobConfig := computeJobConfig(job, specs)
	jobConfigBytes, err := json.Marshal(&jobConfig)
	if err != nil {
		return nil, err
	}
	dmoJob.JobConfig = string(jobConfigBytes)
	return &dmoJob, nil
}

// ExtractTypedJobInfos extract common-api struct and infos from different typed job objects.
func ExtractTypedJobInfos(job metav1.Object) (kind string, spec map[v1.ReplicaType]*v1.ReplicaSpec, status v1.JobStatus, err error) {
	switch typed := job.(type) {
	case *training.TFJob:
		return training.TFJobKind, typed.Spec.TFReplicaSpecs, typed.Status, nil
	case *training.PyTorchJob:
		return training.PyTorchJobKind, typed.Spec.PyTorchReplicaSpecs, typed.Status, nil
	case *training.XGBoostJob:
		return training.XGBoostJobKind, typed.Spec.XGBReplicaSpecs, typed.Status.JobStatus, nil
	case *training.XDLJob:
		return training.XDLJobKind, typed.Spec.XDLReplicaSpecs, typed.Status, nil
	}
	return "", nil, v1.JobStatus{}, fmt.Errorf("unkonwn job kind, %s/%s", job.GetNamespace(), job.GetName())
}

type replicaResources struct {
	Resources corev1.ResourceRequirements `json:"resources"`
	Replicas  int32                       `json:"replicas"`
}

type replicaResourcesMap map[v1.ReplicaType]replicaResources

func computeJobResources(specs map[v1.ReplicaType]*v1.ReplicaSpec) replicaResourcesMap {
	resources := make(replicaResourcesMap)
	for rtype, spec := range specs {
		specResources := computePodResources(&spec.Template.Spec)
		rr := replicaResources{Resources: specResources}
		if spec.Replicas != nil {
			rr.Replicas = *spec.Replicas
		} else {
			rr.Replicas = 1
		}
		resources[rtype] = rr
	}
	return resources
}

type jobConfig struct {
	CodeBindings map[string]string `json:"code_bindings,omitempty"`
	DataBindings []string          `json:"data_bindings,omitempty"`
	Commands     []string          `json:"commands,omitempty"`
}

func computeJobConfig(job metav1.Object, specs map[v1.ReplicaType]*v1.ReplicaSpec) jobConfig {
	var config jobConfig

	// TODO(benjin.mbj) get codeBindings and dataBindings after DataManager feature is ready
	// See: https://aone.alibaba-inc.com/req/30877762
	codeBindings := make(map[string]string, 0)
	gitSyncConfigJSON := job.GetAnnotations()[v1.AnnotationGitSyncConfig]
	if gitSyncConfigJSON != "" {
		err := json.Unmarshal([]byte(gitSyncConfigJSON), &codeBindings)
		if err != nil {
			klog.Errorf("gitSyncConfig json Unmarshal err, json: %s, err: %v", gitSyncConfigJSON, err)
			return config
		}
	}
	config.CodeBindings = codeBindings

	// get dataBindings from first spec
	dataBindings := make([]string, 0)
	commands := make([]string, 0)
	alreadyGetCommand := false
	alreadyGetDataBindings := false
	for _, spec := range specs {
		if !alreadyGetDataBindings {
			for _, volume := range spec.Template.Spec.Volumes {
				dataBindings = append(dataBindings, volume.Name)
				alreadyGetDataBindings = true
			}
		}

		if !alreadyGetCommand {
			// get command config from first container
			for _, container := range spec.Template.Spec.Containers {
				commands = container.Command
				alreadyGetCommand = true
				break
			}
		}

		if alreadyGetDataBindings && alreadyGetCommand {
			break
		}
	}
	config.DataBindings = dataBindings
	config.Commands = commands

	return config
}

// ConvertArenaJobToDMOJob converts a native pod object to dmo pod.
func ConvertArenaJobToDMOJob(job *types.TrainingJobInfo) (*dmo.Job, error) {
	dmoJob := &dmo.Job{
		Namespace:       job.Namespace,
		Name:            job.Name,
		Status:          utils.GetJobStatusFromArenaStatus(job.Status),
		Kind:            utils.GetKindFromArenaJobType(job.Trainer),
		GmtCreated:      time.Unix(job.CreationTimestamp, 0),
		GmtJobSubmitted: time.Unix(job.CreationTimestamp, 0),
	}

	if dmoJob.Status == v1.JobSucceeded || dmoJob.Status == v1.JobFailed {
		duration, err := time.ParseDuration(job.Duration)
		if err != nil {
			return nil, err
		}
		finishedTime := dmoJob.GmtCreated.Add(duration)
		dmoJob.GmtJobFinished = &finishedTime
	}

	return dmoJob, nil
}
