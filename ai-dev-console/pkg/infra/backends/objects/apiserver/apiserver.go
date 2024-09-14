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

package apiserver

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	appsv1alpha1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/apps/v1alpha1"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"
	batch "k8s.io/api/batch/v1"

	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo/converters"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util/workloadgate"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAPIServerBackendService() backends.ObjectStorageBackend {
	return &apiServerBackend{client: clientmgr.GetCtrlClient()}
}

var (
	_        backends.ObjectStorageBackend = &apiServerBackend{}
	allKinds []string
	once     sync.Once
)

type apiServerBackend struct {
	client client.Client
}

func (a *apiServerBackend) ListModels(query *backends.ModelsQuery) ([]*dmo.Model, error) {
	return nil, nil
}

func (a *apiServerBackend) GetModel(modelID string) (*dmo.Model, error) {
	return nil, nil
}

func (a *apiServerBackend) DeleteModel(modelID string) error {
	return nil
}

func (a *apiServerBackend) WriteModel(model *dmo.Model) error {
	return nil
}

func (a *apiServerBackend) ListEvaluateJobs(query *backends.EvaluateJobQuery) ([]*dmo.EvaluateJob, error) {
	return nil, nil
}

func (a *apiServerBackend) GetEvaluateJob(ns, name, evaluateJobID string) (*dmo.EvaluateJob, error) {
	return nil, nil
}

func (a *apiServerBackend) DeleteEvaluateJob(ns, name, evaluateJobID string) error {
	return nil
}

func (a *apiServerBackend) WriteEvaluateJob(evaluateJob *batch.Job, PV_OSMap map[string]string) error {
	return nil
}

func (a *apiServerBackend) UpdateNotebookToken(namespace, name, token string) error {
	return nil
}

func (a *apiServerBackend) GetNotebook(namespace, name string) (*dmo.Notebook, error) {
	return nil, nil
}

func (a *apiServerBackend) ListAllNotebook(query *backends.NotebookQuery) ([]*dmo.Notebook, error) {
	return nil, nil
}

func (a *apiServerBackend) ListNotebook(query *backends.NotebookQuery) ([]*dmo.Notebook, error) {
	return nil, nil
}

func (a *apiServerBackend) DeleteNotebook(namespace, name string) error {
	return nil
}

func (a *apiServerBackend) WriteNotebook(notebook *v1.Notebook) error {
	return nil
}

func (a *apiServerBackend) Initialize() error {
	var e error
	once.Do(func() {
		fn := func(obj runtime.Object) []string {
			meta, err := apimeta.Accessor(obj)
			if err != nil {
				return []string{}
			}
			return []string{meta.GetName()}
		}
		for _, kind := range []string{training.TFJobKind, training.PyTorchJobKind, training.XDLJobKind, training.XGBoostJobKind} {
			job := initJobWithKind(kind)
			_, enabled := workloadgate.IsWorkloadEnable(job, clientmgr.GetScheme())
			if !enabled {
				continue
			}
			allKinds = append(allKinds, kind)
			if err := clientmgr.IndexField(job, "metadata.name", fn); err != nil {
				e = err
			}
		}
	})
	return e
}

func (a *apiServerBackend) Close() error {
	return nil
}

func (a *apiServerBackend) Name() string {
	return "apiserver"
}

func (a *apiServerBackend) UserName(userName string) backends.ObjectStorageBackend {
	return a
}

func (a *apiServerBackend) WritePod(pod *corev1.Pod) error {
	return nil
}

