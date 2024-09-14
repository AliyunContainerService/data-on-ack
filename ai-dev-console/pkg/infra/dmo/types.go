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

package dmo

import (
	"time"

	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"

	"github.com/jinzhu/gorm"
	v1 "k8s.io/api/core/v1"
)

// Pod contains fields collected from original Pod object and extra info that
// we concerned about, they will be persisted by storage backend.
type Pod struct {
	// Primary ID auto incremented by underlying database.
	ID uint64 `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;primaryKey" json:"id"`
	// Metadata we concerned aggregated from pod object.
	Name      string `gorm:"type:varchar(256);column:name" json:"name"`
	Namespace string `gorm:"type:varchar(256);column:namespace" json:"namespace"`
	// Kubernetes UID
	UID         string      `gorm:"type:varchar(256);column:uid" json:"uid"`
	EtcdVersion string      `gorm:"type:varchar(64);column:etcd_version" json:"etcd_version"`
	Status      v1.PodPhase `gorm:"type:varchar(32);column:status" json:"status"`
	Image       string      `gorm:"type:varchar(256);column:image" json:"image"`
	GPU         int         `gorm:"type:tinyint(4);column:gpu" json:"gpu"`
	// Job UID of this pod controlled by.
	JobUID string `gorm:"type:varchar(256);column:job_uid" json:"job_uid"`
	// Job name of this pod controlled by.
	JobName string `gorm:"type:varchar(256);column:job_name" json:"job_name"`
	// Replica type of this pod figured in training job.
	ReplicaType string `gorm:"type:varchar(32);column:replica_type" json:"replica_type"`
	PodJson     string `gorm:"type:text;column:pod_json" json:"pod_json"`
	// IP information allocated for this pod.
	HostIP *string `gorm:"type:varchar(64);column:host_ip" json:"host_ip,omitempty"`
	PodIP  *string `gorm:"type:varchar(64);column:pod_ip" json:"pod_ip,omitempty"`
	// Optional remark text reserved.
	Extended *string `gorm:"type:varchar(4096);column:extended" json:"extended,omitempty"`
	// Timestamps of different pod phases and status transitions.
	GmtCreated     time.Time  `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
	GmtModified    time.Time  `gorm:"type:datetime;column:gmt_modified" json:"gmt_modified"`
	GmtPodRunning  *time.Time `gorm:"type:datetime;column:gmt_pod_running" json:"gmt_pod_running,omitempty"`
	GmtPodFinished *time.Time `gorm:"type:datetime;column:gmt_pod_finished" json:"gmt_pod_finished,omitempty"`
}

// Job contains fields collected from original Job object and extra info that
// we concerned about, they will be persisted by storage backend.
type Job struct {
	// Primary ID auto incremented by underlying database.
	ID uint64 `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;primaryKey" json:"id"`
	// Metadata we concerned aggregated from job object.
	Name        string `gorm:"type:varchar(256);column:name" json:"name"`
	Namespace   string `gorm:"type:varchar(256);column:namespace" json:"namespace"`
	DisplayName string `gorm:"type:varchar(256);column:display_name" json:"display_name"`
	// Kubernetes UID
	UID    string                 `gorm:"type:varchar(256);column:uid" json:"uid"`
	Status apiv1.JobConditionType `gorm:"type:varchar(32);column:status" json:"status"`

	// Kind of this job: TFJob, PytorchJob...
	Kind    string `gorm:"type:varchar(32);column:kind" json:"kind"`
	JobJson string `gorm:"type:text;column:job_json" json:"job_json"`
	// RegionID indicates the physical region(IDC) this job located in, reserved for
	// jobs running in across-region-clusters.
	RegionID  *string `gorm:"type:varchar(256);column:region_id" json:"region_id,omitempty"`
	ClusterID *string `gorm:"type:varchar(256);column:cluster_id" json:"cluster_id,omitempty"`
	// Fields reserved for multi-tenancy job management scenarios, indicating
	// which tenant this job belongs to and who's the owner(user).
	Tenant *string `gorm:"type:varchar(128);column:tenant_id" json:"tenant_id,omitempty"`
	Group  *string `gorm:"type:varchar(128);column:group_id" json:"group_id,omitempty"`
	// if created by RAM account, user is aliyun accountid, else user is username
	User *string `gorm:"type:varchar(128);column:user_id" json:"user_id,omitempty"`

	CreatedBy *string `gorm:"type:varchar(64);column:created_by" json:"created_by,omitempty"`

	ReasonCode *string `gorm:"type:varchar(128);column:reason_code" json:"reason_code"`
	Reason     *string `gorm:"type:varchar(1024);column:reason" json:"reason"`

	EtcdVersion string `gorm:"type:varchar(64);column:etcd_version" json:"etcd_version"`
	IsInK8s     int    `gorm:"type:tinyint(4);column:is_in_k8s" json:"is_in_k8s"`
	// IsDeleted indicates that whether this job has been deleted or not.
	IsDeleted              *int  `gorm:"type:tinyint(4);column:is_deleted;default:0" json:"is_deleted,omitempty"`
	EnableGPUTopologyAware *int8 `gorm:"type:tinyint(4);column:is_enable_gpu_topo_aware" json:"is_enable_gpu_topo_aware"`
	// Optional remark text reserved.
	Extended *string `gorm:"type:text;column:extended" json:"extended,omitempty"`
	// Timestamps of different job phases and status transitions.
	GmtCreated      time.Time  `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
	GmtModified     time.Time  `gorm:"type:datetime;column:gmt_modified" json:"gmt_modified"`
	GmtJobSubmitted time.Time  `gorm:"type:datetime;column:gmt_job_submitted" json:"gmt_job_submitted"`
	GmtJobStopped   *time.Time `gorm:"type:datetime;column:gmt_job_stopped" json:"gmt_job_stopped,omitempty"`
	GmtJobRunning   *time.Time `gorm:"type:datetime;column:gmt_job_running" json:"gmt_job_running,omitempty"`
	GmtJobFinished  *time.Time `gorm:"type:datetime;column:gmt_job_finished" json:"gmt_job_finished,omitempty"`

	// Resources this job requested, including replicas and resources of each type,
	// it's formatted as follows:
	// {
	//   "PS": {
	//     "replicas": 1,
	//     "resources": {"cpu":2, "memory": "10Gi"}
	//   },
	//   "Worker": {
	//     "replicas": 2,
	//     "resources": {"cpu":2, "memory": "10Gi"}
	//   }
	// }
	Resources string `gorm:"type:text;column:resources" json:"resources"`

	// JobConfig indicates this job's basic config
	// it's formatted as follows:
	// {
	//   "code_bindings": {
	//     "source": "https://code.aliyun.com/xiaozhou/tensorflow-sample-code.git",
	//     "branch": "master"
	//   },
	//   "data_bindings": [
	//     "pai-deeplearning-oss",
	//     "pai-deeplearning-nas"
	//   ],
	//   "commands": [
	//	   "/bin/sh",
	//     "-c",
	//     "python tensorflow-sample-code/tfjob/docker/mnist/main.py --max_steps=10000 --data_dir=tensorflow-sample-code/data/"
	//   ]
	// }
	JobConfig string `gorm:"type:text;column:job_config" json:"job_config,omitempty"`
}

// Event contains fields collected from original Event object, they will be persisted
// by storage backend.
type Event struct {
	// Name of this event.
	Name string `gorm:"type:varchar(128);column:name" json:"name"`
	// Kind of object involved by event.
	Kind string `gorm:"type:varchar(32);column:kind" json:"kind"`
	// Type of this event.
	Type string `gorm:"type:varchar(32);column:type" json:"type"`
	// Involved Object Namespace.
	ObjNamespace string `gorm:"type:varchar(64);column:obj_namespace" json:"obj_namespace"`
	// Involved Object Name.
	ObjName string `gorm:"type:varchar(64);column:obj_name" json:"obj_name"`
	// Involved Object UID.
	ObjUID string `gorm:"type:varchar(64);column:obj_uid" json:"obj_uid"`
	// Reason(short, machine understandable string) of this event.
	Reason string `gorm:"type:varchar(128);column:reason" json:"reason"`
	// Message(long, human understandable description) of this event.
	Message string `gorm:"type:text;column:message" json:"message"`
	// Number of times this event has occurred.
	Count int32 `gorm:"type:integer(32);column:reason" json:"count"`
	// Region indicates the physical region(IDC) this job located in.
	Region *string `gorm:"type:varchar(64);column:region" json:"region,omitempty"`
	// The time at which the event was first recorded.
	FirstTimestamp time.Time `gorm:"type:datetime;column:first_timestamp" json:"first_timestamp"`
	// The time at which the most recent occurrence of this event was recorded.
	LastTimestamp time.Time `gorm:"type:datetime;column:last_timestamp" json:"last_timestamp"`
}

type Cron struct {
	// Primary ID auto incremented by underlying database.
	ID uint64 `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;primaryKey" json:"id"`
	// Metadata we concerned aggregated from job object.
	Name      string `gorm:"type:varchar(256);column:name" json:"name"`
	Namespace string `gorm:"type:varchar(256);column:namespace" json:"namespace"`
	// Kubernetes UID
	UID string `gorm:"type:varchar(256);column:uid" json:"uid"`
	// Kind of this job: TFJob, PytorchJob...
	Kind string `gorm:"type:varchar(32);column:kind" json:"kind"`
	// RegionID indicates the physical region(IDC) this job located in, reserved for
	// jobs running in across-region-clusters.
	Status            string     `gorm:"type:varchar(32);column:status" json:"status"`
	RegionID          *string    `gorm:"type:varchar(256);column:region_id" json:"region_id,omitempty"`
	ClusterID         *string    `gorm:"type:varchar(256);column:cluster_id" json:"cluster_id,omitempty"`
	Schedule          string     `gorm:"type:varchar(32);column:schedule" json:"schedule"`
	ConcurrencyPolicy string     `gorm:"type:varchar(32);column:concurrency_policy" json:"concurrency_policy"`
	Active            string     `gorm:"type:text;column:active" json:"active"`
	History           string     `gorm:"type:text;column:history" json:"history"`
	HistoryLimit      *int32     `gorm:"type:integer(32);column:history_limit" json:"history_limit,omitempty"`
	IsInK8s           int        `gorm:"type:tinyint(4);column:is_in_k8s" json:"is_in_k8s"`
	IsDeleted         *int       `gorm:"type:tinyint(4);column:is_deleted;default:0" json:"is_deleted,omitempty"`
	Suspend           *int8      `gorm:"type:tinyint(4);column:suspend" json:"suspend,omitempty"`
	Deadline          *time.Time `gorm:"type:datetime;column:deadline" json:"deadline,omitempty"`
	// if created by RAM account, user is aliyun accountid, else user is username
	User             *string    `gorm:"type:varchar(128);column:user_id" json:"user_id,omitempty"`
	LastScheduleTime *time.Time `gorm:"type:datetime;column:last_schedule_time" json:"last_schedule_time,omitempty"`
	GmtCreated       time.Time  `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
	GmtModified      time.Time  `gorm:"type:datetime;column:gmt_modified" json:"gmt_modified"`
}

type EvaluateJob struct {
	// Primary ID auto incremented by underlying database.
	ID    uint64 `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;primaryKey" json:"id"`
	JobID string `gorm:"type:varchar(50) NOT NULL;column:job_id" json:"job_id"`
	// Metadata we concerned aggregated from job object.
	Name      string `gorm:"type:varchar(256);column:name" json:"name"`
	Namespace string `gorm:"type:varchar(256);column:namespace" json:"namespace"`
	// Kubernetes UID
	UID string `gorm:"type:varchar(256);column:uid" json:"uid"`
	// if created by RAM account, user is aliyun accountid, else user is username
	User *string `gorm:"type:varchar(128);column:user_id" json:"user_id,omitempty"`
	// RegionID indicates the physical region(IDC) this job located in, reserved for
	// jobs running in across-region-clusters.
	//ModelID 		  uint64     `gorm:"type:bigint(20);column:model_id" json:"model_id"`
	ModelName    string                 `gorm:"type:varchar(256);column:model_name" json:"model_name"`
	ModelVersion string                 `gorm:"type:varchar(256);column:model_version" json:"model_version"`
	Status       apiv1.JobConditionType `gorm:"type:varchar(32);column:status" json:"status"`
	Image        string                 `gorm:"type:varchar(256);column:image" json:"image"`
	DatasetPath  string                 `gorm:"type:varchar(256);column:dataset_path" json:"dataset_path"`
	Code         string                 `gorm:"type:text;column:code" json:"code"`
	Command      string                 `gorm:"type:varchar(256);column:command" json:"command"`
	Metrics      string                 `gorm:"type:text;column:metrics" json:"metrics"`
	IsDeleted    int                    `gorm:"type:tinyint(4);column:is_deleted;default:0" json:"is_deleted,omitempty"`
	ReportPath   string                 `gorm:"type:varchar(256);column:report_path" json:"report_path"`
	GmtCreated   time.Time              `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
	GmtModified  time.Time              `gorm:"type:datetime;column:gmt_modified" json:"gmt_modified"`
}

type Model struct {
	//ID         uint64    `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;key" json:"id"`
	ID      uint64 `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;primaryKey" json:"id"`
	Name    string `gorm:"type:varchar(256);column:model_name" json:"model_name"`
	Version string `gorm:"type:varchar(256);column:model_version" json:"model_version"`
	OSSPath string `gorm:"type:varchar(256);column:oss_path" json:"oss_path"`
	JobID   string `gorm:"type:varchar(256);column:job_id" json:"job_id"`
	// if created by RAM account, user is aliyun accountid, else user is username
	User       *string   `gorm:"type:varchar(128);column:user_id" json:"user_id,omitempty"`
	GmtCreated time.Time `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
}

func (model Model) TableName() string {
	return "model"
}

// BeforeCreate update gmt_modified timestamp.
func (model *Model) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

// BeforeUpdate update gmt_modified timestamp.
func (model *Model) BeforeUpdate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

type Notebook struct {
	ID        uint64 `gorm:"type:bigint(20) NOT NULL AUTO_INCREMENT;column:id;primaryKey" json:"id"`
	Name      string `gorm:"type:varchar(256);column:name" json:"name"`
	Namespace string `gorm:"type:varchar(256);column:namespace" json:"namespace"`
	Image     string `gorm:"type:varchar(256);column:image" json:"image"`
	Volumes   string `gorm:"type:text;column:volumes" json:"volumes"`
	Cpu       string `gorm:"type:varchar(256);column:cpu" json:"cpu"`
	Gpu       string `gorm:"type:varchar(256);column:gpu" json:"gpu"`
	Memory    string `gorm:"type:varchar(256);column:memory" json:"memory"`
	UserName  string `gorm:"type:varchar(256);column:user_name" json:"user_name"`
	// if created by RAM account, user is aliyun accountid, else user is username
	User             *string   `gorm:"type:varchar(128);column:user_id" json:"user_id,omitempty"`
	Token            string    `gorm:"type:varchar(256);column:token" json:"token"`
	Status           string    `gorm:"type:varchar(256);column:status" json:"status"`
	ImagePullSecrets string    `gorm:"type:text;column:image_pull_secrets" json:"image_pull_secrets"`
	GmtCreated       time.Time `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
}

