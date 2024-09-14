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
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/gin-gonic/gin"
	"k8s.io/klog"
	"strconv"
)

type ModelsAPIsController struct {
	modelsHandler *handlers.ModelsHandler
}

func NewModelsAPIscontroller(modelsHandler *handlers.ModelsHandler) *ModelsAPIsController {
	return &ModelsAPIsController{
		modelsHandler: modelsHandler,
	}
}

func (mc *ModelsAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	overview := routes.Group("/model")
	overview.POST("/create", mc.createModel)
	overview.GET("/list", mc.listModels)
	overview.GET("/get", mc.getModel)
	overview.DELETE("/delete", mc.deleteModel)
}

func (mc *ModelsAPIsController) createModel(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get data, error: %s", err.Error()))
		return
	}
	err = mc.modelsHandler.CreateModel(data)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get model, error: %s", err.Error()))
		return
	}
	utils.Succeed(c, nil)
}

func (mc *ModelsAPIsController) listModels(c *gin.Context) {
	var (
		curPageNum, curPageSize, curName, curVersion string
	)
	query := backends.ModelsQuery{}
	if curPageNum = c.Query("current_page"); curPageNum != "" {
		pageNum, err := strconv.Atoi(curPageNum)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to parse url parameter[current_page=%s], error: %s", curPageNum, err))
			return
		}
		if query.Pagination == nil {
			query.Pagination = &backends.QueryPagination{}
		}
		query.Pagination.PageNum = pageNum
	}
	if curPageSize = c.Query("page_size"); curPageSize != "" {
		pageSize, err := strconv.Atoi(curPageSize)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to parse url parameter[page_size=%s], error: %s", curPageSize, err))
			return
		}
		if query.Pagination == nil {
			query.Pagination = &backends.QueryPagination{}
		}
		query.Pagination.PageSize = pageSize
	}
	if curName = c.Query("model_name"); curName != "" {
		query.ModelName = curName
	}
	if curVersion = c.Query("model_version"); curVersion != "" {
		query.ModelVersion = curVersion
	}
	klog.Infof("get /model/list with parameters: pageNum = %s, pageSize = %s, modelName = %s, modelVersion = %s", curPageNum, curPageSize, curName, curVersion)
	models, err := mc.modelsHandler.GetModelsList(&query)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to list models from backend, error: %v", err))
		return
	}
	utils.Succeed(c, map[string]interface{}{
		"models": models,
		"total":  query.Pagination.Count,
	})
}

func (mc *ModelsAPIsController) getModel(c *gin.Context) {
	modelID := c.Query("model_id")
	klog.Infof("get /model/get with parameters: model_id=%s", modelID)
	model, err := mc.modelsHandler.GetModelDetails(modelID)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get model detail from backend, error: %v", err))
		return
	} else {
		utils.Succeed(c, model)
	}
}

func (mc *ModelsAPIsController) deleteModel(c *gin.Context) {
	modelID := c.Query("model_id")
	klog.Infof("delete /model/delete with parameters: model_id=%s", modelID)
	err := mc.modelsHandler.DeleteModel(modelID)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to delete model from backend, error: %v", err))
		return
	} else {
		utils.Succeed(c, nil)
	}
}
