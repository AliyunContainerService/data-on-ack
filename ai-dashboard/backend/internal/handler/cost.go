package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
)

// CostHandler provides cost dashboard APIs.
type CostHandler struct {
	costService *service.CostService
}

func newCostHandler(costService *service.CostService) *CostHandler {
	return &CostHandler{costService: costService}
}

func (h *CostHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ops/quotas", h.GetQuotaUsage)
	rg.GET("/ops/gpu-usage", h.GetGPUUsage)
	rg.PUT("/ops/quotas/update", h.UpdateQuotaNode)
}

// GetQuotaUsage returns the quota tree with current resource usage.
func (h *CostHandler) GetQuotaUsage(c *gin.Context) {
	usage, err := h.costService.GetQuotaUsage()
	if err != nil {
		response.Failed(c, response.CodeK8sError, "get quota usage: "+err.Error())
		return
	}
	response.OK(c, usage)
}

// GetGPUUsage returns GPU usage aggregated by user/team.
func (h *CostHandler) GetGPUUsage(c *gin.Context) {
	timeRange := c.DefaultQuery("range", "7d")
	summary, err := h.costService.GetGPUUsage(timeRange)
	if err != nil {
		response.Failed(c, response.CodeK8sError, "get gpu usage: "+err.Error())
		return
	}
	response.OK(c, summary)
}

// UpdateQuotaNode updates a quota node's min/max.
func (h *CostHandler) UpdateQuotaNode(c *gin.Context) {
	var req struct {
		NodeName string            `json:"nodeName"`
		Min      map[string]string `json:"min,omitempty"`
		Max      map[string]string `json:"max,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeK8sError, "invalid request: "+err.Error())
		return
	}
	if req.NodeName == "" {
		response.Failed(c, response.CodeK8sError, "nodeName is required")
		return
	}
	if err := h.costService.UpdateQuotaNode(req.NodeName, req.Min, req.Max); err != nil {
		response.Failed(c, response.CodeK8sError, "update quota: "+err.Error())
		return
	}
	response.OKMsg(c, "quota updated", nil)
}
