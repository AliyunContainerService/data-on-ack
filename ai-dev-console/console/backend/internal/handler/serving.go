package handler

import (
	"strconv"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ServingHandler struct {
	svc *service.ServingService
}

func (h *ServingHandler) List(c *gin.Context) {
	namespaces := getUserNamespaces(c)
	jobs, err := h.svc.List(namespaces)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, jobs)
}

func (h *ServingHandler) Get(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	job, err := h.svc.Get(name, namespace)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, job)
}

func (h *ServingHandler) Create(c *gin.Context) {
	var spec model.ServingSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		response.Failed(c, "invalid request: "+err.Error())
		return
	}
	if !isNamespaceAllowed(c, spec.Namespace) {
		response.FailedWithCode(c, 40300, "access denied: namespace '"+spec.Namespace+"' is not in your allowed scope")
		return
	}
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Create(&spec, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "serving deployment created")
}

func (h *ServingHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Delete(name, namespace, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "serving deployment deleted")
}

// Test proxies a chat/completion request to the inference endpoint.
func (h *ServingHandler) Test(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")

	var reqBody map[string]interface{}
	if err := c.ShouldBindJSON(&reqBody); err != nil {
		response.Failed(c, "invalid request body: "+err.Error())
		return
	}

	result, err := h.svc.TestEndpoint(namespace, name, reqBody)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, result)
}

// Pods returns pods belonging to a serving deployment.
func (h *ServingHandler) Pods(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	pods, err := h.svc.ListServingPods(namespace, name)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, pods)
}

// Logs returns log output for a serving pod.
func (h *ServingHandler) Logs(c *gin.Context) {
	namespace := c.Param("namespace")
	_ = c.Param("name") // for route consistency
	pod := c.Query("pod")
	container := c.Query("container")
	tailStr := c.Query("tail")

	if pod == "" {
		response.Failed(c, "query parameter 'pod' is required")
		return
	}

	var tailLines int64 = 500
	if tailStr != "" {
		if n, err := strconv.ParseInt(tailStr, 10, 64); err == nil && n > 0 {
			tailLines = n
		}
	}

	logs, err := h.svc.GetServingPodLogs(namespace, pod, container, tailLines)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, logs)
}
