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
    
package model

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo/converters"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/resource_utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	quota "k8s.io/kubernetes/pkg/quota/v1"
)

const (
	TimeFormat = "2006-01-02 15:04:05"
	//TimeFormat = time.RFC3339
)

func ConvertDMOCronToCronInfo(cron *dmo.Cron) CronInfo {
	cronInfo := CronInfo{
		Name:              cron.Name,
		Namespace:         cron.Namespace,
		Kind:              cron.Kind,
		Schedule:          cron.Schedule,
		ConcurrencyPolicy: cron.ConcurrencyPolicy,
		Status:            cron.Status,
		CreateTime:        time2Str(cron.GmtCreated),
	}

	if cron.HistoryLimit != nil {
		cronInfo.HistoryLimit = int64(*cron.HistoryLimit)
	}

	if cron.Deadline != nil {
		cronInfo.Deadline = time2Str(*cron.Deadline)
	}

	if cron.LastScheduleTime != nil {
		cronInfo.LastScheduleTime = time2Str(*cron.LastScheduleTime)
	}

	if cron.Suspend != nil && *cron.Suspend == 1 {
		cronInfo.Status = "Suspend"
	} else {
		cronInfo.Status = "Running"
	}

	return cronInfo
}

func ConvertDMOJobToJobInfo(dmoJob *dmo.Job) JobInfo {
	jobInfo := JobInfo{
		Id:        dmoJob.UID,
		Name:      dmoJob.Name,
		JobType:   dmoJob.Kind,
		JobStatus: dmoJob.Status,
		Namespace: dmoJob.Namespace,
		Resources: dmoJob.Resources,
		JobConfig: dmoJob.JobConfig,
	}
	if dmoJob.RegionID != nil {
		jobInfo.DeployRegion = *dmoJob.RegionID
	}

	if !dmoJob.GmtJobSubmitted.IsZero() {
		jobInfo.CreateTime = time2Str(dmoJob.GmtJobSubmitted)
	}
	if !util.Time(dmoJob.GmtJobFinished).IsZero() {
		jobInfo.EndTime = time2Str(*dmoJob.GmtJobFinished)
	}
	if !dmoJob.GmtJobSubmitted.IsZero() && !util.Time(dmoJob.GmtJobFinished).IsZero() {
		jobInfo.DurationTime = GetTimeDiffer(dmoJob.GmtJobSubmitted, *dmoJob.GmtJobFinished)
	}
	if dmoJob.Extended != nil {
		for _, remark := range strings.Split(*dmoJob.Extended, ",") {
			if strings.TrimSpace(remark) == converters.RemarkEnableTensorBoard {
				jobInfo.EnableTensorboard = true
				break
			}
		}
	}

	/*
		if dmoJob.User != nil {
			userID := *dmoJob.User
			userInfo, err := GetUserInfoFromConfigMap(userID)
			if err != nil {
				userInfo.Uid = userID
			}

			jobInfo.JobUserID = userInfo.Uid
			jobInfo.JobUserName = userInfo.LoginName
			if userInfo.Upn != "" {
				jobInfo.JobUserName = userInfo.Upn
			}
		}
	*/

	if dmoJob.User != nil {
		userId := *dmoJob.User
		userInfo, err := GetUserInfoFromConfigMap(userId)
		if err == nil {
			jobInfo.JobUserID = userInfo.Uid
			jobInfo.JobUserName = userInfo.LoginName
			if userInfo.Upn != "" {
				jobInfo.JobUserName = userInfo.Upn
			}
		}
	}

	if len(jobInfo.Resources) > 0 {
		jobResource, err := calculateJobResources(jobInfo.Resources)
		if err != nil {
			klog.Errorf("computeJobResources failed, err: %v", err)
		}

		jobInfo.JobResource = JobResource{
			TotalCPU:    jobResource.Cpu().MilliValue(),
			TotalMemory: jobResource.Memory().Value(),
			TotalGPU:    resource_utils.GetGpuResource(jobResource).MilliValue(),
		}
	}

	return jobInfo
}

