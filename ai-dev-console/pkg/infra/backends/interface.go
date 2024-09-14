/*batch "k8s.io/api/batch/v1"
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

package backends

import (
	appsv1alpha1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/apps/v1alpha1"
	notebookv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"

	batch "k8s.io/api/batch/v1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

// ObjectStorageBackend provides a collection of abstract methods to
// interact with different storage backends, write/read pod and job objects.
type ObjectStorageBackend interface {
	// Initialize initializes a backend storage service with local or remote
	// database.
	Initialize() error
	// Close shutdown backend storage service.
	Close() error
	// Name returns backend name.
	Name() string

	UserName(userName string) ObjectStorageBackend

	PodStorageBackend

	JobStorageBackend

	CronStorageBackend
	EvaluateJobStorageBackend
	ModelsStorageBackend
	NotebookStorageBackend
}

type JobStorageBackend interface {
	// WriteJob append or update a job record to backend, region is optional.
	WriteJob(job metav1.Object, kind string, specs map[apiv1.ReplicaType]*apiv1.ReplicaSpec, runPolicy *apiv1.RunPolicy, jobStatus *apiv1.JobStatus, region string) error
	// ReadJob retrieve a job from backend, region is optional.
	ReadJob(ns, name, jobID, kind, region string) (*dmo.Job, error)
	// ListJobs lists those jobs who satisfied with query conditions.
	ListJobs(query *Query) ([]*dmo.Job, error)
	// UpdateJobRecordStopped updates status of job record as stooped.
	UpdateJobRecordStopped(ns, name, jobID, kind, region string) error
	// RemoveJobRecord updates job as deleted from api-server, but not delete job record
	// from backend, region is optional.
	RemoveJobRecord(ns, name, jobID, kind, region string) error
}

type PodStorageBackend interface {
	// WritePod append or update a pod record to backend, region is optional.
	WritePod(pod *v1.Pod) error
	// ListPods lists pods controlled by some job, region is optional.
	ListPods(ns, kind, name, jobID string) ([]*dmo.Pod, error)
	// UpdatePodRecordStopped updates status of pod record as stopped.
	UpdatePodRecordStopped(ns, name, podID string) error
}

type ConfigStorageBackend interface {
	WriteCodeSource(ns, name, codeSource string) error
	GetCodeSource(ns, name string) (string, error)
	ListCodeSource(ns string) ([]string, error)
	DeleteCodeSource(ns, name string) error

	WriteDataSource(ns, name, codeSource string) error
	GetDataSource(ns, name string) (string, error)
	ListDataSource(ns string) ([]string, error)
	DeleteDataSource(ns, name string) error
}

type CronStorageBackend interface {
	ListCrons(query *CronQuery) ([]*dmo.Cron, error)
	GetCron(ns, name, cronID string) (*dmo.Cron, error)
	DeleteCron(ns, name, cronID string) error
	WriteCron(cron *appsv1alpha1.Cron) error
	ListCronHistories(ns, name, jobName, jobStatus, cronID string) ([]*dmo.Job, error)
}

type EvaluateJobStorageBackend interface {
	ListEvaluateJobs(query *EvaluateJobQuery) ([]*dmo.EvaluateJob, error)
	GetEvaluateJob(ns, name, evaluateJobID string) (*dmo.EvaluateJob, error)
	DeleteEvaluateJob(ns, name, evaluateJobID string) error
	WriteEvaluateJob(evaluateJob *batch.Job, PV_OSMap map[string]string) error
}

type ModelsStorageBackend interface {
	ListModels(query *ModelsQuery) ([]*dmo.Model, error)
	GetModel(modelID string) (*dmo.Model, error)
	DeleteModel(modelID string) error
	WriteModel(model *dmo.Model) error
}

type NotebookStorageBackend interface {
	ListNotebook(query *NotebookQuery) ([]*dmo.Notebook, error)
	ListAllNotebook(query *NotebookQuery) ([]*dmo.Notebook, error)
	DeleteNotebook(namespace, name string) error
	WriteNotebook(notebook *notebookv1.Notebook) error
	GetNotebook(namespace, name string) (*dmo.Notebook, error)
	UpdateNotebookToken(namespace, name, token string) error
}

type ObjectClientBackend interface {
	// Initialize initializes a backend service with local or remote
	// event hub.
	Initialize() error
	// Close shutdown backend service or disconnect the event hub.
	Close() error
	// Name returns backend name.
	Name() string

	UserName(userName string) ObjectClientBackend

	SubmitJob(*dmo.SubmitJobInfo) error
	SubmitEvaluateJob(*dmo.SubmitEvaluateJobInfo) error
	StopJob(ns, name, jobID, kind string) error
	DeleteEvaluateJob(ns, name string) error

	SuspendCron(ns, name, cronID string) error
	ResumeCron(ns, name, cronID string) error
	StopCron(ns, name, cronID string) error
}

// EventStorageBackend provides a collection of abstract methods to
// interact with different storage backends, write/read events.
type EventStorageBackend interface {
	// Initialize initializes a backend storage service with local or remote
	// event hub.
	Initialize() error
	// Close shutdown backend event storage service or disconnect the event hub.
	Close() error
	// Name returns backend name.
	Name() string

	UserName(userName string) EventStorageBackend
	// SaveEvent append or update a event record to backend.
	SaveEvent(event *v1.Event, region string) error
	// ListEvents list all events created by the object with namespaced name.
	ListEvents(namespace, name string, from, to time.Time) ([]*dmo.Event, error)
	// ListLogs list log entries generated by the pod with namespaced name.
	ListLogs(namespace, jobKind, jobName, name string, maxLine int64, from, to time.Time) ([]string, error)
}
