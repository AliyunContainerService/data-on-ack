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
    
package model

type OAuthInfo struct {

	// aliCloud third-part app id
	AppId string `json:"appId"`
	// aliCloud third-part app secret
	AppSecret string `json:"appSecret"`

	WebAppRedirectDomain string   `json:"webAppRedirectDomain"`
	UserInfo             UserInfo `json:"userInfo"`
}

// GetOauthInfo from configmap
func GetOauthInfo(oauthConfig map[string]string) OAuthInfo {
	if oauthConfig == nil {
		return OAuthInfo{}
	}

	OAuthInfo := OAuthInfo{
		AppId:     oauthConfig["appId"],
		AppSecret: oauthConfig["appSecret"],
		UserInfo: UserInfo{
			Aid:       oauthConfig["aid"],
			Uid:       oauthConfig["uid"],
			Name:      oauthConfig["name"],
			LoginName: oauthConfig["loginName"],
			Upn:       oauthConfig["upn"],
		},
	}
	return OAuthInfo
}
