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
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"io/ioutil"
	"k8s.io/klog"
)

const (
	ConfigPath = "/var/addon/token-config"
)

type AkClient struct {
	metadataClient *MetadataClient
}

func NewAkClient() (*AkClient, error) {
	metadataClient, err := NewMetadataClient()
	if err != nil {
		return nil, err
	}
	return &AkClient{
		metadataClient: metadataClient,
	}, nil
}

func (i *AkClient) GetAKInfo() (*model.AKInfo, error) {
	akInfo := &model.AKInfo{}
	encodeTokenCfg, err := ioutil.ReadFile(ConfigPath)
	if err == nil {
		if err := akInfo.DecryptFromString(string(encodeTokenCfg)); err != nil {
			klog.Errorf("decryptFromString failed err: %v", err)
			return nil, err
		}
	} else {
		roleName, err := i.metadataClient.GetRoleName()
		if err != nil {
			klog.Errorf("get role name by meta client failed err:%s", err)
			return nil, err
		}
		roleAuth, err := i.metadataClient.GetRoleAuth(roleName)
		if err != nil {
			klog.Errorf("get role auth by meta client failed err:%s", err)
			return nil, err
		}
		akInfo.AccessKeyId = roleAuth.AccessKeyId
		akInfo.AccessKeySecret = roleAuth.AccessKeySecret
		akInfo.SecurityToken = roleAuth.SecurityToken
		akInfo.Expiration = roleAuth.Expiration
	}
	return akInfo, nil
}