func (a *apiServerBackend) ListPods(ns, kind, name, jobID string) ([]*dmo.Pod, error) {
	pods := corev1.PodList{}
	err := a.client.List(context.Background(), &pods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{apiv1.JobNameLabel: name}),
		Namespace:     ns,
	})
	if err != nil {
		return nil, err
	}
	dmoPods := make([]*dmo.Pod, 0, len(pods.Items))
	for i := range pods.Items {
		dmoPod, err := converters.ConvertPodToDMOPod(&pods.Items[i])
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

func (a *apiServerBackend) UpdatePodRecordStopped(ns, name, podID string) error {
	pod := corev1.Pod{}
	err := a.client.Get(context.Background(), types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return a.client.Delete(context.Background(), &pod)
}

func (a *apiServerBackend) WriteJob(job metav1.Object, kind string, specs map[apiv1.ReplicaType]*apiv1.ReplicaSpec, runPolicy *apiv1.RunPolicy, jobStatus *apiv1.JobStatus, region string) error {
	return nil
}

func (a *apiServerBackend) ReadJob(ns, name, jobID, kind, region string) (*dmo.Job, error) {
	job := initJobWithKind(kind)
	getter := initJobPropertiesWithKind(kind)
	err := a.client.Get(context.Background(), types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, job)
	if err != nil {
		return nil, err
	}
	metaObj, specs, runPolicy, jobStatus := getter(job)
	enableGPUTopo := runPolicy.GPUTopologyPolicy != nil && runPolicy.GPUTopologyPolicy.IsTopologyAware
	dmoJob, err := converters.ConvertJobToDMOJob(metaObj, kind, specs, jobStatus, region, enableGPUTopo)
	if err != nil {
		return nil, err
	}
	return dmoJob, nil
}

func (a *apiServerBackend) ListJobs(query *backends.Query) ([]*dmo.Job, error) {
	klog.Infof("list jobs with query: %+v", query)
	// List job lists for each job kind.
	var (
		options  client.ListOptions
		filters  []func(job *dmo.Job) bool
		dmoJobs  []*dmo.Job
		jobTypes []string
	)
	if query.Namespace != "" {
		options.Namespace = query.Namespace
	}
	if query.StartTime.IsZero() || query.EndTime.IsZero() {
		return nil, fmt.Errorf("StartTime EndTime should not be empty")
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
	if query.Type != "" {
		if !stringSliceContains(query.Type, allKinds) {
			return nil, fmt.Errorf("unsupported job kind [%s]", query.Type)
		}
		jobTypes = []string{query.Type}
	} else {
		jobTypes = allKinds
	}
	for _, kind := range jobTypes {
		jobs, err := a.listJobsWithKind(kind, query.Name, query.RegionID, options, filters...)
		if err != nil {
			return nil, err
		}
		dmoJobs = append(dmoJobs, jobs...)
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

func (a *apiServerBackend) UpdateJobRecordStopped(ns, name, jobID, kind, region string) error {
	job := initJobWithKind(kind)
	err := a.client.Get(context.Background(), types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return a.client.Delete(context.Background(), job)
}

func (a *apiServerBackend) RemoveJobRecord(ns, name, jobID, kind, region string) error {
	return a.UpdateJobRecordStopped(ns, name, "", kind, region)
}

func (a *apiServerBackend) ListCrons(query *backends.CronQuery) ([]*dmo.Cron, error) {
	//TODO
	return nil, nil
}

func (a *apiServerBackend) GetCron(ns, name, cronID string) (*dmo.Cron, error) {
	//TODO
	return nil, nil
}

func (a *apiServerBackend) DeleteCron(ns, name, cronID string) error {
	//TODO
	return nil
}

func (a *apiServerBackend) WriteCron(cron *appsv1alpha1.Cron) error {
	//TODO
	return nil
}

func (a *apiServerBackend) ListCronHistories(ns, name, jobName, jobStatus, cronID string) ([]*dmo.Job, error) {
	//TODO
	return nil, nil
}

func (a *apiServerBackend) listJobsWithKind(kind string, nameLike, region string, options client.ListOptions, filters ...func(*dmo.Job) bool) ([]*dmo.Job, error) {
	list, lister := initJobListWithKind(kind)
	if err := a.client.List(context.Background(), list, &options); err != nil {
		return nil, err
	}
	jobs := lister(list)
	getter := initJobPropertiesWithKind(kind)
	dmoJobs := make([]*dmo.Job, 0, len(jobs))
	for _, job := range jobs {
		metaObj, specs, runPolicy, jobStatus := getter(job)
		if nameLike != "" && !strings.Contains(metaObj.GetName(), nameLike) {
			continue
		}
		enableGPUTopo := runPolicy.GPUTopologyPolicy != nil && runPolicy.GPUTopologyPolicy.IsTopologyAware
		dmoJob, err := converters.ConvertJobToDMOJob(metaObj, kind, specs, jobStatus, region, enableGPUTopo)
		if err != nil {
			return nil, err
		}
		skip := false
		for _, filter := range filters {
			if !filter(dmoJob) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		dmoJobs = append(dmoJobs, dmoJob)
	}
	return dmoJobs, nil
}

func initJobWithKind(kind string) (job runtime.Object) {
	switch kind {
	case training.TFJobKind:
		job = &training.TFJob{}
	case training.PyTorchJobKind:
		job = &training.PyTorchJob{}
	case training.XDLJobKind:
		job = &training.XDLJob{}
	case training.XGBoostJobKind:
		job = &training.XGBoostJob{}
	}
	return
}

type jobPropertiesGetter func(obj runtime.Object) (metav1.Object, map[apiv1.ReplicaType]*apiv1.ReplicaSpec, apiv1.RunPolicy, *apiv1.JobStatus)

func initJobPropertiesWithKind(kind string) (getter jobPropertiesGetter) {
	switch kind {
	case training.TFJobKind:
		getter = func(obj runtime.Object) (metav1.Object, map[apiv1.ReplicaType]*apiv1.ReplicaSpec, apiv1.RunPolicy, *apiv1.JobStatus) {
			tfJob := obj.(*training.TFJob)
			return tfJob, tfJob.Spec.TFReplicaSpecs, tfJob.Spec.RunPolicy, &tfJob.Status
		}
	case training.PyTorchJobKind:
		getter = func(obj runtime.Object) (metav1.Object, map[apiv1.ReplicaType]*apiv1.ReplicaSpec, apiv1.RunPolicy, *apiv1.JobStatus) {
			pytorchJob := obj.(*training.PyTorchJob)
			return pytorchJob, pytorchJob.Spec.PyTorchReplicaSpecs, pytorchJob.Spec.RunPolicy, &pytorchJob.Status
		}
	case training.XDLJobKind:
		getter = func(obj runtime.Object) (metav1.Object, map[apiv1.ReplicaType]*apiv1.ReplicaSpec, apiv1.RunPolicy, *apiv1.JobStatus) {
			xdlJob := obj.(*training.XDLJob)
			return xdlJob, xdlJob.Spec.XDLReplicaSpecs, xdlJob.Spec.RunPolicy, &xdlJob.Status
		}
	case training.XGBoostJobKind:
		getter = func(obj runtime.Object) (metav1.Object, map[apiv1.ReplicaType]*apiv1.ReplicaSpec, apiv1.RunPolicy, *apiv1.JobStatus) {
			xgboostJob := obj.(*training.XGBoostJob)
			return xgboostJob, xgboostJob.Spec.XGBReplicaSpecs, xgboostJob.Spec.RunPolicy, &xgboostJob.Status.JobStatus
		}
	}
	return
}

type jobLister func(list runtime.Object) []runtime.Object

func initJobListWithKind(kind string) (list runtime.Object, lister jobLister) {
	switch kind {
	case training.TFJobKind:
		list = &training.TFJobList{}
		lister = func(list runtime.Object) []runtime.Object {
			tfList := list.(*training.TFJobList)
			jobs := make([]runtime.Object, 0, len(tfList.Items))
			for i := range tfList.Items {
				jobs = append(jobs, &tfList.Items[i])
			}
			return jobs
		}
	case training.PyTorchJobKind:
		list = &training.PyTorchJobList{}
		lister = func(list runtime.Object) []runtime.Object {
			pytorchList := list.(*training.PyTorchJobList)
			jobs := make([]runtime.Object, 0, len(pytorchList.Items))
			for i := range pytorchList.Items {
				jobs = append(jobs, &pytorchList.Items[i])
			}
			return jobs
		}
	case training.XDLJobKind:
		list = &training.XDLJobList{}
		lister = func(list runtime.Object) []runtime.Object {
			xdlList := list.(*training.XDLJobList)
			jobs := make([]runtime.Object, 0, len(xdlList.Items))
			for i := range xdlList.Items {
				jobs = append(jobs, &xdlList.Items[i])
			}
			return jobs
		}
	case training.XGBoostJobKind:
		list = &training.XGBoostJobList{}
		lister = func(list runtime.Object) []runtime.Object {
			xgboostList := list.(*training.XGBoostJobList)
			jobs := make([]runtime.Object, 0, len(xgboostList.Items))
			for i := range xgboostList.Items {
				jobs = append(jobs, &xgboostList.Items[i])
			}
			return jobs
		}
	}
	return
}

func stringSliceContains(val string, slice []string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