func (notebook Notebook) TableName() string {
	return "notebook"
}

// BeforeCreate update gmt_modified timestamp.
func (notebook *Notebook) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

func (pod Pod) TableName() string {
	return "pod"
}

// BeforeCreate update gmt_modified timestamp.
func (pod *Pod) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

// BeforeUpdate update gmt_modified timestamp.
func (pod *Pod) BeforeUpdate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

func (job Job) TableName() string {
	return "job"
}

// BeforeUpdate update gmt_modified timestamp.
func (job *Job) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now())
}

// BeforeUpdate update gmt_modified timestamp.
func (job *Job) BeforeUpdate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now())
}

func (cron Cron) TableName() string {
	return "cron"
}

// BeforeCreate update gmt_modified timestamp.
func (cron *Cron) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

// BeforeUpdate update gmt_modified timestamp.
func (cron *Cron) BeforeUpdate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

func (evaluateJob EvaluateJob) TableName() string {
	return "evaluate"
}

// BeforeCreate update gmt_modified timestamp.
func (evaluateJob *EvaluateJob) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

// BeforeUpdate update gmt_modified timestamp.
func (evaluateJob *EvaluateJob) BeforeUpdate(scope *gorm.Scope) error {
	return scope.SetColumn("gmt_modified", time.Now().UTC())
}

