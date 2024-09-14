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
	"encoding/json"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"k8s.io/klog"
	"strconv"
	"time"
)

func NewEvaluateAPIsController(evaluateHandler *handlers.EvaluateHandler) *EvaluateAPIsController {
	return &EvaluateAPIsController{
		evaluateHandler: evaluateHandler,
	}
}

type EvaluateAPIsController struct {
	evaluateHandler *handlers.EvaluateHandler
}

func (dc *EvaluateAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	overview := routes.Group("/evaluate")
	overview.POST("/create", dc.createEvaluateJob)
	overview.GET("/list", dc.listEvaluateJobs)
	overview.GET("/get", dc.getEvaluateJob)
	overview.GET("/delete", dc.deleteEvaluateJob)
	overview.POST("/compare", dc.compareEvaluateJob)
}

func (dc *EvaluateAPIsController) createEvaluateJob(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get raw posted data from request"))
		return
	}
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	if err = dc.evaluateHandler.SubmitEvaluateJob(loginUserName, data); err != nil {
		handleErr(c, fmt.Sprintf("failed to submit job, error: %s", err))
		return
	}
	utils.Succeed(c, nil)
}

func (dc *EvaluateAPIsController) listEvaluateJobs(c *gin.Context) {
	var (
		curPageNum, curPageSize string
	)

	query := backends.EvaluateJobQuery{}

	session := sessions.Default(c)
	namespaces, ok := session.Get(auth.SessionKeyUserNS).([]string)
	if ok {
		query.AllocatedNamespaces = namespaces
	} else {
		handleErr(c, fmt.Sprintf("Please contact the administrator to allocate resource quotas."))
		return
	}

	if startTime := c.Query("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to parse start time[start_time=%s], error: %s", startTime, err))
			return
		}
		query.StartTime = t
	} else {
		handleErr(c, "start_time should not be empty")
		return
	}
	if endTime := c.Query("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to parse end time[end_time=%s], error: %s", endTime, err))
			return
		}
		query.EndTime = t
	} else {
		query.EndTime = time.Now()
	}
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
	klog.Infof("get /evaluate/list with parameters: pageNum=%s, pageSize=%s",
		curPageNum, curPageSize)
	EvaluateJobInfos, err := dc.evaluateHandler.ListEvaluateJobsFromBackend(&query)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to list evaluateJobs from backend, error: %v", err))
		return
	}
	utils.Succeed(c, map[string]interface{}{
		"evaluateJobInfos": EvaluateJobInfos,
		"total":            query.Pagination.Count,
	})
}

func (dc *EvaluateAPIsController) getEvaluateJob(c *gin.Context) {
	jobID := c.Query("id")
	evaluateJobInfo, err := dc.evaluateHandler.GetEvaluateJobData("", "", jobID)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get evaluateJob, error: %s", err))
		return
	} else {
		utils.Succeed(c, *evaluateJobInfo)
	}
}

func (dc *EvaluateAPIsController) deleteEvaluateJob(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Query("name")
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	klog.Infof("post /evaluate/delete with parameters: namespace=%s, name=%s", namespace, name)
	err := dc.evaluateHandler.DeleteEvaluateJobFromCluster(loginUserName, namespace, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to delete evaluateJob, error: %s", err))
	} else {
		utils.Succeed(c, nil)
	}
}

func (dc *EvaluateAPIsController) compareEvaluateJob(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		klog.Infof("EvaluateJob data err : %s", err.Error())
		utils.Failed(c, err.Error())
		return
	}
	array := handlers.SearchArray{}
	err = json.Unmarshal(data, &array)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to unmarshal evaluateJob, error: %s", err))
	}
	if len(array) < 2 {
		handleErr(c, fmt.Sprintf("Invalid evaluateJob data"))
	}
	metricsArray, err := dc.evaluateHandler.CompareEvaluateJobs(array)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to compare evaluateJobs, error: %s", err))
		return
	}
	utils.Succeed(c, metricsArray)
}
