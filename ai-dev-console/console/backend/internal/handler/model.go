package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ModelHandler struct {
	svc *service.ModelService
}

func (h *ModelHandler) List(c *gin.Context) {
	namespaces := getUserNamespaces(c)
	models, err := h.svc.List(namespaces)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, models)
}

func (h *ModelHandler) Register(c *gin.Context) {
	var spec model.ModelRegisterSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		response.Failed(c, "invalid request: "+err.Error())
		return
	}
	if !isNamespaceAllowed(c, spec.Namespace) {
		response.FailedWithCode(c, 40300, "access denied: namespace '"+spec.Namespace+"' is not in your allowed scope")
		return
	}
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Register(&spec, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "model registered")
}

func (h *ModelHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Delete(name, namespace, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "model deleted")
}
