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

package mysql

import (
	"errors"
	"strconv"
	"sync/atomic"
	"time"

	appsv1alpha1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/apps/v1alpha1"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"
	"github.com/tidwall/gjson"
	batch "k8s.io/api/batch/v1"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo/converters"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/util"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	// initListSize defines the initial capacity when list objects from backend.
	initListSize = 32
)

// errEtcdVersionConflict is returned when a job update loses the optimistic
// lock race, i.e. the row has been advanced to a newer etcd_version by
// another writer between the read and the update. Callers should re-read the
// record and retry.
var errEtcdVersionConflict = errors.New("mysql: job etcd_version conflict, row was updated by another writer")

func NewMysqlBackendService() backends.ObjectStorageBackend {
	klog.Info("use mysql backend for object storage")
	return &mysqlBackend{initialized: 0}
}

var _ backends.ObjectStorageBackend = &mysqlBackend{}

type mysqlBackend struct {
	db          *gorm.DB
	initialized int32
	userName    string
}

func (b *mysqlBackend) ListModels(query *backends.ModelsQuery) ([]*dmo.Model, error) {
	klog.V(3).Infof("[mysql.ListModels] list models, query: %v", query)
	models := make([]*dmo.Model, 0, initListSize)
	db := b.db.Model(&dmo.Model{})

	if query.Pagination != nil {
		db = db.Count(&query.Pagination.Count).
			Limit(query.Pagination.PageSize).
			Offset((query.Pagination.PageNum - 1) * query.Pagination.PageSize)
	}

	if query.ModelName != "" {
		db = db.Where("model_name LIKE ?", "%"+query.ModelName+"%")
	}

	if query.ModelVersion != "" {
		db = db.Where("model_version = ?", query.ModelVersion)
	}
	db = db.Order("id DESC")

	db = db.Find(&models)
	if db.Error != nil {
		return nil, db.Error
	}
	return models, nil
}

