package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type DatasetHandler struct {
	svc *service.DatasetService
}

func (h *DatasetHandler) List(c *gin.Context) {
	namespaces := getUserNamespaces(c)
	datasets, err := h.svc.List(namespaces)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, datasets)
}

func (h *DatasetHandler) Create(c *gin.Context) {
	var spec model.DatasetCreateSpec
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
	response.OKMsg(c, "dataset created")
}

func (h *DatasetHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Delete(name, namespace, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "dataset deleted")
}
