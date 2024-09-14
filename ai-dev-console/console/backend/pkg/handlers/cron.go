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
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/registry"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CronHandler struct {
	client         client.Client
	storageBackend backends.ObjectStorageBackend
	clientBackend  backends.ObjectClientBackend
}

func NewCronHandler(objStorage string, clientTypeName string) (*CronHandler, error) {
	objBackend := registry.GetObjectBackend(objStorage)
	if objBackend == nil {
		return nil, fmt.Errorf("no object backend storage named: %s", objStorage)
	}
	err := objBackend.Initialize()
	if err != nil {
		return nil, err
	}

	clientBackend := registry.GetActionBackend(clientTypeName)
	if clientBackend == nil {
		return nil, fmt.Errorf("no action backend named: %s", objStorage)
	}
	err = clientBackend.Initialize()
	if err != nil {
		return nil, err
	}

	return &CronHandler{
		client:         clientmgr.GetCtrlClient(),
		storageBackend: objBackend,
		clientBackend:  clientBackend,
	}, nil
}

func (cj *CronHandler) ListCron(query *backends.CronQuery) ([]model.CronInfo, error) {
	crons, err := cj.storageBackend.UserName(query.UserName).ListCrons(query)
	if err != nil {
		return nil, err
	}

	cronInfos := make([]model.CronInfo, 0, len(crons))
	for _, cron := range crons {
		cronInfos = append(cronInfos, model.ConvertDMOCronToCronInfo(cron))
	}

	return cronInfos, nil
}

func (cj *CronHandler) ListCronHistory(userName, namespace, name, jobName, jobStatus string) ([]model.JobInfo, error) {
	cronHistories, err := cj.storageBackend.UserName(userName).ListCronHistories(namespace, name, jobName, jobStatus, "")
	if err != nil {
		return nil, err
	}

	cronHistoryInfos := make([]model.JobInfo, 0, len(cronHistories))
	for _, cronHistory := range cronHistories {
		cronHistoryInfos = append(cronHistoryInfos, model.ConvertDMOJobToJobInfo(cronHistory))
	}

	return cronHistoryInfos, nil
}

func (cj *CronHandler) GetCron(userName, namespace, name string) (model.CronInfo, error) {
	cron, err := cj.storageBackend.UserName(userName).GetCron(namespace, name, "")
	if err != nil {
		return model.CronInfo{}, err
	}

	return model.ConvertDMOCronToCronInfo(cron), nil
}

func (cj *CronHandler) DeleteCron(userName, namespace, name string) error {
	klog.Infof("[CronHandler] delete cron, userName:%s namespace:%s name:%s", userName, namespace, name)
	return cj.storageBackend.UserName(userName).DeleteCron(namespace, name, "")
}

func (cj *CronHandler) StopCron(userName, namespace, name string) error {
	klog.Infof("[StopCron] stop cron, userName:%s namespace:%s name:%s", userName, namespace, name)
	return cj.clientBackend.UserName(userName).StopCron(namespace, name, "")
}

func (cj *CronHandler) SuspendCron(userName, namespace, name string) error {
	return cj.clientBackend.UserName(userName).SuspendCron(namespace, name, "")
}

func (cj *CronHandler) ResumeCron(userName, namespace, name string) error {
	return cj.clientBackend.UserName(userName).ResumeCron(namespace, name, "")
}
