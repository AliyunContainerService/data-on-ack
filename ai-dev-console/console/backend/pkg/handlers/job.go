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
	"fmt"
	clientregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/tenant"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"sort"
	"strconv"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	consoleutils "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultUser = "Anonymous"
)

func NewJobHandler(objStorage, clientTypeName string, logHandler *LogHandler) (*JobHandler, error) {
	objBackend := registry.GetObjectBackend(objStorage)
	if objBackend == nil {
		return nil, fmt.Errorf("no object backend storage named: %s", objStorage)
	}
	err := objBackend.Initialize()
	if err != nil {
		return nil, err
	}
	clientBackend := registry.GetActionBackend(clientTypeName)
	if clientBackend == nil {
		return nil, fmt.Errorf("no action backend named: %s", objStorage)
	}
	err = clientBackend.Initialize()
	if err != nil {
		return nil, err
	}
	return &JobHandler{
		logHandler:    logHandler,
		objectBackend: objBackend,
		clientBackend: clientBackend,
		client:        clientmgr.GetCtrlClient(),
		preSubmitHooks: []preSubmitHook{
			tfJobPreSubmitAutoConvertReplicas,
			pytorchJobPreSubmitAutoConvertReplicas,
		},
	}, nil
}

type JobHandler struct {
	client         client.Client
	logHandler     *LogHandler
	objectBackend  backends.ObjectStorageBackend
	clientBackend  backends.ObjectClientBackend
	preSubmitHooks []preSubmitHook
}

func (jh *JobHandler) GetJobFromBackend(userName, ns, jobId, jobName, kind, region string) (model.JobInfo, error) {
	job, err := jh.objectBackend.UserName(userName).ReadJob(ns, jobName, jobId, kind, region)
	if err != nil {
		return model.JobInfo{}, err
	}
	jobInfo := model.ConvertDMOJobToJobInfo(job)
	return jobInfo, nil
}

func (jh *JobHandler) ListJobsFromBackend(query *backends.Query) ([]model.JobInfo, error) {
	klog.Infof("list job by userName:%s", query.UserName)
	jobs, err := jh.objectBackend.UserName(query.UserName).ListJobs(query)
	if err != nil {
		return nil, err
	}
	jobInfos := make([]model.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		jobInfos = append(jobInfos, model.ConvertDMOJobToJobInfo(job))
	}
	return jobInfos, nil
}

func (jh *JobHandler) GetDetailedJobFromBackend(userName, ns, name, jobID, kind, region string) (model.JobInfo, error) {
	klog.Infof("[JobHandler.GetDetailedJobFromBackend] userName:%s ns:%s name:%s jobID:%s kind:%s region:%s",
		userName, ns, name, jobID, kind, region)
	job, err := jh.objectBackend.UserName(userName).ReadJob(ns, name, jobID, kind, region)
	if err != nil {
		klog.Errorf("failed to get job from backend, err: %v", err)
		return model.JobInfo{}, err
	}
	pods, err := jh.objectBackend.UserName(userName).ListPods(ns, kind, "", jobID)
	if err != nil {
		klog.Errorf("failed to list pods from backend, err: %v", err)
		return model.JobInfo{}, err
	}

	var (
		specs               = make([]model.Spec, 0, len(pods))
		specReplicaStatuses = make(map[string]*model.SpecReplicaStatus)
	)

	for _, pod := range pods {
		if _, ok := specReplicaStatuses[pod.ReplicaType]; !ok {
			specReplicaStatuses[pod.ReplicaType] = &model.SpecReplicaStatus{}
		}

		pod.GmtPodFinished = job.GmtJobFinished

		switch pod.Status {
		case corev1.PodSucceeded:
			specReplicaStatuses[pod.ReplicaType].Succeeded++
		case corev1.PodFailed:
			specReplicaStatuses[pod.ReplicaType].Failed++
		case utils.PodStopped:
			specReplicaStatuses[pod.ReplicaType].Stopped++
		default:
			specReplicaStatuses[pod.ReplicaType].Active++
		}

		specs = append(specs, model.ConvertDMOPodToJobSpec(pod))
	}

	jobInfo := model.ConvertDMOJobToJobInfo(job)
	jobInfo.Specs = specs
	jobInfo.SpecsReplicaStatuses = specReplicaStatuses

	return jobInfo, nil
}

func (jh *JobHandler) StopJob(userName, ns, name, jobID, kind, region string) error {
	return jh.clientBackend.UserName(userName).StopJob(ns, name, jobID, kind)
}

func (jh *JobHandler) DeleteJobFromStorage(userName, ns, name, jobID, kind, region string) error {
	return jh.objectBackend.UserName(userName).RemoveJobRecord(ns, name, jobID, kind, region)
}

