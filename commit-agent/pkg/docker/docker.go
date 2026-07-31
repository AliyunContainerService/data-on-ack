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

package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"

	_type "github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/type"
)

// maxStreamLine bounds an individual JSON status line emitted by the docker
// daemon during push. The default bufio.Scanner buffer is 64 KiB which is too
// small for some registries and image manifests.
const maxStreamLine = 4 * 1024 * 1024

// Client wraps the docker SDK client.
type Client struct {
	cli client.CommonAPIClient
}

// errorLine matches the JSON status frames emitted by the docker daemon.
type errorLine struct {
	Error       string      `json:"error"`
	ErrorDetail errorDetail `json:"errorDetail"`
}

type errorDetail struct {
	Message string `json:"message"`
}

// NewDockerClient initialises the docker daemon client. It returns an error
// instead of panicking so the caller can fail gracefully.
func NewDockerClient() (_type.ContainerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("init docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// CommitImageFromSelf commits the live container into a new image reference.
func (c *Client) CommitImageFromSelf(ctx context.Context, containerID, image string) error {
	commitOps := types.ContainerCommitOptions{
		Reference: image,
		Comment:   "",
		Author:    "",
		Changes:   []string{},
		Pause:     false,
		Config:    &container.Config{},
	}

	if _, err := c.cli.ContainerCommit(ctx, containerID, commitOps); err != nil {
		return fmt.Errorf("docker commit: %w", err)
	}
	return nil
}

// PushImageFromSelf pushes the named image to its remote registry.
func (c *Client) PushImageFromSelf(ctx context.Context, imageName, username, password string) error {
	ref, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return fmt.Errorf("parse image %q: %w", imageName, err)
	}
	if reference.IsNameOnly(ref) {
		ref = reference.TagNameOnly(ref)
		if tagged, ok := ref.(reference.Tagged); ok {
			log.Infof("Using default tag: %s", tagged.Tag())
		}
	}

	authConfig := registrytypes.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: reference.Domain(ref),
	}
	encodedAuth, err := registrytypes.EncodeAuthConfig(authConfig)
	if err != nil {
		return fmt.Errorf("encode registry auth: %w", err)
	}

	pushOps := types.ImagePushOptions{
		RegistryAuth: encodedAuth,
		All:          false,
	}

	response, err := c.cli.ImagePush(ctx, reference.FamiliarString(ref), pushOps)
	if err != nil {
		return fmt.Errorf("docker push: %w", err)
	}
	defer response.Close()

	return checkResponse(response)
}

// checkResponse drains the docker push status stream and surfaces the final
// error frame, if any. Bufio's default token size is too small for some
// registry payloads so we expand the scanner buffer.
func checkResponse(rd io.Reader) error {
	scanner := bufio.NewScanner(rd)
	scanner.Buffer(make([]byte, 64*1024), maxStreamLine)

	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
		log.Debugln(lastLine)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read push stream: %w", err)
	}
	if lastLine == "" {
		return nil
	}

	var ln errorLine
	if err := json.Unmarshal([]byte(lastLine), &ln); err != nil {
		// The daemon sometimes returns a non-JSON trailer; treat as success
		// so long as no earlier frame raised an error.
		return nil
	}
	if ln.Error != "" {
		msg := ln.ErrorDetail.Message
		if msg == "" {
			msg = ln.Error
		}
		return errors.New(msg)
	}
	return nil
}
