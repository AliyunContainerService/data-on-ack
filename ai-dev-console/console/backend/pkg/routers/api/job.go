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
	"strconv"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends"
	v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"k8s.io/klog"
)

func NewJobAPIsController(jobHandler *handlers.JobHandler) *JobAPIsController {
	return &JobAPIsController{jobHandler: jobHandler}
}

type JobAPIsController struct {
	jobHandler *handlers.JobHandler
}

func (jc *JobAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	jobAPI := routes.Group("/job")
	jobAPI.GET("/list", jc.ListJobs)
	jobAPI.GET("/detail", jc.GetJobDetail)
	jobAPI.GET("/yaml/:namespace/:name", jc.GetJobYamlData)
	jobAPI.GET("/json/:namespace/:name", jc.GetJobJsonData)
	jobAPI.POST("/stop", jc.StopJob)
	jobAPI.POST("/submit", jc.SubmitJob)
	jobAPI.DELETE("/:namespace/:name", jc.DeleteJob)
	jobAPI.GET("/statistics", jc.GetJobStatistics)
	jobAPI.GET("/running-jobs", jc.GetRunningJobs)

	pvcAPIs := routes.Group("/pvc")
	pvcAPIs.GET("/list", jc.ListPVC)
}

func (jc *JobAPIsController) ListJobs(c *gin.Context) {
	var (
		kind, ns, name, status, curPageNum, curPageSize string
	)

	query := backends.Query{}

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
		query.Status = v1.JobConditionType(status)
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
	klog.Infof("get /job/list with parameters: kind=%s, namespace=%s, name=%s, status=%s, pageNum=%s, pageSize=%s",
		kind, ns, name, status, curPageNum, curPageSize)
	jobInfos, err := jc.jobHandler.ListJobsFromBackend(&query)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to list jobs from backend, error: %v", err))
		return
	}
	utils.Succeed(c, map[string]interface{}{
		"jobInfos": jobInfos,
		"total":    query.Pagination.Count,
	})
}

func (jc *JobAPIsController) GetJobDetail(c *gin.Context) {
	var (
		jobID, deployRegion, namespace, jobName, kind string
	)

	jobID = c.Query("id")
	jobName = c.Query("job_name")
	deployRegion = c.Query("deploy_region")
	namespace = c.Query("namespace")
	kind = c.Query("kind")

	if kind == "" {
		handleErr(c, fmt.Sprintf("job kind must not be empty"))
		return
	}

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	currentPage, err := strconv.Atoi(c.Query("current_page"))
	if err != nil {
		handleErr(c, fmt.Sprintf("invalid current page [%s], error: %v", c.Query("current_page"), err))
		return
	}
	if currentPage <= 0 {
		currentPage = 1
	}

	pageSize, err := strconv.Atoi(c.Query("page_size"))
	if err != nil {
		handleErr(c, fmt.Sprintf("invalid page size [%s], error: %v", c.Query("page_size"), err))
		return
	}

	klog.Infof("get /job/detail with parameters: kind=%s, namespace=%s, name=%s, id=%s, currentPage=%d, deployRegion=%s",
		kind, namespace, jobName, jobID, currentPage, deployRegion)
	jobInfo, err := jc.jobHandler.GetDetailedJobFromBackend(loginUserName, namespace, jobName, jobID, kind, deployRegion)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get detailed job from backend, namespace=%s, name=%s, id=%s, kind=%s, error: %v",
			namespace, jobName, jobID, kind, err))
		return
	}

	replicaType := c.Query("replica_type")
	if replicaType == "" {
		replicaType = "ALL"
	}
	status := c.Query("status")
	if status == "" {
		status = "ALL"
	}

	// Filter jobs by replica type and job status
	jobInfo.Specs = jobFilter(replicaType, status, jobInfo.Specs)
	originTotal := len(jobInfo.Specs)
	startIdx := (currentPage - 1) * pageSize
	if startIdx > len(jobInfo.Specs) {
		utils.Failed(c, "current page out of index")
		return
	}
	endIdx := currentPage * pageSize
	if endIdx > len(jobInfo.Specs) {
		endIdx = len(jobInfo.Specs)
	}
	jobInfo.Specs = jobInfo.Specs[startIdx:endIdx]
	utils.Succeed(c, map[string]interface{}{
		"jobInfo": jobInfo,
		"total":   originTotal,
	})
}

func (jc *JobAPIsController) StopJob(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	region := c.Param("deployRegion")
	kind := c.Query("kind")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	klog.Infof("post /job/stop with parameters: kind=%s, namespace=%s, name=%s", region, namespace, name)
	err := jc.jobHandler.StopJob(loginUserName, namespace, name, "", kind, region)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to stop job, error: %s", err))
	} else {
		utils.Succeed(c, nil)
	}
}

