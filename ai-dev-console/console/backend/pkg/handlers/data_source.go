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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DatasourceConfigMapName = "kubedl-datasource-config"
	DatasourceConfigMapKey  = "datasource"
)

func NewDataSourceHandler() *DataSourceHandler {
	return &DataSourceHandler{client: clientmgr.GetCtrlClient()}
}

type DataSourceHandler struct {
	client client.Client
}

// post
func (ov *DataSourceHandler) PostDataSourceToConfigMap(userName string, dataSource model.DataSource) error {
	klog.Infof("DataSource : %s", dataSource)

	configMap, err := getOrCreateDataSourceConfigMap(userName)
	if err != nil {
		return err
	}

	dataSourceMap, err := getDataSourceMap(configMap)
	if err != nil {
		return err
	}

	_, exists := dataSourceMap[dataSource.Name]
	if exists {
		klog.Errorf("DataSource exists, name: %s", dataSource.Name)
		return fmt.Errorf("DataSource exists, name: %s", dataSource.Name)
	}

	dataSourceMap[dataSource.Name] = dataSource

	return setDataSourceConfigMap(userName, configMap, dataSourceMap)
}

// delete
func (ov *DataSourceHandler) DeleteDataSourceFromConfigMap(userName string, name string) error {
	if len(name) == 0 {
		return fmt.Errorf("name is empty")
	}

	configMap, err := getOrCreateDataSourceConfigMap(userName)
	if err != nil {
		return err
	}

	dataSourceMap, err := getDataSourceMap(configMap)
	if err != nil {
		return err
	}

	_, exists := dataSourceMap[name]
	if !exists {
		klog.Errorf("DataSource not exists, name: %s", name)
		return fmt.Errorf("DataSource not exists, name: %s", name)
	}

	delete(dataSourceMap, name)

	return setDataSourceConfigMap(userName, configMap, dataSourceMap)

}

// put
func (ov *DataSourceHandler) PutDataSourceToConfigMap(userName string, dataSource model.DataSource) error {
	configMap, err := getOrCreateDataSourceConfigMap(userName)
	if err != nil {
		return err
	}

	dataSourceMap, err := getDataSourceMap(configMap)
	if err != nil {
		return err
	}

	dataSource.CreateTime = dataSourceMap[dataSource.Name].CreateTime

	dataSourceMap[dataSource.Name] = dataSource

	rs := setDataSourceConfigMap(userName, configMap, dataSourceMap)

	return rs
}

// get
func (ov *DataSourceHandler) GetDataSourceFromConfigMap(userName string, name string) (model.DataSource, error) {
	if len(name) == 0 {
		return model.DataSource{}, fmt.Errorf("name is empty")
	}

	configMap, err := getOrCreateDataSourceConfigMap(userName)
	if err != nil {
		klog.Errorf("getOrCreateDataSourceConfigMap failed, err: %v", err)
		return model.DataSource{}, err
	}

	dataSourceMap, err := getDataSourceMap(configMap)
	if err != nil {
		klog.Errorf("getDataSourceMap failed, err: %v", err)
		return model.DataSource{}, err
	}

	dataSource, exists := dataSourceMap[name]
	if !exists {
		klog.Errorf("DataSource not exists, userID: %s", name)
		return model.DataSource{}, fmt.Errorf("DataSource not exists, userID: %s", name)
	}

	return dataSource, nil
}

// get all
func (ov *DataSourceHandler) ListDataSourceFromConfigMap(userName string) (model.DataSourceMap, error) {
	configMap, err := getOrCreateDataSourceConfigMap(userName)
	if err != nil {
		klog.Errorf("getOrCreateDataSourceConfigMap failed, err: %v", err)
		return model.DataSourceMap{}, err
	}

	dataSourceMap, err := getDataSourceMap(configMap)
	if err != nil {
		klog.Errorf("getDataSourceMap failed, err: %v", err)
		return model.DataSourceMap{}, err
	}

	return dataSourceMap, nil
}

// set
func setDataSourceConfigMap(userName string, configMap *v1.ConfigMap, dataSourceMap model.DataSourceMap) error {
	if configMap == nil {
		klog.Errorf("ConfigMap is nil")
		return fmt.Errorf("ConfigMap is nil")
	}

	dataSourceMapBytes, err := json.Marshal(dataSourceMap)
	if err != nil {
		klog.Errorf("DataSourceMap Marshal failed, err: %v", err)
	}

	configMap.Data[DatasourceConfigMapKey] = string(dataSourceMapBytes)

	ctrlClient := clientmgr.GetCtrlClient()

	//ctrlClient, err := clientregistry.GetCtrlClient(userName)
	//if err != nil {
	//	return err
	//}

	return ctrlClient.Update(context.TODO(), configMap)
}

func getOrCreateDataSourceConfigMap(userName string) (*v1.ConfigMap, error) {
	ctrlClient := clientmgr.GetCtrlClient()

	//if err != nil {
	//	return nil, err
	//}

	configMap := &v1.ConfigMap{}
	err := ctrlClient.Get(context.TODO(),
		apitypes.NamespacedName{
			Namespace: constants.SystemNamespace,
			Name:      DatasourceConfigMapName,
		}, configMap)

	// Create initial user info ConfigMap if not exists
	if errors.IsNotFound(err) {
		initConfigMap := &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.SystemNamespace,
				Name:      DatasourceConfigMapName,
			},
			Data: map[string]string{
				DatasourceConfigMapKey: "{}",
			},
		}

		err = ctrlClient.Create(context.TODO(), initConfigMap)
		if err != nil {
			klog.Errorf("create configmap failed, err:%v", err)
			return nil, err
		}

		return initConfigMap, nil
	} else if err != nil {
		klog.Errorf("Failed to get ConfigMap, ns: %s, name: %s, err: %v", constants.SystemNamespace, DatasourceConfigMapName, err)
		return configMap, err
	}
	return configMap, nil
}

func getDataSourceMap(configMap *v1.ConfigMap) (model.DataSourceMap, error) {
	if configMap == nil {
		klog.Errorf("ConfigMap is nil")
		return model.DataSourceMap{}, fmt.Errorf("ConfigMap is nil")
	}

	datasources, exists := configMap.Data[DatasourceConfigMapKey]
	if !exists {
		klog.Errorf("ConfigMap key `%s` not exists", DatasourceConfigMapKey)
		return model.DataSourceMap{}, fmt.Errorf("ConfigMap key `%s` not exists", DatasourceConfigMapKey)
	}
	if len(datasources) == 0 {
		klog.Warningf("DataSources is empty")
		return model.DataSourceMap{}, nil
	}

	dataSourceMap := model.DataSourceMap{}
	err := json.Unmarshal([]byte(datasources), &dataSourceMap)
	if err != nil {
		klog.Errorf("ConfigMap json Unmarshal error, content: %s, err: %v", datasources, err)
		return dataSourceMap, err
	}

	return dataSourceMap, nil
}
