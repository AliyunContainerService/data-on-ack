package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type MetricsHandler struct {
	svc *service.MetricsService
}

// Metrics returns GPU metrics for a training job.
func (h *MetricsHandler) Metrics(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	metricName := c.DefaultQuery("metric", "gpu_util")
	kind := c.DefaultQuery("kind", "PyTorchJob")

	result, err := h.svc.GetTrainingJobMetrics(namespace, name, kind, metricName)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, result)
}

// AvailableMetrics returns the list of available GPU metric types.
func (h *MetricsHandler) AvailableMetrics(c *gin.Context) {
	metrics := h.svc.GetAvailableMetrics()
	response.OK(c, metrics)
}
