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

package arena

import (
	"fmt"
	appsv1alpha1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/apps/v1alpha1"
	clientregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/tenant"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/objects/apiserver"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo/converters"

	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"github.com/kubeflow/arena/pkg/apis/types"
	"k8s.io/klog"
)

func NewArenaBackendService() backends.ObjectStorageBackend {
	return &arenaBackend{
		ObjectStorageBackend: apiserver.NewAPIServerBackendService(),
		arena:                clientmgr.GetArenaClient(),
	}
}

var _ backends.ObjectStorageBackend = &arenaBackend{}

type arenaBackend struct {
	backends.ObjectStorageBackend
	arena    *arenaclient.ArenaClient
	userName string
}

func (a *arenaBackend) UserName(userName string) backends.ObjectStorageBackend {
	copyArenaBackend := &arenaBackend{
		arena:                a.arena,
		ObjectStorageBackend: a.ObjectStorageBackend,
		userName:             userName,
	}
	return copyArenaBackend
}

func (a *arenaBackend) Name() string {
	return "arena"
}

func (a *arenaBackend) getArenaClient() *arenaclient.ArenaClient {
	var arena *arenaclient.ArenaClient
	var err error
	if a.userName == "" {
		arena = a.arena
	} else {
		arena, err = clientregistry.GetArenaClient(a.userName)
		if err != nil {
			klog.Errorf("get arena client of user %s failed, err:%v", a.userName, err)
		}
	}
	return arena
}

func (a *arenaBackend) ListPods(ns, kind, name, jobID string) ([]*dmo.Pod, error) {
	job, err := a.getArenaClient().Training().Namespace(ns).Get(name, utils.GetArenaJobTypeFromKind(kind), false)
	if err != nil {
		klog.Errorf("get job %v/%v error: %v", ns, name, err)
		return nil, err
	}
	dmoPods := make([]*dmo.Pod, 0, len(job.Instances))
	for i := range job.Instances {
		dmoPod, err := converters.ConvertArenaInstanceToDMOPod(job, &job.Instances[i])
		if err != nil {
			return nil, err
		}
		dmoPods = append(dmoPods, dmoPod)
	}

	if len(dmoPods) > 0 {
		// Order by create timestamp.
		sort.SliceStable(dmoPods, func(i, j int) bool {
			if dmoPods[i].ReplicaType != dmoPods[j].ReplicaType {
				return dmoPods[i].ReplicaType < dmoPods[j].ReplicaType
			}
			is := strings.Split(dmoPods[i].Name, "-")
			if len(is) <= 0 {
				return false
			}
			ii, err := strconv.Atoi(is[len(is)-1])
			if err != nil {
				return false
			}
			js := strings.Split(dmoPods[j].Name, "-")
			if len(js) <= 0 {
				return true
			}
			ji, err := strconv.Atoi(js[len(js)-1])
			if err != nil {
				return true
			}
			if ii != ji {
				return ii < ji
			}
			return dmoPods[i].GmtCreated.Before(dmoPods[j].GmtCreated)
		})
	}

	return dmoPods, nil
}

func (a *arenaBackend) ReadJob(ns, name, jobID, kind, region string) (*dmo.Job, error) {
	job, err := a.getArenaClient().Training().Namespace(ns).Get(name, utils.GetArenaJobTypeFromKind(kind), false)
	if err != nil {
		klog.Errorf("get job %v/%v error: %v", ns, name, err)
		return nil, err
	}

	dmoJob, err := converters.ConvertArenaJobToDMOJob(job)
	if err != nil {
		return nil, err
	}
	return dmoJob, nil
}

