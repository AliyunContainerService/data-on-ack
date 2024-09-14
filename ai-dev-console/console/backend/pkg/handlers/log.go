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
    
package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
)

func NewLogHandler(eventStorage string) (*LogHandler, error) {
	eventBackend := registry.GetEventBackend(eventStorage)
	if eventBackend == nil {
		return nil, fmt.Errorf("no event backend storage named: %s", eventStorage)
	}
	err := eventBackend.Initialize()
	if err != nil {
		return nil, err
	}
	return &LogHandler{eventBackend: eventBackend}, nil
}

type LogHandler struct {
	eventBackend backends.EventStorageBackend
}

func (lh *LogHandler) GetLogs(namespace, jobKind, jobName, podName, userName string, from, to time.Time) ([]string, error) {
	logs, err := lh.eventBackend.UserName(userName).ListLogs(namespace, jobKind, jobName, podName, 2000, from, to)
	if err != nil {
		return nil, err
	}
	return logs, nil
}

func (lh *LogHandler) DownloadLogs(namespace, jobKind, jobName, podName, userName string, from, to time.Time) ([]byte, error) {
	logs, err := lh.eventBackend.UserName(userName).ListLogs(namespace, jobKind, jobName, podName, -1, from, to)
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(logs, "\r\n")), nil
}

func (lh *LogHandler) GetEvents(namespace, objName, userId string, from, to time.Time) ([]string, error) {
	list, err := lh.eventBackend.UserName(userId).ListEvents(namespace, objName, from, to)
	if err != nil {
		return nil, err
	}
	if len(list) > 2000 {
		list = list[:2000]
	}
	msg := []string{}
	for _, ev := range list {
		msg = append(msg, fmt.Sprintf("%s %s", ev.LastTimestamp.Format(model.TimeFormat), ev.Message))
	}
	return msg, nil
}
