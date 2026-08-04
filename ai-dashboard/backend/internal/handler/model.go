package handler

import (
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
)

// ModelAdminHandler provides model version and lineage APIs for the admin dashboard.
type ModelAdminHandler struct {
	modelService *service.ModelAdminService
}

func newModelAdminHandler(modelService *service.ModelAdminService) *ModelAdminHandler {
	return &ModelAdminHandler{modelService: modelService}
}

func (h *ModelAdminHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ops/models", h.ListModels)
	rg.GET("/ops/models/:name/versions", h.GetVersions)
	rg.POST("/ops/models/:name/versions", h.CreateVersion)
	rg.GET("/ops/models/:name/lineage", h.GetLineage)
}

// ListModels returns all registered models with version counts.
func (h *ModelAdminHandler) ListModels(c *gin.Context) {
	models, err := h.modelService.ListModels()
	if err != nil {
		response.Failed(c, response.CodeK8sError, "list models: "+err.Error())
		return
	}
	response.OK(c, models)
}

// GetVersions returns all versions of a specific model.
func (h *ModelAdminHandler) GetVersions(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "default")

	versions, err := h.modelService.GetModelVersions(name, namespace)
	if err != nil {
		response.Failed(c, response.CodeK8sError, "get versions: "+err.Error())
		return
	}
	response.OK(c, versions)
}

// CreateVersion adds a new version to a model.
func (h *ModelAdminHandler) CreateVersion(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "default")

	var req struct {
		Version     string             `json:"version"`
		TrainedFrom string             `json:"trainedFrom,omitempty"`
		DatasetRef  string             `json:"datasetRef,omitempty"`
		Metrics     map[string]float64 `json:"metrics,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeK8sError, "invalid request: "+err.Error())
		return
	}
	if req.Version == "" {
		response.Failed(c, response.CodeK8sError, "version is required")
		return
	}

	version := &service.ModelVersion{
		Version:     req.Version,
		TrainedFrom: req.TrainedFrom,
		DatasetRef:  req.DatasetRef,
		Metrics:     req.Metrics,
		CreatedAt:   time.Now().Format(time.RFC3339),
		Status:      "ready",
	}

	if err := h.modelService.CreateModelVersion(name, namespace, version); err != nil {
		response.Failed(c, response.CodeK8sError, "create version: "+err.Error())
		return
	}
	response.OKMsg(c, "version created", nil)
}

// GetLineage returns the lineage DAG for a model.
func (h *ModelAdminHandler) GetLineage(c *gin.Context) {
	name := c.Param("name")
	namespace := c.DefaultQuery("namespace", "default")

	lineage, err := h.modelService.GetModelLineage(name, namespace)
	if err != nil {
		response.Failed(c, response.CodeK8sError, "get lineage: "+err.Error())
		return
	}
	response.OK(c, lineage)
}