func (jh *JobHandler) GetJobYamlData(userId, ns, name, kind string) ([]byte, error) {
	job := consoleutils.InitJobRuntimeObjectByKind(kind)
	if job == nil {
		return nil, fmt.Errorf("unsupported job kind: %s", kind)
	}

	ctrlClient, err := clientregistry.GetCtrlClient(userId)
	if err != nil {
		return nil, err
	}

	err = ctrlClient.Get(context.Background(), types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, job)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(job)
}

func (jh *JobHandler) GetJobJsonData(userName, ns, name, kind string) ([]byte, error) {
	klog.Infof("[JobHandler.GetJobJsonData] userName:%s ns:%s name:%s kind:%s", userName, ns, name, kind)
	job := consoleutils.InitJobRuntimeObjectByKind(kind)
	if job == nil {
		return nil, fmt.Errorf("unsupported job kind: %s", kind)
	}

	ctrlClient, err := clientregistry.GetCtrlClient(userName)
	if err != nil {
		klog.Errorf("get ctrl client failed, err: %v", err)
		return nil, err
	}

	err = ctrlClient.Get(context.Background(), types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, job)

	if err != nil {
		klog.Errorf("get job failed, err:%v", err)
		return nil, err
	}

	return json.Marshal(job)
}

func (jh *JobHandler) SubmitJob(userName string, jobInfo *dmo.SubmitJobInfo) error {
	return jh.clientBackend.UserName(userName).SubmitJob(jobInfo)
}

func (jh *JobHandler) SubmitJobWithKind(userName string, data []byte, kind string) error {
	job := model.SubmitJobArgs{}
	err := json.Unmarshal(data, &job)
	if err != nil {
		return err
	}

	klog.Infof("received submit job args: %v", string(data))
	return jh.clientBackend.UserName(userName).SubmitJob(&job.SubmitJobInfo)
}

func (jh *JobHandler) submitJob(job client.Object) error {
	for _, hook := range jh.preSubmitHooks {
		hook(job)
	}

	return jh.client.Create(context.Background(), job)
}

func (jh *JobHandler) ListPVC(ns string) ([]string, error) {
	list := &corev1.PersistentVolumeClaimList{}
	if err := jh.client.List(context.TODO(), list, &client.ListOptions{Namespace: ns}); err != nil {
		return nil, err
	}
	ret := make([]string, 0, len(list.Items))
	for _, pvc := range list.Items {
		ret = append(ret, pvc.Name)
	}
	return ret, nil
}

func (jh *JobHandler) GetJobStatisticsFromBackend(query *backends.Query) (model.JobStatistics, error) {
	jobStatistics := model.JobStatistics{}
	jobInfos, err := jh.ListJobsFromBackend(query)
	if err != nil {
		return jobStatistics, err
	}

	historyJobsMap := make(map[string]*model.HistoryJobStatistic)
	totalJobCount := int32(0)
	for _, jobInfo := range jobInfos {
		userID := jobInfo.JobUserID
		if len(userID) == 0 {
			userID = defaultUser
		}

		userName := jobInfo.JobUserName
		if len(userName) == 0 {
			userName = userID
		}

		namespace := jobInfo.Namespace

		if _, ok := historyJobsMap[namespace]; !ok {
			historyJobsMap[namespace] = &model.HistoryJobStatistic{}
		}
		historyJobsMap[namespace].UserName = userName
		historyJobsMap[namespace].UserID = userID
		historyJobsMap[namespace].Namespace = namespace
		historyJobsMap[namespace].JobCount++
		totalJobCount++
	}

	jobStatistics.TotalJobCount = totalJobCount
	for _, stat := range historyJobsMap {
		ratio := float64(stat.JobCount*100) / float64(totalJobCount)
		stat.JobRatio, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", ratio), 64)
		jobStatistics.HistoryJobs = append(jobStatistics.HistoryJobs, stat)
	}

	// Sort history jobs by job ratio
	sort.SliceStable(jobStatistics.HistoryJobs, func(i, j int) bool {
		return jobStatistics.HistoryJobs[i].JobRatio > jobStatistics.HistoryJobs[j].JobRatio
	})

	jobStatistics.StartTime = query.StartTime.Format(time.RFC3339)
	jobStatistics.EndTime = query.EndTime.Format(time.RFC3339)

	return jobStatistics, nil
}

func (jh *JobHandler) GetRunningJobsFromBackend(query *backends.Query) ([]model.JobInfo, error) {
	runningJobs, err := jh.ListJobsFromBackend(query)
	if err != nil {
		return runningJobs, err
	}

	/*
		// sort by job resource
		sort.SliceStable(runningJobs, func(i, j int) bool {
			return runningJobs[i].JobResource.TotalGPU > runningJobs[j].JobResource.TotalGPU ||
				runningJobs[i].JobResource.TotalCPU > runningJobs[j].JobResource.TotalCPU ||
				runningJobs[i].JobResource.TotalMemory > runningJobs[j].JobResource.TotalMemory
		})
	*/

	return runningJobs, nil
}
