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
    
package api

import (
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/gin-contrib/sessions"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

func NewDataSourceAPIsController(dataSourceHandler *handlers.DataSourceHandler) *DataSourceAPIsController {
	return &DataSourceAPIsController{
		dataSourceHandler: dataSourceHandler,
	}
}

type DataSourceAPIsController struct {
	dataSourceHandler *handlers.DataSourceHandler
}

func (dc *DataSourceAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	overview := routes.Group("/datasource")
	overview.POST("", dc.postDataSource)
	overview.DELETE("/:name", dc.deleteDataSource)
	overview.PUT("", dc.putDataSource)
	overview.GET("", dc.getDataSource)
	overview.GET("/:name", dc.getDataSource)
}

func (dc *DataSourceAPIsController) postDataSource(c *gin.Context) {
	userid := c.PostForm("userid")
	username := c.PostForm("username")
	namespace := c.PostForm("namespace")
	name := c.PostForm("name")
	_type := c.PostForm("type")
	pvcName := c.PostForm("pvc_name")
	localPath := c.PostForm("local_path")
	description := c.PostForm("description")
	createTime := time.Now().Format("2006-01-02 15:04:05")
	updateTime := time.Now().Format("2006-01-02 15:04:05")

	datasource := model.DataSource{
		UserId:      userid,
		Username:    username,
		Namespace:   namespace,
		Name:        name,
		Type:        _type,
		PvcName:     pvcName,
		LocalPath:   localPath,
		Description: description,
		CreateTime:  createTime,
		UpdateTime:  updateTime,
	}

	err := dc.dataSourceHandler.PostDataSourceToConfigMap(username, datasource)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to create data, error: %v", err))
		return
	}
	utils.Succeed(c, "success to create data")
}

func (dc *DataSourceAPIsController) deleteDataSource(c *gin.Context) {
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	name := c.Param("name")
	err := dc.dataSourceHandler.DeleteDataSourceFromConfigMap(loginUserName, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to delete data, error: %v", err))
		return
	}
	utils.Succeed(c, "success to delete data.")
}

func (dc *DataSourceAPIsController) putDataSource(c *gin.Context) {
	userId := c.PostForm("userid")
	username := c.PostForm("username")
	namespace := c.PostForm("namespace")
	name := c.PostForm("name")
	_type := c.PostForm("type")
	pvcName := c.PostForm("pvc_name")
	localPath := c.PostForm("local_path")
	description := c.PostForm("description")
	updateTime := time.Now().Format("2006-01-02 15:04:05")

	datasource := model.DataSource{
		UserId:      userId,
		Username:    username,
		Namespace:   namespace,
		Name:        name,
		Type:        _type,
		PvcName:     pvcName,
		LocalPath:   localPath,
		Description: description,
		UpdateTime:  updateTime,
	}

	err := dc.dataSourceHandler.PutDataSourceToConfigMap(username, datasource)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to update data, err=%v", err))
		return
	}
	utils.Succeed(c, "success to update data")
}

func (dc *DataSourceAPIsController) getDataSource(c *gin.Context) {
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	name := c.Param("name")
	if name == "" {
		dataSources, err := dc.dataSourceHandler.ListDataSourceFromConfigMap(loginUserName)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to get data, error: %v", err))
			return
		}
		for k, v := range dataSources {
			if v.Username != loginUserName {
				delete(dataSources, k)
			}
		}
		utils.Succeed(c, dataSources)
	} else {
		dataSources, err := dc.dataSourceHandler.GetDataSourceFromConfigMap(loginUserName, name)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to get data, error: %v", err))
			return
		}
		utils.Succeed(c, dataSources)
	}

}
