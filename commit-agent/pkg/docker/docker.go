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
	"io"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"

	_type "github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/type"
)

type Client struct {
	Ctx    context.Context
	Client client.CommonAPIClient
}

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func NewDockerClient() (_type.ContainerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	return &Client{
		Ctx:    context.Background(),
		Client: cli,
	}, nil
}

func (c *Client) CommitImageFromSelf(containerID string, image string) error {

	commitOps := types.ContainerCommitOptions{
		Reference: image,
		Comment:   "",
		Author:    "",
		Changes:   []string{},
		Pause:     false,
		Config:    &container.Config{},
	}

	_, err := c.Client.ContainerCommit(c.Ctx, containerID, commitOps)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) PushImageFromSelf(imageName, username, password string) error {
	ref, err := reference.ParseNormalizedNamed(imageName)
	switch {
	case err != nil:
		return err
	case reference.IsNameOnly(ref):
		ref = reference.TagNameOnly(ref)
		if tagged, ok := ref.(reference.Tagged); ok {
			log.Infof("Using default tag: %s\n", tagged.Tag())
		}
	}

	authConfig := registrytypes.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: reference.Domain(ref),
	}

	encodedAuth, err := registrytypes.EncodeAuthConfig(authConfig)
	if err != nil {
		return err
	}

	pushOps := types.ImagePushOptions{
		RegistryAuth: encodedAuth,
		All:          false,
	}

	response, err := c.Client.ImagePush(c.Ctx, reference.FamiliarString(ref), pushOps)
	if err != nil {
		log.Infof("push image failed: %v", err)
		return err
	}

	if err := checkResponse(response); err != nil {
		return err
	}

	return nil
}

func checkResponse(rd io.Reader) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
		log.Println(scanner.Text())
	}

	errLine := &ErrorLine{}
	err := json.Unmarshal([]byte(lastLine), errLine)
	if err != nil {
		return err
	}
	if errLine.Error != "" {
		return errors.New(errLine.ErrorDetail.Message)
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
