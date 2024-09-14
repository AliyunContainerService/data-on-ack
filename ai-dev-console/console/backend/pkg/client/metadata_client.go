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
	"encoding/json"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	MetaDataEndpoint = "http://100.100.100.200"
)

type MetadataClient struct {
	httpClient *http.Client
}

func NewMetadataClient() (*MetadataClient, error) {
	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{Transport: tr}
	return &MetadataClient{httpClient: client}, nil
}

func (c *MetadataClient) getStringFromUrl(url string) (string, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *MetadataClient) GetRegionId() (regionId string, err error) {
	regionIdUrl := fmt.Sprintf("%s/latest/meta-data/region-id", MetaDataEndpoint)
	return c.getStringFromUrl(regionIdUrl)
}

func (c *MetadataClient) GetRoleName() (roleName string, err error) {
	roleNameUrl := fmt.Sprintf("%s/latest/meta-data/ram/security-credentials/", MetaDataEndpoint)
	return c.getStringFromUrl(roleNameUrl)
}

func (c *MetadataClient) GetRoleAuth(roleName string) (roleAuth *model.RoleAuth, err error) {
	roleAuth = &model.RoleAuth{}
	roleAuthUrl := fmt.Sprintf("%s/latest/meta-data/ram/security-credentials/%s", MetaDataEndpoint, roleName)
	roleAuthStr, err := c.getStringFromUrl(roleAuthUrl)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(roleAuthStr), roleAuth)
	if err != nil {
		return nil, err
	}
	return roleAuth, err
}
