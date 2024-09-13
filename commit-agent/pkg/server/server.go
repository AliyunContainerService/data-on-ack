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

package server

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"net"
	"os"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/operate"
	"github.com/AliyunContainerService/data-on-ack/commit-agent/v1beta1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	// Unix Domain Socket
	netProtocol = "unix"
	Version     = "0.1.0"
)

type ImageServer struct {
	v1beta1.UnimplementedImageServiceServer
	pathToUnixSocket string
	net.Listener
	*grpc.Server
}

// New creates an instance of the Image Service Server.
func New(pathToUnixSocketFile string) (*ImageServer, error) {
	imageServer := &ImageServer{
		pathToUnixSocket: pathToUnixSocketFile,
	}
	return imageServer, nil
}

func (s *ImageServer) setupRPCServer() error {
	if err := s.cleanSockFile(); err != nil {
		return err
	}

	listener, err := net.Listen(netProtocol, s.pathToUnixSocket)
	if err != nil {
		return fmt.Errorf("failed to start listener, error: %v", err)
	}
	s.Listener = listener
	log.Infof("register unix domain socket: %s", s.pathToUnixSocket)
	server := grpc.NewServer()
	v1beta1.RegisterImageServiceServer(server, s)
	s.Server = server
	return nil
}

func (s *ImageServer) StartRPCServer() (*grpc.Server, chan error) {
	errorChan := make(chan error, 1)
	if err := s.setupRPCServer(); err != nil {
		errorChan <- err
		close(errorChan)
		return nil, errorChan
	}

	go func() {
		defer func() {
			close(errorChan)
		}()
		errorChan <- s.Serve(s.Listener)
	}()
	log.Infof("image server started successfully.")
	return s.Server, errorChan
}

func (s *ImageServer) cleanSockFile() error {
	err := unix.Unlink(s.pathToUnixSocket)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete socket file, error: %v", err)
	}
	return nil
}

func (s *ImageServer) Version(ctx context.Context, request *v1beta1.VersionRequest) (*v1beta1.VersionResponse, error) {
	log.Infoln(Version)
	return &v1beta1.VersionResponse{Version: Version}, nil
}

func (s *ImageServer) CommitImage(ctx context.Context, request *v1beta1.CommitRequest) (*v1beta1.CommitResponse, error) {
	result, err := operate.CommitContainer(request.ContainerID, request.Image)

	return &v1beta1.CommitResponse{Result: result}, err
}

func (s *ImageServer) PushImage(ctx context.Context, request *v1beta1.PushRequest) (*v1beta1.PushResponse, error) {
	result, err := operate.PushImage(request.Image, request.Username, request.Password)

	return &v1beta1.PushResponse{Result: result}, err
}
