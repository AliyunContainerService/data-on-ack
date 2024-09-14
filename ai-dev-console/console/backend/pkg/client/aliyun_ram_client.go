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
	utils "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
	log "github.com/sirupsen/logrus"
	"time"
)

var (
	RamDefaultEndpoint    = "ims.aliyuncs.com"
	RamDefaultVPCEndpoint = "ims.vpc-proxy.aliyuncs.com"
)

type AliyunRamClient struct {
	akInfo    *model.AKInfo
	ramClient *ims.Client
	akClient  *AkClient
}

func NewAliyunRamClient() (*AliyunRamClient, error) {
	akClient, err := NewAkClient()
	if err != nil {
		return nil, err
	}
	return &AliyunRamClient{
		akClient: akClient,
	}, nil
}

func (c *AliyunRamClient) GetRamClient() (ramClient *ims.Client, err error) {
	if c.akInfo == nil {
		if c.akInfo, err = c.akClient.GetAKInfo(); err != nil {
			return nil, err
		}
	}

	isTokenExpired := isTokenExpired(c.akInfo)
	if isTokenExpired {
		if c.akInfo, err = c.akClient.GetAKInfo(); err != nil {
			return nil, err
		}
	}
	if isTokenExpired || nil == c.ramClient {
		config := NewRamClientConfigFromAKInfo(*c.akInfo)
		c.ramClient, err = ims.NewClient(config)
		if err != nil {
			return nil, err
		}
	}
	return c.ramClient, nil
}

func (c *AliyunRamClient) GetMetadataClient() *MetadataClient {
	return c.akClient.metadataClient
}

func isTokenExpired(akInfo *model.AKInfo) bool {
	layout := "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, akInfo.Expiration)
	if err != nil {
		return true
	}
	return t.Before(time.Now())
}

func NewRamClientConfigFromAKInfo(akInfo model.AKInfo) (config *openapi.Config) {
	config = &openapi.Config{}
	config.AccessKeyId = &akInfo.AccessKeyId
	config.AccessKeySecret = &akInfo.AccessKeySecret
	config.SecurityToken = &akInfo.SecurityToken
	endPoint := RamDefaultVPCEndpoint
	if !utils.IsDomainNameAvailable(RamDefaultEndpoint) {
		endPoint = RamDefaultEndpoint
	}
	log.Infof("using ram endpoint:%s", endPoint)
	config.Endpoint = &endPoint
	return config
}
