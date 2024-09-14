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

package converters

import (
	"encoding/json"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/notebook/v1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
	"k8s.io/klog"
	"time"
)

type TempVolume struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type NotebookStatus string

const (
	Running  NotebookStatus = "Running"
	Stopped  NotebookStatus = "Stopped"
	Deleted  NotebookStatus = "Deleted"
	Starting NotebookStatus = "Starting"
)

type Notebook struct {
	Name             string    `gorm:"type:varchar(256);column:name" json:"name"`
	Namespace        string    `gorm:"type:varchar(256);column:namespace" json:"namespace"`
	Image            string    `gorm:"type:varchar(256);column:image" json:"image"`
	Volumes          string    `gorm:"type:text;column:volumes" json:"volumes"`
	Cpu              string    `gorm:"type:varchar(256);column:cpu" json:"cpu"`
	Gpu              string    `gorm:"type:varchar(256);column:gpu" json:"gpu"`
	Memory           string    `gorm:"type:varchar(256);column:memory" json:"memory"`
	User             *string   `gorm:"type:varchar(256);column:user_id" json:"user_id"`
	UserName         string    `gorm:"type:varchar(256);column:user_name" json:"user_name"`
	Token            string    `gorm:"type:varchar(256);column:token" json:"token"`
	Status           string    `gorm:"type:varchar(256);column:status" json:"status"`
	ImagePullSecrets string    `gorm:"type:text;column:image_pull_secrets" json:"image_pull_secrets"`
	GmtCreated       time.Time `gorm:"type:datetime;column:gmt_created" json:"gmt_created"`
}

func (notebook Notebook) TableName() string {
	return "notebook"
}

func ConvertNotebookToDMONotebook(notebook *v1.Notebook) (*Notebook, *dmo.Notebook) {
	klog.V(5).Infof("[ConvertNotebookToDMONotebook] notebook: %s/%s", notebook.Namespace, notebook.Name)

	volumes := make([]TempVolume, 0, len(notebook.Spec.Template.Spec.Containers[0].VolumeMounts))
	for _, item := range notebook.Spec.Template.Spec.Containers[0].VolumeMounts {
		volumes = append(volumes, TempVolume{
			Name: item.Name,
			Path: item.MountPath,
		})
	}

	volumesByte, err := json.Marshal(volumes)
	if err != nil {
		volumesByte = []byte("[]")
	}

	cpu := notebook.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String()
	memory := notebook.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String()
	limits := notebook.Spec.Template.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"]
	//gpu, _ := json.Marshal(notebook.Spec.Template.Spec.Containers[0].Resources.Requests["nvidia.com/gpu"])
	gpu := limits.String()

	status := Stopped
	if len(notebook.Status.Conditions) < 1 {
		status = Starting
	} else if notebook.Status.Conditions[0].Type == "Running" {
		status = Running
	} else if notebook.Status.Conditions[0].Type == "Waiting" {
		status = Starting
	}

	pullSecrets := "[]"
	if notebook.Spec.Template.Spec.ImagePullSecrets != nil {
		imagePullSecrets, _ := json.Marshal(notebook.Spec.Template.Spec.ImagePullSecrets)
		pullSecrets = string(imagePullSecrets)
	}

	userName := notebook.Labels["userName"]
	uid := notebook.Labels["arena.kubeflow.org/console-user"]

	dmoNotebook := dmo.Notebook{
		Name:             notebook.Name,
		Namespace:        notebook.Namespace,
		Image:            notebook.Spec.Template.Spec.Containers[0].Image,
		Volumes:          string(volumesByte),
		User:             &uid,
		UserName:         userName,
		Cpu:              cpu,
		Memory:           memory,
		Gpu:              gpu,
		Status:           string(status),
		Token:            notebook.Labels["Token"],
		ImagePullSecrets: pullSecrets,
		GmtCreated:       notebook.GetCreationTimestamp().Time,
	}

	tempNotebook := Notebook{
		Name:             notebook.Name,
		Namespace:        notebook.Namespace,
		Image:            notebook.Spec.Template.Spec.Containers[0].Image,
		Volumes:          string(volumesByte),
		User:             &uid,
		UserName:         userName,
		Cpu:              cpu,
		Memory:           memory,
		Gpu:              gpu,
		Status:           string(status),
		Token:            notebook.Labels["Token"],
		ImagePullSecrets: pullSecrets,
		GmtCreated:       notebook.GetCreationTimestamp().Time,
	}
	return &tempNotebook, &dmoNotebook
}
