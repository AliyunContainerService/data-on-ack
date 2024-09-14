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
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	v1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"

	"k8s.io/klog"
)

var (
	consoleClientMgr = &ConsoleClientMgr{}
)

type ConsoleClientMgr struct {
	aliyunRamClient *AliyunRamClient
}

func GetAliyunRamClient() *AliyunRamClient {
	return consoleClientMgr.aliyunRamClient
}

func GetClusterId() string {
	configMap := &v1.ConfigMap{}
	err := clientmgr.GetCtrlClient().Get(context.TODO(), apitypes.NamespacedName{
		Namespace: "kube-system",
		Name:      "ack-cluster-profile",
	}, configMap)
	if err != nil {
		klog.Errorf("oauth failed get oauth configMap, ns: %s, name: %s, err: %v", constants.SystemNamespace, configMap, err)
		return ""
	}
	return configMap.Data["clusterid"]
}

func Init() {
	var err error
	consoleClientMgr.aliyunRamClient, err = NewAliyunRamClient()
	if err != nil {
		klog.Errorf("init console client manager error:%v", err)
		return
	}
}
