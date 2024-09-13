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

package client

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

func GetVersion(client v1beta1.ImageServiceClient, request *v1beta1.VersionRequest) {
	response, err := client.Version(context.TODO(), request)
	if err != nil {
		log.Fatalf("get version failed: %v", err)
	}
	log.Println(response.Version)
}

func CommitImage(client v1beta1.ImageServiceClient, request *v1beta1.CommitRequest) {
	response, err := client.CommitImage(context.TODO(), request)
	if err != nil {
		log.Fatalf("commit image failed: %v", err)
	}
	log.Println(response.Result)
}

func PushImage(client v1beta1.ImageServiceClient, request *v1beta1.PushRequest) {
	log.Println("Start pushing the image: ", request.Image)
	log.Println("Waiting...")
	response, err := client.PushImage(context.TODO(), request)
	if err != nil {
		log.Fatalf("Image push failed: %v", err)
	}
	log.Println(response.Result)
}
