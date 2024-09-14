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
	"context"
	"encoding/json"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	clientmgr "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	clientregistry "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/tenant"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CodesourceConfigMapName     = "kubedl-codesource-config"
	CodesourceConfigMapKey      = "codesource"
	CodesourceSecretGitUsername = "git_username"
	CodesourceSecretGitPassword = "git_password"
)

func NewCodeSourceHandler() *CodeSourceHandler {
	return &CodeSourceHandler{client: clientmgr.GetCtrlClient()}
}

type CodeSourceHandler struct {
	client client.Client
}

// post
func (ov *CodeSourceHandler) PostCodeSourceToConfigMap(userName string, codeSource model.CodeSource) error {
	klog.Infof("CodeSource : %s", codeSource)

	configMap, err := getOrCreateCodeSourceConfigMap(userName)
	if err != nil {
		return err
	}

	codeSourceMap, err := getCodeSourceMap(userName, configMap)
	if err != nil {
		return err
	}

	_, exists := codeSourceMap[codeSource.Name]
	if exists {
		klog.Errorf("CodeSource exists, name: %s", codeSource.Name)
		return fmt.Errorf("CodeSource exists, name: %s", codeSource.Name)
	}

	gitUsername := codeSource.GitUsername
	gitPassword := codeSource.GitPassword

	codeSource.GitUsername = ""
	codeSource.GitPassword = ""
	codeSourceMap[codeSource.Name] = codeSource

	err = setCodeSourceConfigMap(userName, configMap, codeSourceMap)
	if err != nil {
		return err
	}

	secretName := generateSecretName(codeSource.Name)
	return createSecret(userName, secretName, gitUsername, gitPassword)
}

// delete
func (ov *CodeSourceHandler) DeleteCodeSourceFromConfigMap(userName, name string) error {
	if len(name) == 0 {
		return fmt.Errorf("name is empty")
	}

	configMap, err := getOrCreateCodeSourceConfigMap(userName)
	if err != nil {
		return err
	}

	codeSourceMap, err := getCodeSourceMap(userName, configMap)
	if err != nil {
		return err
	}

	_, exists := codeSourceMap[name]
	if !exists {
		klog.Errorf("CodeSource not exists, name: %s", name)
		return fmt.Errorf("CodeSource not exists, name: %s", name)
	}

	delete(codeSourceMap, name)

	err = setCodeSourceConfigMap(userName, configMap, codeSourceMap)
	if err != nil {
		return err
	}

	secretName := generateSecretName(name)
	return deleteSecret(userName, secretName)
}

// put
func (ov *CodeSourceHandler) PutCodeSourceToConfigMap(userName string, codeSource model.CodeSource) error {
	configMap, err := getOrCreateCodeSourceConfigMap(userName)
	if err != nil {
		return err
	}

	codeSourceMap, err := getCodeSourceMap(userName, configMap)
	if err != nil {
		return err
	}

	codeSource.CreateTime = codeSourceMap[codeSource.Name].CreateTime

	codeSourceMap[codeSource.Name] = codeSource

	return setCodeSourceConfigMap(userName, configMap, codeSourceMap)
}

// get
func (ov *CodeSourceHandler) GetCodeSourceFromConfigMap(userName, name string) (model.CodeSource, error) {
	if len(name) == 0 {
		return model.CodeSource{}, fmt.Errorf("name is empty")
	}

	configMap, err := getOrCreateCodeSourceConfigMap(userName)
	if err != nil {
		return model.CodeSource{}, err
	}

	codeSourceMap, err := getCodeSourceMap(userName, configMap)
	if err != nil {
		return model.CodeSource{}, err
	}

	codeSource, exists := codeSourceMap[name]
	if !exists {
		klog.Errorf("CodeSource %s not exists", name)
		return model.CodeSource{}, fmt.Errorf("CodeSource %s not exists", name)
	}

	secretName := generateSecretName(name)
	secret, err := getSecret(userName, secretName)
	if err == nil && secret != nil && secret.Data != nil {
		if secret.Data[CodesourceSecretGitUsername] != nil {
			codeSource.GitUsername = string(secret.Data[CodesourceSecretGitUsername])
		}

		if secret.Data[CodesourceSecretGitPassword] != nil {
			codeSource.GitPassword = string(secret.Data[CodesourceSecretGitPassword])
		}
	}

	return codeSource, nil
}

// get all
func (ov *CodeSourceHandler) ListCodeSourceFromConfigMap(userName string) (model.CodeSourceMap, error) {
	configMap, err := getOrCreateCodeSourceConfigMap(userName)
	if err != nil {
		return model.CodeSourceMap{}, err
	}

	codeSourceMap, err := getCodeSourceMap(userName, configMap)
	if err != nil {
		return model.CodeSourceMap{}, err
	}

	return codeSourceMap, nil
}

// set
func setCodeSourceConfigMap(userName string, configMap *v1.ConfigMap, codeSourceMap model.CodeSourceMap) error {
	if configMap == nil {
		klog.Errorf("ConfigMap is nil")
		return fmt.Errorf("ConfigMap is nil")
	}

	codeSourceMapBytes, err := json.Marshal(codeSourceMap)
	if err != nil {
		klog.Errorf("CodeSourceMap Marshal failed, err: %v", err)
	}

	configMap.Data[CodesourceConfigMapKey] = string(codeSourceMapBytes)

	ctrlClient, err := clientregistry.GetCtrlClient(userName)
	if err != nil {
		return err
	}

	rs := ctrlClient.Update(context.TODO(), configMap)

	return rs
}

