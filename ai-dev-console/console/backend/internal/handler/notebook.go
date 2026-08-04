package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type NotebookHandler struct {
	svc *service.NotebookService
}

func (h *NotebookHandler) List(c *gin.Context) {
	namespaces := getUserNamespaces(c)
	notebooks, err := h.svc.List(namespaces)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, notebooks)
}

func (h *NotebookHandler) Get(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	nb, err := h.svc.Get(name, namespace)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, nb)
}

func (h *NotebookHandler) Create(c *gin.Context) {
	var spec model.NotebookSpec
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
	response.OKMsg(c, "notebook created")
}

func (h *NotebookHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Delete(name, namespace, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "notebook deleted")
}

func (h *NotebookHandler) Stop(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Stop(name, namespace, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "notebook stopped")
}

func (h *NotebookHandler) Start(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Start(name, namespace, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "notebook started")
}

// Resize updates notebook resource limits (must be stopped).
func (h *NotebookHandler) Resize(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)

	var spec model.NotebookResizeSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		response.Failed(c, "invalid request: "+err.Error())
		return
	}

	if err := h.svc.Resize(name, namespace, userName, spec.CPU, spec.Memory, spec.GPU); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "notebook resized")
}

// SSHInfo returns connection info for SSH/terminal access.
func (h *NotebookHandler) SSHInfo(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")

	info, err := h.svc.GetSSHInfo(name, namespace)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, info)
}
