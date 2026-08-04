package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type QuotaHandler struct {
	quotaService *service.QuotaService
}

func newQuotaHandler(qSvc *service.QuotaService) *QuotaHandler {
	return &QuotaHandler{quotaService: qSvc}
}

func (h *QuotaHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/group/list", h.List)
	rg.POST("/group/create", h.Create)
	rg.PUT("/group/update", h.Update)
}

func (h *QuotaHandler) List(c *gin.Context) {
	trees, err := h.quotaService.ListQuotaTrees()
	if err != nil {
		response.Failed(c, response.CodeQuotaError, "list quota trees failed: "+err.Error())
		return
	}
	response.OK(c, trees)
}

func (h *QuotaHandler) Create(c *gin.Context) {
	var tree model.ElasticQuotaTree
	if err := c.ShouldBindJSON(&tree); err != nil {
		response.Failed(c, response.CodeQuotaError, "invalid request: "+err.Error())
		return
	}
	if err := h.quotaService.CreateQuotaTree(&tree); err != nil {
		response.Failed(c, response.CodeQuotaError, "create quota tree failed: "+err.Error())
		return
	}
	response.OKMsg(c, "quota tree created", tree)
}

func (h *QuotaHandler) Update(c *gin.Context) {
	action := c.Query("action")
	treeName := c.Query("treeName")
	oldNodeName := c.Query("oldNodeName")
	newNodeName := c.Query("newNodeName")
	prefix := c.Query("prefix")

	if treeName == "" {
		response.Failed(c, response.CodeQuotaError, "treeName is required")
		return
	}

	var node model.ElasticQuotaNode
	if err := c.ShouldBindJSON(&node); err != nil {
		response.Failed(c, response.CodeQuotaError, "invalid request body: "+err.Error())
		return
	}

	switch action {
	case "add":
		parentPath := prefix
		if oldNodeName != "" {
			parentPath = oldNodeName
		}
		if err := h.quotaService.AddNode(treeName, parentPath, &node); err != nil {
			response.Failed(c, response.CodeQuotaError, "add node failed: "+err.Error())
			return
		}
	case "delete":
		targetPath := oldNodeName
		if targetPath == "" {
			targetPath = node.Name
		}
		if err := h.quotaService.DeleteNode(treeName, targetPath); err != nil {
			response.Failed(c, response.CodeQuotaError, "delete node failed: "+err.Error())
			return
		}
	case "update":
		targetPath := oldNodeName
		if targetPath == "" {
			targetPath = node.Name
		}
		if newName := buildNewNodeName(newNodeName, prefix); newName != "" {
			node.Name = newName
		}
		if err := h.quotaService.UpdateNode(treeName, targetPath, &node); err != nil {
			response.Failed(c, response.CodeQuotaError, "update node failed: "+err.Error())
			return
		}
	default:
		response.Failed(c, response.CodeQuotaError, "unknown action: "+action)
		return
	}

	response.OKMsg(c, "quota tree updated", nil)
}

func buildNewNodeName(newNodeName, prefix string) string {
	if newNodeName == "" {
		return ""
	}
	if prefix != "" {
		return prefix + "." + newNodeName
	}
	return newNodeName
}