func (jc *JobAPIsController) DeleteJob(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	id := c.Query("id")
	kind := c.Query("kind")
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	klog.Infof("post /job/delete with parameters: kind=%s, namespace=%s, name=%s", kind, namespace, name)
	err := jc.jobHandler.StopJob(loginUserName, namespace, name, id, kind, "")
	//err := jc.jobHandler.DeleteJobFromStorage(uid, namespace, name, id, kind, "")
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to delete job, error: %s", err))
	} else {
		utils.Succeed(c, nil)
	}
}

func (jc *JobAPIsController) ListPVC(c *gin.Context) {
	namespace := c.Query("namespace")
	if pvc, err := jc.jobHandler.ListPVC(namespace); err != nil {
		handleErr(c, fmt.Sprintf("failed to list pvc, error: %v", err))
		return
	} else {
		utils.Succeed(c, pvc)
	}
}

func (jc *JobAPIsController) SubmitJob(c *gin.Context) {
	kind := c.Query("kind")
	if kind == "" {
		handleErr(c, fmt.Sprintf("job kind is empty"))
		return
	}
	data, err := c.GetRawData()
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get raw posted data from request"))
		return
	}

	job := model.SubmitJobArgs{}
	err = json.Unmarshal(data, &job)
	if err != nil {
		handleErr(c, fmt.Sprintf("job args is invalid"))
		return
	}

	jobInfo := job.SubmitJobInfo

	session := sessions.Default(c)
	uid, ok := session.Get(auth.SessionKeyLoginID).(string)
	if ok {
		if jobInfo.Labels == nil {
			labels := make(map[string]string)
			labels["arena.kubeflow.org/console-user"] = uid
			jobInfo.Labels = labels
		} else {
			jobInfo.Labels["arena.kubeflow.org/console-user"] = uid
		}
	}

	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	if err = jc.jobHandler.SubmitJob(loginUserName, &jobInfo); err != nil {
		handleErr(c, fmt.Sprintf("failed to submit job, error: %s", err))
		return
	}
	utils.Succeed(c, nil)
}

func (jc *JobAPIsController) GetJobYamlData(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	kind := c.Query("kind")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	data, err := jc.jobHandler.GetJobYamlData(loginUserName, namespace, name, kind)
	if err != nil {
		handleErr(c, err.Error())
		return
	}
	utils.Succeed(c, string(data))
}

func (jc *JobAPIsController) GetJobJsonData(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	kind := c.Query("kind")

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	data, err := jc.jobHandler.GetJobJsonData(loginUserName, namespace, name, kind)
	if err != nil {
		handleErr(c, err.Error())
		return
	}

	utils.Succeed(c, string(data))
}

func (jc *JobAPIsController) GetJobStatistics(c *gin.Context) {
	query := backends.Query{}

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

	deleted := 0
	query.Deleted = &deleted

	klog.Infof("get /job/statistics with parameters: start_time:%s, end_time:%s",
		query.StartTime.Format(time.RFC3339), query.EndTime.Format(time.RFC3339))
	jobStatistics, err := jc.jobHandler.GetJobStatisticsFromBackend(&query)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get jobs statistics from backend, error: %v", err))
		return
	}
	utils.Succeed(c, map[string]interface{}{
		"jobStatistics": jobStatistics,
	})
}

func (jc *JobAPIsController) GetRunningJobs(c *gin.Context) {
	query := backends.Query{}

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

	query.Status = v1.JobRunning

	limit := int(-1)
	if limitPara := c.Query("limit"); limitPara != "" {
		var err error
		limit, err = strconv.Atoi(limitPara)
		if err != nil {
			handleErr(c, fmt.Sprintf("failed to parse limit[limit=%s], error: %s", limitPara, err))
			return
		}
	}

	deleted := 0
	query.Deleted = &deleted

	klog.Infof("get /job/running-jobs with parameters: start_time:%s, end_time:%s",
		query.StartTime.Format(time.RFC3339), query.EndTime.Format(time.RFC3339))
	runningJobs, err := jc.jobHandler.GetRunningJobsFromBackend(&query)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get running jobs from backend, error: %v", err))
		return
	}

	if limit != -1 && limit <= len(runningJobs) {
		runningJobs = runningJobs[0:limit]
	}
	utils.Succeed(c, map[string]interface{}{
		"runningJobs": runningJobs,
	})
}

func handleErr(c *gin.Context, msg string) {
	formattedMsg := msg
	session := sessions.Default(c)
	uid := session.Get(auth.SessionKeyLoginID)
	if uid != nil && uid.(string) != "" {
		formattedMsg = fmt.Sprintf("Error: %s, uid: %s", msg, uid.(string))
	}
	klog.Error(formattedMsg)
	utils.Failed(c, msg)
}

func jobFilter(replica, status string, jobs []model.Spec) []model.Spec {
	if replica == "ALL" && status == "ALL" {
		return jobs
	}
	filtered := make([]model.Spec, 0)
	for i := range jobs {
		if (replica == "ALL" || replica == strings.ToUpper(jobs[i].ReplicaType)) &&
			(status == "ALL" || status == strings.ToUpper(string(jobs[i].Status))) {
			filtered = append(filtered, jobs[i])
		}
	}
	return filtered
}
