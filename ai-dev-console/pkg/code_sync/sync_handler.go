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

package code_sync

import (
	"path"

	apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultCodeRootPath = "/code"
)

type CodeSyncHandler interface {
	InitContainer(optsConfig []byte, mountVolume *v1.Volume) (c *v1.Container, codePath string, repoName string, err error)
}

type SyncOptions struct {
	// Code source address.(required)
	Source string `json:"source"`
	// Image contains toolkits to execute syncing code.
	Image string `json:"image,omitempty"`
	// Code root/destination directory path.
	// Root: the path to save downloaded files.
	// Dest: the name of (a symlink to) a directory in which to check-out files
	RootPath string `json:"rootPath,omitempty"`
	DestPath string `json:"destPath,omitempty"`

	// Relative Code path of workingDir that will be mounted to main containers.
	RelativeCodePath string `json:"relativeCodePath,omitempty"`

	// User-customized environment variables.
	Envs []v1.EnvVar `json:"envs,omitempty"`
}

func InjectCodeSyncInitContainers(metaObj metav1.Object, specTemplate *v1.PodTemplateSpec) error {
	var err error

	if cfg, ok := metaObj.GetAnnotations()[apiv1.AnnotationGitSyncConfig]; ok {
		if err = injectCodeSyncInitContainer([]byte(cfg), &gitSyncHandler{}, specTemplate, &v1.Volume{
			Name:         "git-sync",
			VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
		}); err != nil {
			return err
		}
	}

	// TODO(SimonCqk): support other sources.

	return nil
}

func injectCodeSyncInitContainer(optsConfig []byte, handler CodeSyncHandler, specTemplate *v1.PodTemplateSpec, mountVolume *v1.Volume) error {
	initContainer, relativeCodePath, repoName, err := handler.InitContainer(optsConfig, mountVolume)
	if err != nil {
		return err
	}

	initContainer.Resources = *specTemplate.Spec.Containers[0].Resources.DeepCopy()
	specTemplate.Spec.InitContainers = append(specTemplate.Spec.InitContainers, *initContainer)

	// TODO: May be change the value of SubPath if code sync support other storage (hdfs/http)
	// Inject volumes and volume mounts into main containers.
	specTemplate.Spec.Volumes = append(specTemplate.Spec.Volumes, *mountVolume)
	for idx := range specTemplate.Spec.Containers {
		container := &specTemplate.Spec.Containers[idx]
		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			Name:      mountVolume.Name,
			ReadOnly:  false,
			MountPath: path.Join(container.WorkingDir, relativeCodePath),
			SubPath:   repoName,
		})
	}

	return nil
}
