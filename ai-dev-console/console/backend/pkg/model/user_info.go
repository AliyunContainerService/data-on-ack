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

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

type UserInfo struct {
	// aliCloud main account id
	Aid string `json:"aid"`
	// aliCloud login account id
	Uid string `json:"uid"`
	// aliCloud display name
	Name string `json:"name"`
	// aliCloud login name
	LoginName string `json:"login_name"`
	// aliCloud ram login account
	Upn string `json:"upn"`
	// Namespaces specifies the authorized namespaces
	Namespaces []string `json:"namespaces"`
}

type UserInfoMap map[string]UserInfo

const (
	configMapName     = "kubedl-user-info-config"
	configMapKeyUsers = "users"
)

func StoreUserInfoToConfigMap(userInfo UserInfo) error {
	configMap, err := getOrCreateUserInfoConfigMap()
	if err != nil {
		return err
	}

	userInfoMap, err := getUserInfoMap(configMap)
	if err != nil {
		return err
	}

	userInfoMap[userInfo.Uid] = userInfo

	return updateUserInfoConfigMap(configMap, userInfoMap)
}

func GetUserInfoFromConfigMap(userID string) (UserInfo, error) {
	if len(userID) == 0 {
		return UserInfo{}, fmt.Errorf("userID is empty")
	}

	configMap, err := getOrCreateUserInfoConfigMap()
	if err != nil {
		return UserInfo{}, err
	}

	userInfoMap, err := getUserInfoMap(configMap)
	if err != nil {
		return UserInfo{}, err
	}

	userInfo, exists := userInfoMap[userID]
	if !exists {
		klog.Errorf("UserInfo not exists, userID: %s", userID)
		return UserInfo{}, fmt.Errorf("UserInfo not exists, userID: %s", userID)
	}

	return userInfo, nil
}

func getOrCreateUserInfoConfigMap() (*v1.ConfigMap, error) {
	configMap := &v1.ConfigMap{}
	err := clientmgr.GetCtrlClient().Get(context.TODO(),
		apitypes.NamespacedName{
			Namespace: constants.SystemNamespace,
			Name:      configMapName,
		}, configMap)

	// Create initial user info ConfigMap if not exists
	if errors.IsNotFound(err) {
		initConfigMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.SystemNamespace,
				Name:      configMapName,
			},
			Data: map[string]string{
				configMapKeyUsers: "{}",
			},
		}
		clientmgr.GetCtrlClient().Create(context.TODO(), initConfigMap)
		return initConfigMap, nil
	} else if err != nil {
		klog.Errorf("Failed to get ConfigMap, ns: %s, name: %s, err: %v", constants.SystemNamespace, configMapName, err)
		return configMap, err
	}

	return configMap, nil
}

func updateUserInfoConfigMap(configMap *v1.ConfigMap, userInfoMap UserInfoMap) error {
	if configMap == nil {
		klog.Errorf("ConfigMap is nil")
		return fmt.Errorf("ConfigMap is nil")
	}

	userInfoMapBytes, err := json.Marshal(userInfoMap)
	if err != nil {
		klog.Errorf("UserInfoMap Marshal failed, err: %v", err)
	}

	configMap.Data[configMapKeyUsers] = string(userInfoMapBytes)

	return clientmgr.GetCtrlClient().Update(context.TODO(), configMap)
}

func getUserInfoMap(configMap *v1.ConfigMap) (UserInfoMap, error) {
	if configMap == nil {
		klog.Errorf("ConfigMap is nil")
		return UserInfoMap{}, fmt.Errorf("ConfigMap is nil")
	}

	users, exists := configMap.Data[configMapKeyUsers]
	if !exists {
		klog.Errorf("ConfigMap key `%s` not exists", configMapKeyUsers)
		return UserInfoMap{}, fmt.Errorf("ConfigMap key `%s` not exists", configMapKeyUsers)
	}
	if len(users) == 0 {
		klog.Warningf("UserInfoMap is empty")
		return UserInfoMap{}, nil
	}

	userInfoMap := UserInfoMap{}
	err := json.Unmarshal([]byte(users), &userInfoMap)
	if err != nil {
		klog.Errorf("ConfigMap json Unmarshal error, content: %s, err: %v", users, err)
		return userInfoMap, err
	}

	return userInfoMap, nil
}
