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

package operate

import (
	"fmt"
	"os"
	"syscall"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/containerd"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/docker"
	_type "github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/type"
	log "github.com/sirupsen/logrus"
)

func FileExist(path string) bool {
	err := syscall.Access(path, syscall.F_OK)
	return !os.IsNotExist(err)
}

func CommitContainer(containerID, image string) (string, error) {
	isDocker := FileExist("/host/run/docker.sock")
	var client _type.ContainerClient
	var err error

	if isDocker {
		client, err = docker.NewDockerClient()
		if err != nil {
			log.Errorln("Docker client init error", err)
			return "Docker client init error", err
		}
	} else {
		client, err = containerd.NewContainerdClient()
		if err != nil {
			log.Errorln("Containerd client init error", err)
			return "Containerd client init error", err
		}
	}

	err = client.CommitImageFromSelf(containerID, image)
	if err != nil {
		log.Errorln("Container save error", err)
		return "Container save error", err
	}
	msg := fmt.Sprintf("Container save success, image: %s", image)
	log.Println(msg)
	return msg, nil
}

func PushImage(image string, username string, password string) (string, error) {
	isDocker := FileExist("/host/run/docker.sock")
	var client _type.ContainerClient
	var err error

	if isDocker {
		client, err = docker.NewDockerClient()
		if err != nil {
			log.Errorln("Docker client init error", err)
			return "Docker client init error", err
		}
	} else {
		client, err = containerd.NewContainerdClient()
		if err != nil {
			log.Errorln("Containerd client init error", err)
			return "Containerd client init error", err
		}
	}

	err = client.PushImageFromSelf(image, username, password)
	if err != nil {
		log.Errorln("image push error:", err)
		return "image push error", err
	}
	msg := fmt.Sprintf("Image pushed successfully: %s", image)
	log.Println(msg)
	return msg, nil
}