func (b *mysqlBackend) GetModel(modelID string) (*dmo.Model, error) {
	model := dmo.Model{}
	id, err := strconv.Atoi(modelID)
	if err != nil {
		return nil, err
	}
	uint64ID := uint64(id)
	query := &dmo.Model{ID: uint64ID}
	result := b.db.Where(query).First(&model)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

func (b *mysqlBackend) DeleteModel(modelID string) error {
	model := dmo.Model{}
	id, err := strconv.Atoi(modelID)
	if err != nil {
		return err
	}
	uint64ID := uint64(id)
	query := &dmo.Model{
		ID: uint64ID,
	}
	err = b.db.Where(query).Delete(&model).Error
	if err != nil {
		klog.Errorf("fail to delete mode : %s", modelID)
		return err
	}
	return nil
}

type Model struct {
	Name       string    `gorm:"type:varchar(256);column:model_name" json:"model_name"`
	Version    string    `gorm:"type:varchar(256);column:model_version" json:"model_version"`
	OSSPath    string    `gorm:"type:varchar(256);column:oss_path" json:"oss_path"`
	JobID      string    `gorm:"type:varchar(256);column:job_id" json:"job_id"`
	GmtCreated time.Time `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
}

func (model Model) TableName() string {
	return "model"
}

func (b *mysqlBackend) WriteModel(model *dmo.Model) error {
	klog.V(3).Infof("create model: %s", model.Name)
	err := b.db.Create(Model{
		Name:       model.Name,
		Version:    model.Version,
		OSSPath:    model.OSSPath,
		JobID:      model.JobID,
		GmtCreated: model.GmtCreated,
	}).Error
	if err != nil {
		klog.Errorf("fail to create model, %s, %s", model.Name, err.Error())
		return err
	}
	return nil
}

func (b *mysqlBackend) ListEvaluateJobs(query *backends.EvaluateJobQuery) ([]*dmo.EvaluateJob, error) {
	klog.V(3).Infof("[mysql.ListEvaluateJobs] list evaluateJobs, query: %v", query)

	evaluateJobs := make([]*dmo.EvaluateJob, 0, initListSize)
	db := b.db.Model(&dmo.EvaluateJob{})
	db = db.Where("gmt_created < ?", query.EndTime).
		Where("gmt_created > ?", query.StartTime).Where("is_deleted = 0 or is_deleted is null")
	db = db.Order("gmt_created DESC")
	if query.Pagination != nil {
		db = db.Count(&query.Pagination.Count).
			Limit(query.Pagination.PageSize).
			Offset((query.Pagination.PageNum - 1) * query.Pagination.PageSize)
	}
	db = db.Find(&evaluateJobs)
	if db.Error != nil {
		return nil, db.Error
	}
	return evaluateJobs, nil
}

func (b *mysqlBackend) GetEvaluateJob(ns, name, evaluateJobID string) (*dmo.EvaluateJob, error) {
	klog.Infof("[mysql.GetEvaluateJob] evaluateJob job_id:%s", evaluateJobID)
	evaluateJob := dmo.EvaluateJob{}
	query := &dmo.EvaluateJob{JobID: evaluateJobID}
	//if evaluateJobID != "" {
	//	query.UID = evaluateJobID
	//}
	result := b.db.Where(query).First(&evaluateJob)
	if result.Error != nil {
		return nil, result.Error
	}
	return &evaluateJob, nil
}

func (b *mysqlBackend) DeleteEvaluateJob(ns, name, evaluateJobID string) error {
	klog.Infof("[mysql.DeleteEvaluateJob] evaluateJob namespace: %s, name: %s, uid: %s", ns, name, evaluateJobID)

	dmoEvaluateJob, err := b.SearchEvaluateJob(ns, name, evaluateJobID)
	if err != nil {
		klog.Errorf("fail to search job %s, error:%s", name, err.Error())
		return err
	}

	dmoEvaluateJob.IsDeleted = 1

	return b.updateEvaluateJob(&dmo.EvaluateJob{Namespace: ns, Name: name, UID: evaluateJobID}, dmoEvaluateJob)
}

func (b *mysqlBackend) SearchEvaluateJob(ns, name, ID string) (*dmo.EvaluateJob, error) {
	klog.Infof("[mysql.SearchEvaluateJob] evaluateJob job_id:%s", ID)
	evaluateJob := dmo.EvaluateJob{}
	query := &dmo.EvaluateJob{Name: name, Namespace: ns}
	if ID != "" {
		query.UID = ID
	}
	result := b.db.Where(query).First(&evaluateJob)
	if result.Error != nil {
		return nil, result.Error
	}
	return &evaluateJob, nil
}

func (b *mysqlBackend) WriteEvaluateJob(evaluateJob *batch.Job, PV_OSMap map[string]string) error {
	klog.V(3).Infof("[mysql.WriteEvaluateJob] evaluateJob namespace: %s, name: %s, uid: %s", evaluateJob.Namespace, evaluateJob.Name, evaluateJob.UID)

	//OSS_temp_path := ""
	//for _, volumeMounts := range evaluateJob.Spec.Template.Spec.Containers[0].VolumeMounts {
	//	if value, ok := PV_OSMap[volumeMounts.Name];ok {
	//		if value != "" {
	//			OSS_temp_path = value + "|" + volumeMounts.MountPath
	//		}
	//	}
	//}

	dmoEvaluateJob := converters.ConvertEvaluateJobToDMOEvaluateJob(evaluateJob)
	dmoEvaluateJob.IsDeleted = 0

	oldEvaluateJob, err := b.SearchEvaluateJob(evaluateJob.Namespace, evaluateJob.Name, string(evaluateJob.UID))
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return b.db.Create(dmoEvaluateJob).Error
		}
		klog.Errorf("fail to search evaluatejob, ns: %s, name: %s, err:%s", evaluateJob.Namespace, evaluateJob.Name, err.Error())
		return err
	}
	return b.updateEvaluateJob(oldEvaluateJob, dmoEvaluateJob)
}

func (b *mysqlBackend) UpdateNotebookToken(namespace, name, token string) error {
	db := b.db
	query := &dmo.Notebook{Namespace: namespace, Name: name}
	return db.Model(&dmo.Notebook{}).Where(query).Update("token", token).Error
}

func (b *mysqlBackend) ListNotebook(query *backends.NotebookQuery) ([]*dmo.Notebook, error) {
	db := b.db
	results := make([]*dmo.Notebook, 0, initListSize)
	//db = db.Model(&dmo.Notebook{}).Where("namespace = ? AND (user_name = '' OR user_name = ? )", query.Namespace, query.UserName)
	if query.UID == "" && query.UserName == "" {
		db = db.Model(&dmo.Notebook{}).Where("namespace = ?", query.Namespace)
	} else {
		db = db.Model(&dmo.Notebook{}).Where("namespace = ? AND (user_name = ? OR user_id = ?)", query.Namespace, query.UserName, query.UID)
	}

	if db.Error != nil {
		return nil, db.Error
	}
	db.Find(&results)
	if db.Error != nil {
		return nil, db.Error
	}
	return results, nil
}

func (b *mysqlBackend) ListAllNotebook(query *backends.NotebookQuery) ([]*dmo.Notebook, error) {
	db := b.db
	results := make([]*dmo.Notebook, 0, initListSize)
	//db = db.Model(&dmo.Notebook{}).Where("namespace = ? AND (user_name = '' OR user_name = ? )", query.Namespace, query.UserName)
	db = db.Model(&dmo.Notebook{}).Where("namespace = ? ", query.Namespace)
	if db.Error != nil {
		return nil, db.Error
	}
	db.Find(&results)
	if db.Error != nil {
		return nil, db.Error
	}
	return results, nil
}

func (b *mysqlBackend) DeleteNotebook(namespace, name string) error {
	query := &dmo.Notebook{Namespace: namespace, Name: name}
	return b.db.Where(query).Delete(&dmo.Notebook{}).Error
}

func (b *mysqlBackend) WriteNotebook(notebook *v1.Notebook) error {
	tempNotebook, dmoNotebook := converters.ConvertNotebookToDMONotebook(notebook)
	oldNotebook, err := b.GetNotebook(notebook.Namespace, notebook.Name)
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			db := b.db.Create(*tempNotebook)
			return db.Error
		}
		return err
	}

	return b.updateNotebook(oldNotebook, dmoNotebook)
}

func (b *mysqlBackend) GetNotebook(namespace, name string) (*dmo.Notebook, error) {
	notebook := dmo.Notebook{}
	query := &dmo.Notebook{Namespace: namespace, Name: name}

	result := b.db.Where(query).First(&notebook)
	if result.Error != nil {
		return nil, result.Error
	}
	return &notebook, nil
}

func (b *mysqlBackend) updateNotebook(oldNotebook, newNotebook *dmo.Notebook) error {
	return b.db.
		Model(oldNotebook).
		Where(&dmo.Notebook{
			Name:      oldNotebook.Name,
			Namespace: oldNotebook.Namespace,
		}).Updates(newNotebook).Error
}

func (b *mysqlBackend) Initialize() error {
	klog.Info("init mysql object backend")
	if atomic.LoadInt32(&b.initialized) == 1 {
		return nil
	}
	if err := b.init(); err != nil {
		return err
	}
	atomic.StoreInt32(&b.initialized, 1)
	return nil
}

func (b *mysqlBackend) Close() error {
	if b.db == nil {
		return nil
	}
	return b.db.Commit().Close()
}

func (b *mysqlBackend) Name() string {
	return "mysql"
}

func (b *mysqlBackend) UserName(userName string) backends.ObjectStorageBackend {
	copiedMysqlBackend := &mysqlBackend{
		db:          b.db,
		initialized: b.initialized,
		userName:    userName,
	}
	return copiedMysqlBackend
}

func (b *mysqlBackend) WritePod(pod *corev1.Pod) error {
	klog.V(3).Infof("[mysql.WritePod] pod: %s/%s", pod.Namespace, pod.Name)
	dmoPod := dmo.Pod{}
	query := &dmo.Pod{UID: string(pod.UID), Namespace: pod.Namespace, Name: pod.Name}

	result := b.db.Where(query).First(&dmoPod)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			return b.createNewPod(pod)
		}
		klog.Errorf("fail to get pod: %s/%s, err:%s", pod.Namespace, pod.Name, result.Error.Error())
		return result.Error
	}

	newPod, err := converters.ConvertPodToDMOPod(pod)
	if err != nil {
		klog.Errorf("fail to convert pod: %s/%s, err:%s", pod.Namespace, pod.Name, err.Error())
		return err
	}
	return b.updatePod(&dmoPod, newPod)
}

func (b *mysqlBackend) ListPods(ns, kind, name, jobID string) ([]*dmo.Pod, error) {
	klog.V(3).Infof("[mysql.ListPods] jobID: %s", jobID)

	podList := make([]*dmo.Pod, 0, initListSize)
	query := &dmo.Pod{Namespace: ns, Name: name, JobUID: jobID}
	result := b.db.Where(query).
		//Order("type").
		Order("CAST(SUBSTRING_INDEX(name, '-', -1) AS SIGNED)").
		Order("gmt_created DESC").
		Find(&podList)
	if result.Error != nil {
		return nil, result.Error
	}
	return podList, nil
}

func (b *mysqlBackend) UpdatePodRecordStopped(ns, name, podID string) error {
	klog.V(3).Infof("[mysql.StopPod] pod: %s/%s/%s", ns, name, podID)

	oldPod := dmo.Pod{}
	if result := b.db.Where(&dmo.Pod{UID: podID, Namespace: ns, Name: name}).First(&oldPod); result.Error != nil {
		klog.Errorf("fail to get pod: %s/%s, err:%s", ns, name, result.Error.Error())
		return result.Error
	}

	newPod := &dmo.Pod{
		EtcdVersion:    oldPod.EtcdVersion,
		Status:         oldPod.Status,
		HostIP:         oldPod.HostIP,
		PodIP:          oldPod.PodIP,
		Extended:       oldPod.Extended,
		GmtCreated:     oldPod.GmtCreated,
		GmtPodRunning:  oldPod.GmtPodRunning,
		GmtPodFinished: oldPod.GmtPodFinished,
	}
	if status := oldPod.Status; status == corev1.PodPending || status == corev1.PodRunning || status == corev1.PodUnknown {
		newPod.Status = utils.PodStopped
		newPod.GmtPodFinished = util.TimePtr(time.Now())
		if newPod.GmtPodRunning == nil || newPod.GmtPodRunning.IsZero() {
			newPod.GmtPodRunning = oldPod.GmtPodRunning
		}
	}

	return b.updatePod(&oldPod, newPod)
}

func (b *mysqlBackend) WriteJob(job metav1.Object, kind string, specs map[apiv1.ReplicaType]*apiv1.ReplicaSpec, runPolicy *apiv1.RunPolicy, jobStatus *apiv1.JobStatus, region string) error {
	klog.V(3).Infof("[mysql.WriteJob] kind: %s job: %s/%s", kind, job.GetNamespace(), job.GetName())

	gpuTopoAware := runPolicy.GPUTopologyPolicy != nil && runPolicy.GPUTopologyPolicy.IsTopologyAware

	dmoJob, err := b.ReadJob(job.GetNamespace(), job.GetName(), string(job.GetUID()), kind, region)
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return b.createNewJob(job, kind, specs, jobStatus, region, gpuTopoAware)
		}
		klog.Errorf("fail to read job : %s/%s, error:%s", job.GetNamespace(), job.GetName(), err.Error())
		return err
	}

	newJob, err := converters.ConvertJobToDMOJob(job, kind, specs, jobStatus, region, gpuTopoAware)
	if err != nil {
		klog.Errorf("fail to convert job : %s/%s, error:%s", job.GetNamespace(), job.GetName(), err.Error())
		return err
	}
	if newJob.GmtCreated.IsZero() {
		newJob.GmtCreated = newJob.GmtJobSubmitted
	}
	return b.updateJob(dmoJob, newJob)
}

func (b *mysqlBackend) ReadJob(ns, name, jobID, kind, region string) (*dmo.Job, error) {
	klog.V(3).Infof("[mysql.ReadJob] jobID: %s", jobID)

	job := dmo.Job{}
	query := &dmo.Job{UID: jobID, Namespace: ns, Name: name, Kind: kind}
	if region != "" {
		query.RegionID = &region
	}
	result := b.db.Where(query).First(&job)
	if result.Error != nil {
		klog.Errorf("fail to read job : %s/%s, error:%s", ns, name, result.Error.Error())

		return nil, result.Error
	}
	return &job, nil
}

func (b *mysqlBackend) ListJobs(query *backends.Query) ([]*dmo.Job, error) {
	klog.V(3).Infof("[mysql.ListJobs] query: %+v", query)

	jobList := make([]*dmo.Job, 0, initListSize)
	db := b.db.Model(&dmo.Job{})
	db = db.Where("gmt_created < ?", query.EndTime).
		Where("gmt_created > ?", query.StartTime)
	if query.Deleted != nil {
		db = db.Where("is_deleted = ?", *query.Deleted)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Namespace != "" {
		db = db.Where("namespace LIKE ?", "%"+query.Namespace+"%")
	} else {
		if len(query.AllocatedNamespaces) == 1 {
			db = db.Where("namespace = ?", query.AllocatedNamespaces[0])
		} else {
			db = db.Where("namespace IN (?)", query.AllocatedNamespaces)
		}
	}
	if query.Type != "" {
		db = db.Where("kind = ?", query.Type)
	}
	if query.JobID != "" {
		db = db.Where("job_id = ?", query.JobID)
	}
	if query.RegionID != "" {
		db = db.Where("region_id = ?", query.RegionID)
	}
	if query.IsCron {
		db = db.Where("created_by = 'Cron'")
	} else {
		db = db.Where("created_by <> 'Cron'")
	}

	if query.UID != "" {
		//ai-dev-console 是为了兼容遗留数据
		//db = db.Where("user_id IN (?, 'ai-dev-console','NULL')", query.UID)
		db = db.Where("user_id = ? or user_id is NULL or user_id = 'ai-dev-console'", query.UID)
	}

	db = db.Order("gmt_created DESC")
	if query.Pagination != nil {
		db = db.Count(&query.Pagination.Count).
			Limit(query.Pagination.PageSize).
			Offset((query.Pagination.PageNum - 1) * query.Pagination.PageSize)
	}
	db = db.Find(&jobList)
	if db.Error != nil {
		return nil, db.Error
	}
	return jobList, nil
}

func (b *mysqlBackend) UpdateJobRecordStopped(ns, name, jobID, kind, region string) error {
	klog.V(3).Infof("[mysql.UpdateJobRecordStopped] jobID: %s, region: %s", jobID, region)

	job, err := b.ReadJob(ns, name, jobID, kind, region)
	if err != nil {
		return err
	}

	newJob := &dmo.Job{
		UID:            job.UID,
		EtcdVersion:    job.EtcdVersion,
		Status:         job.Status,
		RegionID:       job.RegionID,
		IsDeleted:      job.IsDeleted,
		GmtJobFinished: job.GmtJobFinished,
	}
	if status := job.Status; status == apiv1.JobRunning || status == apiv1.JobCreated ||
		status == apiv1.JobRestarting || status == utils.JobStopping || status == utils.JobStopped {
		newJob.Status = utils.JobStopped
		newJob.IsInK8s = 0
		now := time.Now()
		newJob.GmtJobStopped = util.TimePtr(now)
		newJob.GmtJobFinished = util.TimePtr(now)
	}
	return b.updateJob(job, newJob)
}

func (b *mysqlBackend) RemoveJobRecord(ns, name, jobID, kind, region string) error {
	klog.V(3).Infof("[mysql.RemoveJobRecord] jobID: %s, region: %s", jobID, region)

	job, err := b.ReadJob(ns, name, jobID, kind, region)
	if err != nil {
		return err
	}

	deleted := 1
	newJob := &dmo.Job{
		Namespace:      job.Namespace,
		UID:            job.UID,
		EtcdVersion:    job.EtcdVersion,
		Status:         job.Status,
		RegionID:       job.RegionID,
		IsDeleted:      &deleted,
		IsInK8s:        0,
		GmtJobFinished: job.GmtJobFinished,
	}
	return b.updateJob(job, newJob)
}

func (b *mysqlBackend) ListCrons(query *backends.CronQuery) ([]*dmo.Cron, error) {
	klog.V(3).Infof("[mysql.ListCrons] list crons, query: %v", query)

	crons := make([]*dmo.Cron, 0, initListSize)
	db := b.db.Model(&dmo.Cron{})
	db = db.Where("gmt_created < ?", query.EndTime).
		Where("gmt_created > ?", query.StartTime)
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Namespace != "" {
		db = db.Where("namespace LIKE ?", "%"+query.Namespace+"%")
	} else {
		if len(query.AllocatedNamespaces) == 1 {
			db = db.Where("namespace = ?", query.AllocatedNamespaces[0])
		} else {
			db = db.Where("namespace IN (?)", query.AllocatedNamespaces)
		}
	}
	if query.Type != "" {
		db = db.Where("kind = ?", query.Type)
	}
	if query.RegionID != "" {
		db = db.Where("region_id = ?", query.RegionID)
	}
	if query.UID != "" {
		//db = db.Where("user_id IN (?, '')", query.UID)
		db = db.Where("user_id = ? or user_id is NULL", query.UID)
	}
	db = db.Where("is_deleted = ?", *query.Deleted)
	db = db.Order("gmt_created DESC")
	if query.Pagination != nil {
		db = db.Count(&query.Pagination.Count).
			Limit(query.Pagination.PageSize).
			Offset((query.Pagination.PageNum - 1) * query.Pagination.PageSize)
	}
	db = db.Find(&crons)
	if db.Error != nil {
		return nil, db.Error
	}
	return crons, nil
}

func (b *mysqlBackend) GetCron(ns, name, uid string) (*dmo.Cron, error) {
	klog.Infof("[mysql.GetCron] cron namespace:%s name:%s uid:%s", ns, name, uid)
	cron := dmo.Cron{}
	query := &dmo.Cron{Namespace: ns, Name: name}
	if uid != "" {
		query.UID = uid
	}
	result := b.db.Where(query).First(&cron)
	if result.Error != nil {
		return nil, result.Error
	}
	return &cron, nil
}

func (b *mysqlBackend) DeleteCron(ns, name, uid string) error {
	klog.Infof("[mysql.DeleteCron] cron namespace: %s, name: %s, uid: %s", ns, name, uid)

	dmoCron, err := b.GetCron(ns, name, uid)
	if err != nil {
		return err
	}

	deleted := 1
	dmoCron.IsInK8s = 0
	dmoCron.IsDeleted = &deleted

	r := gjson.Parse(dmoCron.History)
	for _, history := range r.Array() {
		jobName := history.Get("object.name").String()
		b.removeJobRecordOfCron(dmoCron.Namespace, jobName, dmoCron.Kind)
	}

	return b.updateCron(&dmo.Cron{Namespace: ns, Name: name, UID: uid}, dmoCron)
}

func (b *mysqlBackend) removeJobRecordOfCron(ns, name, kind string) error {
	klog.Infof("[mysql.removeJobRecordOfCron] ns: %s, name: %s kind: %s", ns, name, kind)

	job, err := b.readJobOfCron(ns, name, kind)
	if err != nil {
		return err
	}

	deleted := 1
	newJob := &dmo.Job{
		Namespace:      job.Namespace,
		UID:            job.UID,
		EtcdVersion:    job.EtcdVersion,
		Status:         job.Status,
		RegionID:       job.RegionID,
		IsDeleted:      &deleted,
		IsInK8s:        0,
		GmtJobFinished: job.GmtJobFinished,
	}
	return b.updateJob(job, newJob)
}

func (b *mysqlBackend) readJobOfCron(ns, name, kind string) (*dmo.Job, error) {
	klog.Infof("[mysql.readJobOfCron] ns: %s name: %s kind: %s", ns, name, kind)

	job := dmo.Job{}
	query := &dmo.Job{Namespace: ns, Name: name, Kind: kind}
	result := b.db.Where(query).First(&job)
	if result.Error != nil {
		if gorm.IsRecordNotFoundError(result.Error) {
			// Hack(qiukai.cqk): try select by name only for PAI-DLC, name uniqueness guaranteed
			// by PAI-DLC service.
			query = &dmo.Job{Name: name}
			result = b.db.Where(query).First(&job)
		}
		if result.Error != nil {
			return nil, result.Error
		}
	}
	return &job, nil
}

func (b *mysqlBackend) WriteCron(cron *appsv1alpha1.Cron) error {
	klog.V(3).Infof("[mysql.WriteCron] cron namespace: %s, name: %s, uid: %s", cron.Namespace, cron.Name, cron.UID)
	dmoCron := converters.ConvertCronToDMOCron(cron)

	oldCron, err := b.GetCron(cron.Namespace, cron.Name, string(cron.UID))
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return b.db.Create(dmoCron).Error
		}
		return err
	}
	dmoCron.IsInK8s = 1
	dmoCron.GmtModified = time.Now()
	return b.updateCron(oldCron, dmoCron)
}

func (b *mysqlBackend) ListCronHistories(ns, name, jobName, jobStatus, cronID string) ([]*dmo.Job, error) {
	klog.V(3).Infof("[mysql.ListCronHistories] cron namespace: %s, name: %s, job status: %s", ns, name, jobStatus)

	cron, err := b.GetCron(ns, name, string(cronID))
	if err != nil {
		return nil, err
	}

	jobList := make([]*dmo.Job, 0, initListSize)
	r := gjson.Parse(cron.History)

	for _, h := range r.Array() {
		hname := h.Get("object").Get("name").String()
		hstatus := h.Get("status").String()
		hkind := h.Get("object").Get("kind").String()

		if jobName == "" || jobName == hname {
			if jobStatus == "" || jobStatus == hstatus {
				job, err := b.ReadJob(ns, hname, "", hkind, "")
				if err != nil {
					return nil, err
				}
				jobList = append(jobList, job)
			}
		}
	}

	return jobList, nil
}

func (b *mysqlBackend) updateCron(oldCron, newCron *dmo.Cron) error {
	return b.db.
		Model(oldCron).
		Where(&dmo.Cron{
			Name:      oldCron.Name,
			Namespace: oldCron.Namespace,
			UID:       oldCron.UID,
		}).Updates(cronUpdatesMap(newCron)).Error
}

// cronUpdatesMap mirrors jobUpdatesMap for dmo.Cron: gorm v1 struct-based
// Updates skip zero-value fields, which would drop is_in_k8s = 0 when a cron
// is deleted (DeleteCron). Nil/zero fields are omitted to keep the previous
// semantics for every other column.
func cronUpdatesMap(cron *dmo.Cron) map[string]interface{} {
	updates := map[string]interface{}{
		"is_in_k8s":    cron.IsInK8s,
		"gmt_modified": time.Now(),
	}
	if cron.Name != "" {
		updates["name"] = cron.Name
	}
	if cron.Namespace != "" {
		updates["namespace"] = cron.Namespace
	}
	if cron.UID != "" {
		updates["uid"] = cron.UID
	}
	if cron.Kind != "" {
		updates["kind"] = cron.Kind
	}
	if cron.Status != "" {
		updates["status"] = cron.Status
	}
	if cron.RegionID != nil {
		updates["region_id"] = *cron.RegionID
	}
	if cron.ClusterID != nil {
		updates["cluster_id"] = *cron.ClusterID
	}
	if cron.Schedule != "" {
		updates["schedule"] = cron.Schedule
	}
	if cron.ConcurrencyPolicy != "" {
		updates["concurrency_policy"] = cron.ConcurrencyPolicy
	}
	if cron.Active != "" {
		updates["active"] = cron.Active
	}
	if cron.History != "" {
		updates["history"] = cron.History
	}
	if cron.HistoryLimit != nil {
		updates["history_limit"] = *cron.HistoryLimit
	}
	if cron.IsDeleted != nil {
		updates["is_deleted"] = *cron.IsDeleted
	}
	if cron.Suspend != nil {
		updates["suspend"] = *cron.Suspend
	}
	if cron.Deadline != nil {
		updates["deadline"] = *cron.Deadline
	}
	if cron.User != nil {
		updates["user_id"] = *cron.User
	}
	if cron.LastScheduleTime != nil {
		updates["last_schedule_time"] = *cron.LastScheduleTime
	}
	if !cron.GmtCreated.IsZero() {
		updates["gmt_created"] = cron.GmtCreated
	}
	if !cron.GmtModified.IsZero() {
		updates["gmt_modified"] = cron.GmtModified
	}
	return updates
}

func (b *mysqlBackend) createNewPod(pod *corev1.Pod) error {
	dmoPod, err := converters.ConvertPodToDMOPod(pod)
	if err != nil {
		klog.Errorf("fail to create pod: %s/%s, err:%s", pod.Namespace, pod.Name, err.Error())
		return err
	}
	return b.db.Create(dmoPod).Error
}

func (b *mysqlBackend) updatePod(oldPod, newPod *dmo.Pod) error {
	var (
		oldVersion, newVersion int64
		err                    error
	)
	// Compare versions between two pods.
	if oldVersion, err = strconv.ParseInt(oldPod.EtcdVersion, 10, 64); err != nil {
		klog.Errorf("fail to strconv.ParseInt: %s/%s, err:%s", oldPod.Namespace, oldPod.Name, err.Error())
		return err
	}
	if newVersion, err = strconv.ParseInt(newPod.EtcdVersion, 10, 64); err != nil {
		klog.Errorf("fail to strconv.ParseInt: %s/%s, err:%s", oldPod.Namespace, oldPod.Name, err.Error())
		return err
	}
	if oldVersion > newVersion {
		klog.Warningf("try to update a pod newer than the existing one, old version: %d, new version: %d",
			oldVersion, newVersion)
		return nil
	}
	// Setup timestamps if new pod has not set.
	if oldPod.GmtPodRunning != nil && !oldPod.GmtPodRunning.IsZero() && newPod.GmtPodRunning == nil {
		newPod.GmtPodRunning = oldPod.GmtPodRunning
	}
	if oldPod.GmtPodFinished != nil && !oldPod.GmtPodFinished.IsZero() && newPod.GmtPodFinished == nil {
		newPod.GmtPodFinished = oldPod.GmtPodFinished
	}
	// Only update pod when the old one differs with the new one.
	podEquals := oldPod.EtcdVersion == newPod.EtcdVersion && oldPod.Status == newPod.Status &&
		(oldPod.GmtPodRunning != nil && newPod.GmtPodRunning != nil && oldPod.GmtPodRunning.Equal(*newPod.GmtPodRunning)) &&
		(oldPod.GmtPodFinished != nil && newPod.GmtPodFinished != nil && oldPod.GmtPodFinished.Equal(*newPod.GmtPodFinished))

	if podEquals {
		return nil
	}

	// Do updating.
	result := b.db.Model(&dmo.Pod{}).Where(&dmo.Pod{
		Name:        oldPod.Name,
		Namespace:   oldPod.Namespace,
		UID:         oldPod.UID,
		EtcdVersion: oldPod.EtcdVersion,
	}).Updates(&dmo.Pod{
		EtcdVersion:    newPod.EtcdVersion,
		Status:         newPod.Status,
		Image:          newPod.Image,
		HostIP:         newPod.HostIP,
		PodIP:          newPod.PodIP,
		PodJson:        newPod.PodJson,
		Extended:       newPod.Extended,
		GmtPodRunning:  newPod.GmtPodRunning,
		GmtPodFinished: newPod.GmtPodFinished,
	})
	if result.Error != nil {
		klog.Errorf("fail to update pod: %s/%s, err:%s", oldPod.Namespace, oldPod.Name, result.Error.Error())
		return result.Error
	}
	if result.RowsAffected < 1 {
		klog.Warningf("update pod with no row affected, old version: %s", oldPod.EtcdVersion)
	}
	klog.V(3).Infof("[mysql.updatePod]success to update pod: %s/%s", oldPod.Namespace, oldPod.Name)
	return nil
}

func (b *mysqlBackend) createNewJob(job metav1.Object, kind string, specs map[apiv1.ReplicaType]*apiv1.ReplicaSpec, jobStatus *apiv1.JobStatus, region string, gpuTopoAware bool) error {
	newJob, err := converters.ConvertJobToDMOJob(job, kind, specs, jobStatus, region, gpuTopoAware)
	if err != nil {
		return err
	}
	if newJob.GmtCreated.IsZero() {
		newJob.GmtCreated = newJob.GmtJobSubmitted
	}
	return b.db.Create(newJob).Error
}

func (b *mysqlBackend) updateJob(oldJob, newJob *dmo.Job) error {
	var (
		oldVersion, newVersion int64
		err                    error
	)
	// Compare versions between two pods.
	if oldVersion, err = strconv.ParseInt(oldJob.EtcdVersion, 10, 64); err != nil {
		return err
	}
	if newVersion, err = strconv.ParseInt(newJob.EtcdVersion, 10, 64); err != nil {
		return err
	}
	if oldVersion > newVersion {
		klog.Warningf("try to update a job newer than the existing one, old version: %d, new version: %d",
			oldVersion, newVersion)
		return nil
	}

	// Only update job when the old one differs with the new one.
	jobEquals := oldVersion == newVersion && oldJob.Status == newJob.Status &&
		(oldJob.IsDeleted != nil && newJob.IsDeleted != nil && *oldJob.IsDeleted == *newJob.IsDeleted)
	if jobEquals {
		return nil
	}

	if oldJob.GmtJobRunning != nil && !oldJob.GmtJobRunning.IsZero() && newJob.GmtJobRunning == nil {
		newJob.GmtJobRunning = oldJob.GmtJobRunning
	}

	// Build an explicit column->value map: gorm v1 struct-based Updates skip
	// zero-value fields, which would silently drop legitimate writes such as
	// is_in_k8s = 0 when a job leaves the cluster.
	result := b.db.Model(&dmo.Job{}).Where(&dmo.Job{
		Name:      oldJob.Name,
		Namespace: oldJob.Namespace,
		UID:       oldJob.UID,
	}).
		// Optimistic locking: only update the row if it still carries the
		// etcd_version observed at read time. A concurrent writer that already
		// advanced the version makes this UPDATE affect zero rows.
		Where("etcd_version = ?", oldJob.EtcdVersion).
		Updates(jobUpdatesMap(newJob))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		klog.Warningf("[mysql.updateJob] etcd_version conflict, job %s/%s (uid: %s) expected version: %s",
			oldJob.Namespace, oldJob.Name, oldJob.UID, oldJob.EtcdVersion)
		return errEtcdVersionConflict
	}

	if oldJob.Status != apiv1.JobSucceeded && oldJob.Status != apiv1.JobFailed && oldJob.Status != utils.JobStopped {
		if newJob.GmtJobFinished != nil {
			klog.Infof("[updateJob digest] jobID: %s, duration: %dm, old status: %s, new status: %s",
				newJob.UID, newJob.GmtJobFinished.Sub(oldJob.GmtJobSubmitted)/time.Minute, oldJob.Status, newJob.Status)
		} else {
			klog.Infof("[updateJob digest] jobID: %s, old status: %s, new status: %s",
				newJob.UID, oldJob.Status, newJob.Status)
		}
	}
	return nil
}

// jobUpdatesMap builds the explicit column->value update map for a dmo.Job row.
// It mirrors the fields previously passed to the struct-based Updates call,
// with two differences:
//   - is_in_k8s is always written, so resetting it to 0 (job removed from the
//     cluster) is actually persisted instead of being skipped as a zero value;
//   - nil pointer / zero-value fields are omitted to preserve the previous
//     "never overwrite with zero" semantics for every other column.
func jobUpdatesMap(job *dmo.Job) map[string]interface{} {
	updates := map[string]interface{}{
		"is_in_k8s": job.IsInK8s,
		// gorm v1 map-based Updates do not touch gmt_modified automatically,
		// keep it fresh explicitly.
		"gmt_modified": time.Now(),
	}
	if job.Name != "" {
		updates["name"] = job.Name
	}
	if job.Namespace != "" {
		updates["namespace"] = job.Namespace
	}
	if job.UID != "" {
		updates["uid"] = job.UID
	}
	if job.Status != "" {
		updates["status"] = string(job.Status)
	}
	if job.RegionID != nil {
		updates["region_id"] = *job.RegionID
	}
	if job.EtcdVersion != "" {
		updates["etcd_version"] = job.EtcdVersion
	}
	if job.JobJson != "" {
		updates["job_json"] = job.JobJson
	}
	if job.Extended != nil {
		updates["extended"] = *job.Extended
	}
	if job.IsDeleted != nil {
		updates["is_deleted"] = *job.IsDeleted
	}
	if !job.GmtJobSubmitted.IsZero() {
		updates["gmt_job_submitted"] = job.GmtJobSubmitted
	}
	if job.GmtJobRunning != nil {
		updates["gmt_job_running"] = *job.GmtJobRunning
	}
	if job.GmtJobStopped != nil {
		updates["gmt_job_stopped"] = *job.GmtJobStopped
	}
	if job.GmtJobFinished != nil {
		updates["gmt_job_finished"] = *job.GmtJobFinished
	}
	if job.ReasonCode != nil {
		updates["reason_code"] = *job.ReasonCode
	}
	if job.Reason != nil {
		updates["reason"] = *job.Reason
	}
	return updates
}

func (b *mysqlBackend) init() error {
	klog.Infof("init mysql")
	dbSource, logMode, err := GetMysqlDBSource()
	if err != nil {
		return err
	}
	if b.db, err = gorm.Open("mysql", dbSource); err != nil {
		return err
	}
	b.db.LogMode(logMode == "debug")

	// Try create tables if they have not been created in database, or the
	// storage service will not work.
	if !b.db.HasTable(&dmo.Pod{}) {
		klog.Infof("database has not table %s, try to create it", dmo.Pod{}.TableName())
		err = b.db.CreateTable(&dmo.Pod{}).Error
		if err != nil {
			return err
		}
	}
	if !b.db.HasTable(&dmo.Job{}) {
		klog.Infof("database has not table %s, try to create it", dmo.Job{}.TableName())
		err = b.db.CreateTable(&dmo.Job{}).Error
		if err != nil {
			return err
		}
	}
	if !b.db.HasTable(&dmo.Cron{}) {
		klog.Infof("database has not table %s, try to create it", dmo.Cron{}.TableName())
		err = b.db.CreateTable(&dmo.Cron{}).Error
		if err != nil {
			return err
		}
	}
	if !b.db.HasTable(&dmo.Model{}) {
		klog.Infof("database has not table %s, try to create it", dmo.Model{}.TableName())
		err = b.db.CreateTable(&dmo.Model{}).Error
		if err != nil {
			return err
		}
	}
	if !b.db.HasTable(&dmo.EvaluateJob{}) {
		klog.Infof("database has not table %s, try to create it", dmo.EvaluateJob{}.TableName())
		err = b.db.CreateTable(&dmo.EvaluateJob{}).Error
		if err != nil {
			return err
		}
	}
	if !b.db.HasTable(&dmo.Notebook{}) {
		klog.Infof("database has not table %s, try to create it", dmo.Notebook{}.TableName())
		err = b.db.CreateTable(&dmo.Notebook{}).Error
		if err != nil {
			return err
		}
	}

	// Add the user_id column to pre-existing tables. The column check makes
	// the migration idempotent so restarts no longer rely on ignoring
	// "duplicate column" errors.
	migrations := []struct {
		model interface{}
		table string
	}{
		{&dmo.Cron{}, "cron"},
		{&dmo.Model{}, "model"},
		{&dmo.EvaluateJob{}, "evaluate"},
		{&dmo.Notebook{}, "notebook"},
	}
	for _, m := range migrations {
		if b.db.HasTable(m.model) && !b.db.NewScope(m.model).HasColumn("user_id") {
			if err := b.db.Exec("ALTER TABLE " + m.table + " ADD user_id VARCHAR(128)").Error; err != nil {
				klog.Errorf("add user_id column to %s failed: %v", m.table, err)
				return err
			}
		}
	}

	return nil
}

func (b *mysqlBackend) updateEvaluateJob(oldJob *dmo.EvaluateJob, job *dmo.EvaluateJob) error {
	return b.db.
		Model(oldJob).
		Where(&dmo.EvaluateJob{
			Name:      oldJob.Name,
			Namespace: oldJob.Namespace,
			UID:       oldJob.UID,
		}).Updates(job).Error
}