func ConvertDMOPodToJobSpec(pod *dmo.Pod) Spec {
	spec := Spec{
		Name:        pod.Name,
		PodId:       pod.UID,
		ReplicaType: pod.ReplicaType,
		Status:      pod.Status,
		GPU:         pod.GPU,
	}
	if pod.PodIP != nil {
		spec.ContainerIp = *pod.PodIP
	}
	if pod.HostIP != nil {
		spec.HostIp = *pod.HostIP
	}
	if pod.Extended != nil {
		spec.Remark = *pod.Extended
	}
	if !pod.GmtCreated.IsZero() {
		spec.CreateTime = time2Str(pod.GmtCreated)
	}
	if pod.GmtPodRunning != nil && !pod.GmtPodRunning.IsZero() {
		spec.StartTime = time2Str(*pod.GmtPodRunning)
	}
	if !util.Time(pod.GmtPodFinished).IsZero() {
		spec.EndTime = time2Str(*pod.GmtPodFinished)
	}
	if !pod.GmtCreated.IsZero() && !util.Time(pod.GmtPodFinished).IsZero() {
		spec.DurationTime = GetTimeDiffer(pod.GmtCreated, *pod.GmtPodFinished)
	}
	return spec
}

func ConvertDMOEvaluateJobToEvaluateJobInfo(evaluateJob *dmo.EvaluateJob) EvaluateJobInfo {
	evaluateJobInfo := EvaluateJobInfo{
		ID:           evaluateJob.JobID,
		Name:         evaluateJob.Name,
		Namespace:    evaluateJob.Namespace,
		Image:        evaluateJob.Image,
		ModelVersion: evaluateJob.ModelVersion,
		ModelName:    evaluateJob.ModelName,
		Metrics:      evaluateJob.Metrics,
		Status:       string(evaluateJob.Status),
		CreateTime:   time2Str(evaluateJob.GmtCreated),
		ModifiedTime: time2Str(evaluateJob.GmtModified),
	}
	return evaluateJobInfo
}

// GetTimeDiffer computes time differ duration between 2 time values, formated as
// 2h2m2s.
func GetTimeDiffer(startTime time.Time, endTime time.Time) (differ string) {
	seconds := endTime.Sub(startTime).Seconds()
	var buffer bytes.Buffer
	hours := math.Floor(seconds / 3600)
	if hours > 0 {
		buffer.WriteString(strconv.FormatFloat(hours, 'g', -1, 64))
		buffer.WriteString("h")
		seconds = seconds - 3600*hours
	}
	minutes := math.Floor(seconds / 60)
	if minutes > 0 {
		buffer.WriteString(strconv.FormatFloat(minutes, 'g', -1, 64))
		buffer.WriteString("m")
		seconds = seconds - 60*minutes
	}
	buffer.WriteString(strconv.FormatFloat(seconds, 'g', -1, 64))
	buffer.WriteString("s")
	return buffer.String()
}

type replicaResources struct {
	Resources corev1.ResourceRequirements `json:"resources"`
	Replicas  int32                       `json:"replicas"`
}

type replicaResourcesMap map[v1.ReplicaType]replicaResources

func calculateJobResources(resourcesJSON string) (corev1.ResourceList, error) {
	jobResource := make(corev1.ResourceList)
	replicaResources := replicaResourcesMap{}
	err := json.Unmarshal([]byte(resourcesJSON), &replicaResources)
	if err != nil {
		klog.Errorf("Unmarshal resourceJson failed, content: %s, err: %v", resourcesJSON, err)
		return jobResource, err
	}

	for _, replicaResource := range replicaResources {
		replicas := int64(replicaResource.Replicas)
		totalResource := resource_utils.Multiply(replicas, replicaResource.Resources.Limits)
		jobResource = quota.Add(jobResource, totalResource)
	}

	return jobResource, nil
}

func time2Str(t time.Time) string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err == nil {
		return t.In(loc).Format(TimeFormat)
	} else {
		return t.Local().Format(TimeFormat)
	}

}
