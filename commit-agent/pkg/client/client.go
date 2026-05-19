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
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
)

// GetVersion calls Version on the agent and prints the response.
// Returns the version string and an error so callers may handle failure
// (Cobra's RunE expects error returns) instead of os.Exit'ing.
func GetVersion(ctx context.Context, c v1beta1.ImageServiceClient, request *v1beta1.VersionRequest) (string, error) {
	resp, err := c.Version(ctx, request)
	if err != nil {
		return "", fmt.Errorf("get version: %w", err)
	}
	log.Infoln(resp.Version)
	return resp.Version, nil
}

// CommitImage requests a commit and prints the result.
func CommitImage(ctx context.Context, c v1beta1.ImageServiceClient, request *v1beta1.CommitRequest) error {
	resp, err := c.CommitImage(ctx, request)
	if err != nil {
		return fmt.Errorf("commit image: %w", err)
	}
	log.Infoln(resp.Result)
	return nil
}

// PushImage requests a push and prints the result.
func PushImage(ctx context.Context, c v1beta1.ImageServiceClient, request *v1beta1.PushRequest) error {
	log.Infoln("Start pushing the image:", request.Image)
	log.Infoln("Waiting...")
	resp, err := c.PushImage(ctx, request)
	if err != nil {
		return fmt.Errorf("push image: %w", err)
	}
	log.Infoln(resp.Result)
	return nil
}
