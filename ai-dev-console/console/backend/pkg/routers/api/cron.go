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
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"k8s.io/klog"
	"strconv"
	"time"
)

type CronAPIsController struct {
	cronHandler *handlers.CronHandler
}

func NewCronAPIsController(cronHandler *handlers.CronHandler) *CronAPIsController {
	return &CronAPIsController{cronHandler: cronHandler}
}

func (cj *CronAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	cronJobAPI := routes.Group("/cron")
	cronJobAPI.GET("/list", cj.ListCron)
	cronJobAPI.GET("/get/:namespace/:name", cj.GetCron)
	cronJobAPI.GET("/history/:namespace/:name", cj.ListCronHistory)
	cronJobAPI.POST("/resume/:namespace/:name", cj.ResumeCron)
	cronJobAPI.POST("/suspend/:namespace/:name", cj.SuspendCron)
	cronJobAPI.DELETE("/:namespace/:name", cj.DeleteCron)
}

func (cj *CronAPIsController) ListCron(c *gin.Context) {
	var (
		kind, ns, name, status, curPageNum, curPageSize string
	)

	query := backends.CronQuery{}

	deleted := 0
	query.Deleted = &deleted

	session := sessions.Default(c)
	namespaces, ok := session.Get(auth.SessionKeyUserNS).([]string)
	if ok {
		query.AllocatedNamespaces = namespaces
	} else {
		handleErr(c, fmt.Sprintf("Please contact the administrator to allocate resource quotas."))
		return
	}

	uid, ok := session.Get(auth.SessionKeyLoginID).(string)
	if ok {
		query.UID = uid
	}
	loginUserName, ok := session.Get(auth.SessionKeyLoginName).(string)
	if ok {
		query.UserName = loginUserName
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
	if kind = c.Query("kind"); kind != "" {
		query.Type = kind
	}
	if ns = c.Query("namespace"); ns != "" {
		query.Namespace = ns
	}
	if name = c.Query("name"); name != "" {
		query.Name = name
	}
	if status = c.Query("status"); status != "" {
		query.Status = status
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
	klog.Infof("get /cron/list with parameters: kind=%s, namespace=%s, name=%s, status=%s, pageNum=%s, pageSize=%s",
		kind, ns, name, status, curPageNum, curPageSize)

	cronInfos, err := cj.cronHandler.ListCron(&query)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to list crons from backend, error: %v", err))
		return
	}
	utils.Succeed(c, map[string]interface{}{
		"cronInfos": cronInfos,
		"total":     query.Pagination.Count,
	})
}

func (cj *CronAPIsController) ListCronHistory(c *gin.Context) {
	namespace := c.Query("namespace")
	name := c.Query("name")
	jobName := c.Query("job_name")
	jobStatus := c.Query("job_status")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	cronHistoryInfos, err := cj.cronHandler.ListCronHistory(loginUserName, namespace, name, jobName, jobStatus)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to list cron history from backend, error: %v", err))
		return
	}

	utils.Succeed(c, map[string]interface{}{
		"cronHistories": cronHistoryInfos,
		"total":         len(cronHistoryInfos),
	})
}

func (cj *CronAPIsController) GetCron(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	cronInfo, err := cj.cronHandler.GetCron(loginUserName, namespace, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get cron info from backend, error: %v", err))
		return
	}

	utils.Succeed(c, map[string]interface{}{
		"cronInfo": cronInfo,
	})
}

func (cj *CronAPIsController) SuspendCron(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	err := cj.cronHandler.SuspendCron(loginUserName, namespace, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to suspend cron from backend, error: %v", err))
		return
	}

	utils.Succeed(c, nil)
}

func (cj *CronAPIsController) ResumeCron(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	err := cj.cronHandler.ResumeCron(loginUserName, namespace, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to resume cron from backend, error: %v", err))
		return
	}

	utils.Succeed(c, nil)
}

func (cj *CronAPIsController) DeleteCron(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	klog.Infof("delete cron, namespace: %s name: %s", namespace, name)

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	err := cj.cronHandler.StopCron(loginUserName, namespace, name)
	//err := cj.cronHandler.DeleteCron(loginName, namespace, name)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to delete cron from backend, error: %v", err))
		return
	}

	utils.Succeed(c, nil)
}
