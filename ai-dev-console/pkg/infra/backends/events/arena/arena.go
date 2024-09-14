/*
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

package arena

import (
	"bytes"
	"fmt"
	clientregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/tenant"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/events/apiserver"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/utils"

	"github.com/kubeflow/arena/pkg/apis/arenaclient"
	"github.com/kubeflow/arena/pkg/apis/logger"
	"k8s.io/klog"
)

func NewArenaEventBackend() backends.EventStorageBackend {
	return &arenaEventBackend{
		EventStorageBackend: apiserver.NewAPIServerEventBackend(),
		arena:               clientmgr.GetArenaClient(),
	}
}

var _ backends.EventStorageBackend = &arenaEventBackend{}

type arenaEventBackend struct {
	backends.EventStorageBackend
	arena    *arenaclient.ArenaClient
	userName string
}

func (a *arenaEventBackend) Name() string {
	return "arena"
}

func (a *arenaEventBackend) UserName(userName string) backends.EventStorageBackend {
	copyArenaEventBackend := &arenaEventBackend{
		EventStorageBackend: a.EventStorageBackend,
		arena:               a.arena,
		userName:            userName,
	}
	return copyArenaEventBackend
}

func (a *arenaEventBackend) getArenaClient() *arenaclient.ArenaClient {
	var arena *arenaclient.ArenaClient
	var err error
	if a.userName == "" {
		arena = a.arena
	} else {
		arena, err = clientregistry.GetArenaClient(a.userName)
		if err != nil {
			klog.Errorf("get arena client of user %s failed, err:%v", a.userName, err)
		}
	}
	return arena
}

func (a *arenaEventBackend) ListLogs(namespace, jobKind, jobName, name string, maxLine int64, from, to time.Time) ([]string, error) {
	klog.Infof("[arenaEventBackend.ListLogs] ns:%s name:%s jobKind:%s jobName:%s", namespace, name, jobKind, jobName)
	buf := new(bytes.Buffer)
	logArgs, err := logger.NewLoggerBuilder().Instance(name).WriterCloser(&writerCloser{Buffer: buf}).Build()
	if err != nil {
		fmt.Printf("failed to build log args,reason: %v\n", err)
		return nil, err
	}

	err = a.getArenaClient().Training().Namespace(namespace).Logs(jobName, utils.GetArenaJobTypeFromKind(jobKind), logArgs)
	if err != nil {
		klog.Errorf("list %v/%v logs error: %v", namespace, name, err)
		return []string{}, err
	}
	return strings.Split(buf.String(), "\n"), nil
}

type writerCloser struct {
	*bytes.Buffer
}

func (wc *writerCloser) Close() error {
	return nil
}