func (a *arenaBackend) ListJobs(query *backends.Query) ([]*dmo.Job, error) {
	// List job lists for each job kind.
	var (
		filters      []func(job *dmo.Job) bool
		dmoJobs      []*dmo.Job
		jobType      = types.AllTrainingJob
		trainingJobs []*types.TrainingJobInfo
		err          error
	)
	if query.StartTime.IsZero() || query.EndTime.IsZero() {
		return nil, fmt.Errorf("StartTime EndTime should not be empty")
	}
	if query.Type != "" {
		jobType = utils.GetArenaJobTypeFromKind(query.Type)
	}
	if query.Namespace != "" {
		trainingJobs, err = a.getArenaClient().Training().Namespace(query.Namespace).List(false, jobType, false)
	} else {
		for _, namespace := range query.AllocatedNamespaces {
			jobs, err1 := a.getArenaClient().Training().Namespace(namespace).List(false, jobType, false)
			if err1 == nil {
				trainingJobs = append(trainingJobs, jobs...)
			}
		}
	}
	if err != nil {
		return nil, err
	}

	filters = append(filters, func(job *dmo.Job) bool {
		if job.GmtJobSubmitted.Before(query.StartTime) {
			if job.GmtJobFinished == nil || job.GmtJobFinished.IsZero() {
				return true
			}
			if job.GmtJobFinished != nil && job.GmtJobFinished.After(query.StartTime) {
				return true
			}
			return false
		}
		if job.GmtJobSubmitted.Before(query.EndTime) {
			return true
		}
		return false
	})

	if query.Status != "" {
		filters = append(filters, func(job *dmo.Job) bool {
			return strings.ToLower(string(job.Status)) ==
				strings.ToLower(string(query.Status))
		})
	}

	if query.Name != "" {
		filters = append(filters, func(job *dmo.Job) bool {
			return strings.ToLower(job.Name) ==
				strings.ToLower(query.Name)
		})
	}

	if query.Type != "" {
		filters = append(filters, func(job *dmo.Job) bool {
			return strings.ToLower(job.Kind) ==
				strings.ToLower(query.Type)
		})
	}

	for _, j := range trainingJobs {
		job, err := converters.ConvertArenaJobToDMOJob(j)
		if err != nil {
			return nil, err
		}
		skip := false
		for _, filter := range filters {
			if !filter(job) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		dmoJobs = append(dmoJobs, job)
	}

	if len(dmoJobs) > 1 {
		// Order by create timestamp.
		sort.SliceStable(dmoJobs, func(i, j int) bool {
			if dmoJobs[i].GmtJobSubmitted.Equal(dmoJobs[j].GmtJobSubmitted) {
				return dmoJobs[i].Name < dmoJobs[j].Name
			}
			return dmoJobs[i].GmtJobSubmitted.After(dmoJobs[j].GmtJobSubmitted)
		})
	}

	if query.Pagination != nil && len(dmoJobs) > 1 {
		query.Pagination.Count = len(dmoJobs)
		count := query.Pagination.Count
		pageNum := query.Pagination.PageNum
		pageSize := query.Pagination.PageSize
		startIdx := pageSize * (pageNum - 1)
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(dmoJobs)-1 {
			startIdx = len(dmoJobs) - 1
		}
		endIdx := len(dmoJobs)
		if count > 0 {
			endIdx = int(math.Min(float64(startIdx+pageSize), float64(endIdx)))
		}
		klog.Infof("list jobs with pagination, start index: %d, end index: %d", startIdx, endIdx)
		dmoJobs = dmoJobs[startIdx:endIdx]
	}
	return dmoJobs, nil
}

func (a *arenaBackend) StopJob(ns, name, jobID, kind, region string) error {
	return a.RemoveJobRecord(ns, name, jobID, kind, region)
}

func (a *arenaBackend) RemoveJobRecord(ns, name, jobID, kind, region string) error {
	err := a.getArenaClient().Training().Namespace(ns).Delete(utils.GetArenaJobTypeFromKind(kind), name)
	if err != nil {
		klog.Errorf("delete job %v/%v error: %v", ns, name, err)
		return err
	}
	return nil
}

func (a *arenaBackend) ListCrons(query *backends.CronQuery) ([]*dmo.Cron, error) {
	klog.Infof("list cron with query: %+v", query)
	var dmoCrons []*dmo.Cron
	var cronInfos []*types.CronInfo
	var err error

	if query.Namespace != "" {
		cronInfos, err = a.getArenaClient().Cron().Namespace(query.Namespace).List(false)
		if err != nil {
			return nil, err
		}
	} else {
		for _, namespace := range query.AllocatedNamespaces {
			infos, err1 := a.getArenaClient().Cron().Namespace(namespace).List(false)
			if err1 == nil {
				cronInfos = append(cronInfos, infos...)
			}
		}
	}

	for _, cronInfo := range cronInfos {
		if query.Name != "" && cronInfo.Name != query.Name {
			continue
		}

		if query.Type != "" && cronInfo.Type != query.Type {
			continue
		}

		status := "Running"
		if cronInfo.Suspend {
			status = "Suspend"
		}

		if query.Status != "" && query.Status != status {
			continue
		}

		createTime, _ := time.Parse(time.RFC3339, cronInfo.CreationTimestamp)
		if createTime.Before(query.StartTime) || createTime.After(query.EndTime) {
			continue
		}

		historyLimit := int32(cronInfo.HistoryLimit)

		var cron = &dmo.Cron{
			Name:              cronInfo.Name,
			Namespace:         cronInfo.Namespace,
			Kind:              cronInfo.Type,
			Schedule:          cronInfo.Schedule,
			ConcurrencyPolicy: cronInfo.ConcurrencyPolicy,
			Deadline:          str2time(cronInfo.Deadline),
			HistoryLimit:      &historyLimit,
			GmtCreated:        createTime,
			Status:            status,
		}
		dmoCrons = append(dmoCrons, cron)
	}

	if len(dmoCrons) > 1 {
		sort.Slice(dmoCrons, func(i, j int) bool {
			return dmoCrons[i].GmtCreated.After(dmoCrons[j].GmtCreated)
		})
	}

	if query.Pagination != nil && len(dmoCrons) > 1 {
		query.Pagination.Count = len(dmoCrons)
		count := query.Pagination.Count
		pageNum := query.Pagination.PageNum
		pageSize := query.Pagination.PageSize
		startIdx := pageSize * (pageNum - 1)
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx > len(dmoCrons)-1 {
			startIdx = len(dmoCrons) - 1
		}
		endIdx := len(dmoCrons)
		if count > 0 {
			endIdx = int(math.Min(float64(startIdx+pageSize), float64(endIdx)))
		}
		klog.Infof("list crons with pagination, start index: %d, end index: %d", startIdx, endIdx)
		dmoCrons = dmoCrons[startIdx:endIdx]
	}

	return dmoCrons, nil
}

func (a *arenaBackend) GetCron(ns, name, cronID string) (*dmo.Cron, error) {
	klog.Infof("get cron, ns: %s name: %s", ns, name)
	if ns == "" {
		ns = "default"
	}
	cronInfo, err := a.getArenaClient().Cron().Namespace(ns).Get(name)
	if err != nil {
		return nil, err
	}
	status := "Running"
	if cronInfo.Suspend {
		status = "Suspend"
	}

	historyLimit := int32(cronInfo.HistoryLimit)

	cron := &dmo.Cron{
		Name:              cronInfo.Name,
		Namespace:         cronInfo.Namespace,
		Kind:              cronInfo.Type,
		Schedule:          cronInfo.Schedule,
		ConcurrencyPolicy: cronInfo.ConcurrencyPolicy,
		Deadline:          str2time(cronInfo.Deadline),
		HistoryLimit:      &historyLimit,
		GmtCreated:        *str2time(cronInfo.CreationTimestamp),
		Status:            status,
	}
	return cron, nil
}

func (a *arenaBackend) DeleteCron(ns, name, cronID string) error {
	return a.getArenaClient().Cron().Namespace(ns).Delete(name)
}

func (a *arenaBackend) WriteCron(cron *appsv1alpha1.Cron) error {
	return nil
}

func (a *arenaBackend) ListCronHistories(ns, name, jobName, jobStatus, cronID string) ([]*dmo.Job, error) {
	klog.Infof("list cron history, ns:%s name:%s jobName:%s jobStatus:%s", ns, name, jobName, jobStatus)
	cronInfo, err := a.getArenaClient().Cron().Namespace(ns).Get(name)
	if err != nil {
		return nil, err
	}

	var dmoJobs []*dmo.Job
	num := len(cronInfo.History)
	if num > 0 {
		for _, item := range cronInfo.History {
			if jobName != "" && item.Name != jobName {
				continue
			}

			if jobStatus != "" && item.Status != jobStatus {
				continue
			}

			jobCreateTime, _ := time.Parse(time.RFC3339, item.CreateTime)

			var jobFinishedTime time.Time
			if item.FinishTime != "" {
				jobFinishedTime, _ = time.Parse(time.RFC3339, item.CreateTime)
			}

			job := &dmo.Job{
				Name:            item.Name,
				Namespace:       item.Namespace,
				Status:          utils.GetJobStatusFromString(item.Status),
				Kind:            item.Kind,
				GmtCreated:      jobCreateTime,
				GmtJobSubmitted: jobCreateTime,
				GmtJobFinished:  &jobFinishedTime,
			}

			dmoJobs = append(dmoJobs, job)
		}
	}

	if len(dmoJobs) > 1 {
		sort.Slice(dmoJobs, func(i, j int) bool {
			return dmoJobs[i].GmtCreated.After(dmoJobs[j].GmtCreated)
		})
	}

	return dmoJobs, nil
}

func str2time(str string) *time.Time {
	if str == "" {
		return nil
	}

	t, err := time.Parse(time.RFC3339, str)
	if err != nil {
		return nil
	}
	return &t
}
