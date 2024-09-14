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
	"bytes"
	"fmt"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/gin-contrib/sessions"
	"net/http"
	"sync"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

func NewLogsAPIsController(logHandler *handlers.LogHandler) *LogsAPIsController {
	return &LogsAPIsController{logHandler: logHandler}
}

type LogsAPIsController struct {
	logHandler *handlers.LogHandler
	lock       sync.Mutex
}

func (lc *LogsAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	logAPIs := routes.Group("/log")
	logAPIs.GET("/logs/:namespace/:jobKind/:jobName/:podName", lc.GetPodLogsOfJob)
	logAPIs.GET("/download/:namespace/:jobKind/:jobName/:podName", lc.DownloadPodLogsOfJob)

	eventAPIs := routes.Group("/event")
	eventAPIs.GET("/events/:namespace/:objName", lc.GetEvents)
}

func (lc *LogsAPIsController) GetPodLogsOfJob(c *gin.Context) {
	namespace := utils.Param(c, "namespace")
	jobKind := utils.Param(c, "jobKind")
	jobName := utils.Param(c, "jobName")
	podName := utils.Param(c, "podName")
	//uid := utils.Query(c, "uid")
	from := utils.Query(c, "fromTime")
	to := utils.Query(c, "toTime")

	// Assert critical parameters are not empty.
	if namespace == "" || podName == "" || from == "" {
		utils.Failed(c, "(namespace, podName, fromTime) should not be empty")
		return
	}

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	// Transform fromTime and toTime time values.
	fromTime, toTime, err := utils.TimeTransform(from, to)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to transform fromTime and toTime values, fromTime=%s, toTime=%s, error: %v", from, to, err))
		return
	}
	if toTime.IsZero() {
		toTime = time.Now()
	}

	logs, err := lc.logHandler.GetLogs(namespace, jobKind, jobName, podName, loginUserName, fromTime, toTime)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get logs of pod %s/%s, error: %v", namespace, podName, err))
	} else {
		utils.Succeed(c, logs)
	}
}

func (lc *LogsAPIsController) DownloadPodLogsOfJob(c *gin.Context) {
	namespace := utils.Param(c, "namespace")
	jobKind := utils.Param(c, "jobKind")
	jobName := utils.Param(c, "jobName")
	podName := utils.Param(c, "podName")
	//uid := utils.Query(c, "uid")
	from := utils.Query(c, "fromTime")
	to := utils.Query(c, "toTime")
	// Assert critical parameters are not empty.
	if namespace == "" || podName == "" || from == "" {
		utils.Failed(c, "(namespace, podName, fromTime) should not be empty")
		return
	}

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	// Transform fromTime and toTime time values.
	fromTime, toTime, err := utils.TimeTransform(from, to)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to transform fromTime and toTime values, fromTime=%s, toTime=%s, error: %v", from, to, err))
		return
	}
	if toTime.IsZero() {
		toTime = time.Now()
	}

	lc.lock.Lock()
	defer lc.lock.Unlock()

	// Get logs and response by a plain text-content file.
	logBytes, err := lc.logHandler.DownloadLogs(namespace, jobKind, jobName, podName, loginUserName, fromTime, toTime)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to download log content, pod: %s/%s, error: %v", namespace, podName, err))
		return
	} else {
		logCtx := bytes.NewReader(logBytes)
		contentLen := len(logBytes)
		contentType := "text/plain"
		extraHeaders := map[string]string{
			"Content-Disposition": fmt.Sprintf(`attachment; filename="%s_%s_%s_%d.log"`,
				namespace, podName, loginUserName, time.Now().Unix()),
		}
		c.DataFromReader(http.StatusOK, int64(contentLen), contentType, logCtx, extraHeaders)
	}
}

func (lc *LogsAPIsController) GetPodLogs(c *gin.Context) {

}

func (lc *LogsAPIsController) GetEvents(c *gin.Context) {
	namespace := utils.Param(c, "namespace")
	objName := utils.Param(c, "objName")
	//uid := utils.Query(c, "uid")
	from := utils.Query(c, "fromTime")
	to := utils.Query(c, "toTime")

	if namespace == "" || objName == "" || from == "" {
		utils.Failed(c, "(namespace, objName, fromTime) should not be empty")
		return
	}

	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	fromTime, toTime, err := utils.TimeTransform(from, to)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to transform fromTime and toTime values, fromTime=%s, toTime=%s, error: %v", from, to, err))
		return
	}
	if toTime.IsZero() {
		toTime = time.Now()
	}

	events, err := lc.logHandler.GetEvents(namespace, objName, loginUserName, fromTime, toTime)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to get job events, obj: %s/%s, error: %v", namespace, objName, err))
	} else {
		utils.Succeed(c, events)
	}
}