func getOrCreateCodeSourceConfigMap(userName string) (*v1.ConfigMap, error) {
	ctrlClient, err := clientregistry.GetCtrlClient(userName)
	if err != nil {
		return nil, err
	}

	configMap := &v1.ConfigMap{}
	err = ctrlClient.Get(context.TODO(),
		apitypes.NamespacedName{
			Namespace: constants.SystemNamespace,
			Name:      CodesourceConfigMapName,
		}, configMap)

	// Create initial user info ConfigMap if not exists
	if errors.IsNotFound(err) {
		initConfigMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.SystemNamespace,
				Name:      CodesourceConfigMapName,
			},
			Data: map[string]string{
				CodesourceConfigMapKey: "{}",
			},
		}
		err = ctrlClient.Create(context.TODO(), initConfigMap)
		if err != nil {
			return nil, err
		}
		return initConfigMap, nil
	} else if err != nil {
		klog.Errorf("Failed to get ConfigMap, ns: %s, name: %s, err: %v", constants.SystemNamespace, CodesourceConfigMapName, err)
		return configMap, err
	}

	return configMap, nil
}

func getCodeSourceMap(userName string, configMap *v1.ConfigMap) (model.CodeSourceMap, error) {
	if configMap == nil {
		klog.Errorf("ConfigMap is nil")
		return model.CodeSourceMap{}, fmt.Errorf("ConfigMap is nil")
	}

	codesources, exists := configMap.Data[CodesourceConfigMapKey]
	if !exists {
		klog.Errorf("ConfigMap key `%s` not exists", CodesourceConfigMapKey)
		return model.CodeSourceMap{}, fmt.Errorf("ConfigMap key `%s` not exists", CodesourceConfigMapKey)
	}
	if len(codesources) == 0 {
		klog.Warningf("CodeSources is empty")
		return model.CodeSourceMap{}, nil
	}

	codeSourceMap := model.CodeSourceMap{}
	err := json.Unmarshal([]byte(codesources), &codeSourceMap)
	if err != nil {
		klog.Errorf("ConfigMap json Unmarshal error, content: %s, err: %v", codesources, err)
		return codeSourceMap, err
	}

	for k, v := range codeSourceMap {
		secretName := generateSecretName(v.Name)
		secret, err := getSecret(userName, secretName)
		if err == nil && secret != nil && secret.Data != nil {
			if secret.Data[CodesourceSecretGitUsername] != nil {
				v.GitUsername = string(secret.Data[CodesourceSecretGitUsername])
			}

			if secret.Data[CodesourceSecretGitPassword] != nil {
				v.GitPassword = string(secret.Data[CodesourceSecretGitPassword])
			}

			codeSourceMap[k] = v
		}
	}

	return codeSourceMap, nil
}

func createSecret(userName string, secretName string, username string, password string) error {
	klog.Infof("create secret, name: %s username: %s password: %s", secretName, username, password)
	dynamicClient, err := clientregistry.GetCtrlDynamicClient(userName)
	if err != nil {
		return err
	}

	secret := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      secretName,
			"namespace": constants.SystemNamespace,
		},
		"data": map[string][]byte{
			CodesourceSecretGitUsername: []byte(username),
			CodesourceSecretGitPassword: []byte(password),
		},
	}

	_, err = dynamicClient.Resource(v1.SchemeGroupVersion.WithResource("secrets")).
		Namespace(constants.SystemNamespace).
		Create(context.Background(),
			&unstructured.Unstructured{
				Object: secret,
			}, metav1.CreateOptions{})

	if err != nil {
		klog.Errorf("create secret failed, err:%v", err)
	}

	return err
}

func getSecret(userName string, secretName string) (*v1.Secret, error) {
	klog.Infof("get secret, ns:%s name:%s", constants.SystemNamespace, secretName)

	dynamicClient, err := clientregistry.GetCtrlDynamicClient(userName)

	if err != nil {
		return nil, err
	}

	unstructuredObj, err := dynamicClient.Resource(v1.SchemeGroupVersion.WithResource("secrets")).
		Namespace(constants.SystemNamespace).
		Get(context.Background(), secretName, metav1.GetOptions{})

	if err != nil {
		klog.Errorf("get secret failed, err: %v", err)
		return nil, err
	}

	secret := v1.Secret{}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, &secret)

	if err != nil {
		klog.Errorf("get secret failed, err: %v", err)
		return nil, err
	}

	return &secret, nil
}

func deleteSecret(userName string, secretName string) error {
	klog.Infof("delete secret, name: %s", secretName)
	dynamicClient, err := clientregistry.GetCtrlDynamicClient(userName)

	if err != nil {
		return err
	}

	err = dynamicClient.Resource(v1.SchemeGroupVersion.WithResource("secrets")).
		Namespace(constants.SystemNamespace).
		Delete(context.Background(), secretName, metav1.DeleteOptions{})

	return err
}

func generateSecretName(codeSourceName string) string {
	return "codesource-" + codeSourceName
}
