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

package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/AliyunContainerService/data-on-ack/commit-agent/pkg/server"
)

var (
	socketAddress = flag.String("socket-address", "/host/run/commit-agent/commit-agent.sock", "the socket address which was listened by commit-agent server")
)

func main() {
	flag.Parse()

	mustValidateFlags(*socketAddress)
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	p, err := server.New(*socketAddress)
	if err != nil {
		log.Fatalf("failed to init commit-agent, %v", err)
	}
	svr, errChan := p.StartRPCServer()
	defer svr.GracefulStop()

	for {
		select {
		case sig := <-signals:
			log.Fatalf("captured %v, shutting down", sig)
		case err := <-errChan:
			log.Fatal(err)
		}
	}
}

func mustValidateFlags(pathToUnixSocket string) {
	// Using an actual socket file instead of in-memory Linux socket namespace object.
	log.Infof("Checking socket path %s", pathToUnixSocket)
	if !strings.HasPrefix(pathToUnixSocket, "@") {
		socketDir := filepath.Dir(pathToUnixSocket)
		_, err := os.Stat(socketDir)
		log.Infof("Unix Socket directory is %s", socketDir)
		if err != nil && os.IsNotExist(err) {
			log.Infof(" Directory %s portion of socket-address flag: %s does not exist, create it.", socketDir, pathToUnixSocket)
			err = os.MkdirAll(socketDir, os.ModeDir)
			if err != nil {
				log.Fatalf(" Directory %s create failed, err: %s", socketDir, err)
			}
		}
	}
	log.Infof("Communication between commit-cli and commit-agent will be via %s", pathToUnixSocket)
}