func (e Event) TableName() string {
	return "event"
}

type SubmitJobInfo struct {
	Name          string                    `json:"name"`
	Namespace     string                    `json:"namespace"`
	Kind          string                    `json:"kind"`
	Annotations   map[string]string         `json:"annotations"`
	Labels        map[string]string         `json:"labels"`
	NodeSelectors map[string]string         `json:"nodeSelectors"`
	Toleration    map[string]TolerationData `json:"tolerates"`

	Shell            string   `json:"shell"`
	Command          []string `json:"command"`
	ImagePullSecrets []string `json:"imagePullSecrets"`
	WorkingDir       string   `json:"workingDir"`

	ChiefImage  string `json:"chiefImage"`
	ChiefCPU    string `json:"chiefCPU"`
	ChiefMemory string `json:"chiefMemory"`
	ChiefGPU    int    `json:"chiefGPU"`

	PsCount  int32  `json:"psCount"`
	PsImage  string `json:"psImage"`
	PsCPU    string `json:"psCPU"`
	PsMemory string `json:"psMemory"`
	PsGPU    int    `json:"psGPU"`

	WorkerCount  int32  `json:"workerCount"`
	WorkerImage  string `json:"workerImage"`
	WorkerCPU    string `json:"workerCPU"`
	WorkerMemory string `json:"workerMemory"`
	WorkerGPU    int    `json:"workerGPU"`

	EvaluatorImage  string `json:"evaluatorImage"`
	EvaluatorCPU    string `json:"evaluatorCPU"`
	EvaluatorMemory string `json:"evaluatorMemory"`
	EvaluatorGPU    int    `json:"evaluatorGPU"`

	Volumes map[string]string `json:"volumes"`

	CodeType     string `json:"codeType"`
	CodeSource   string `json:"codeSource"`
	CodeBranch   string `json:"codeBranch"`
	CodeDestPath string `json:"codeDestPath"`
	CodeUser     string `json:"codeUser"`
	CodePassword string `json:"codePassword"`

	EnableTensorboard bool   `json:"enableTensorboard"`
	LogDir            string `json:"logDir"`
	TensorboardHost   string `json:"tensorboardHost"`

	EnableCron              bool   `json:"enableCron"`
	Schedule                string `json:"schedule"`
	ConcurrencyPolicy       string `json:"concurrencyPolicy"`
	Deadline                string `json:"deadline"`
	HistoryLimit            int    `json:"historyLimit"`
	TTLSecondsAfterFinished int32  `json:"ttlSecondsAfterFinished"`
}

type SubmitEvaluateJobInfo struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Image            string            `json:"image"`
	ModelPath        string            `json:"modelPath"`
	ModelName        string            `json:"modelName"`
	ModelVersion     string            `json:"modelVersion"`
	DatasetPath      string            `json:"datasetPath"`
	MetricsPath      string            `json:"metricsPath"`
	Command          []string          `json:"command"`
	DataSources      map[string]string `json:"dataSources"`
	Envs             map[string]string `json:"envs"`
	ImagePullSecrets []string          `json:"imagePullSecrets"`
	Annotations      map[string]string `json:"annotations"`
	WorkingDir       string            `json:"workingDir"`

	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	GPU    int    `json:"gpu"`

	CodeType     string `json:"codeType"`
	CodeSource   string `json:"codeSource"`
	CodeBranch   string `json:"codeBranch"`
	CodeDestPath string `json:"codeDestPath"`
	CodeUser     string `json:"codeUser"`
	CodePassword string `json:"codePassword"`
}

type TolerationData struct {
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"`
}
