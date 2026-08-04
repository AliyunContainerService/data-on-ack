package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type ExperimentHandler struct {
	svc *service.ExperimentService
}

// List returns all experiments in user-accessible namespaces.
func (h *ExperimentHandler) List(c *gin.Context) {
	namespaces := getUserNamespaces(c)
	experiments, err := h.svc.List(namespaces)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, experiments)
}

// Get returns a single experiment with all its runs.
func (h *ExperimentHandler) Get(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}
	if !isNamespaceAllowed(c, namespace) {
		response.FailedWithCode(c, 40300, "access denied: namespace '"+namespace+"' is not in your allowed scope")
		return
	}
	experiment, err := h.svc.Get(name, namespace)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, experiment)
}

// Create creates a new experiment.
func (h *ExperimentHandler) Create(c *gin.Context) {
	var spec model.ExperimentCreateSpec
	if err := c.ShouldBindJSON(&spec); err != nil {
		response.Failed(c, "invalid request: "+err.Error())
		return
	}
	if spec.Name == "" {
		response.Failed(c, "experiment name is required")
		return
	}
	if spec.Namespace == "" {
		spec.Namespace = "default"
	}
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Create(&spec, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "experiment created")
}

// AddRun associates a training job with an experiment.
func (h *ExperimentHandler) AddRun(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}
	if !isNamespaceAllowed(c, namespace) {
		response.FailedWithCode(c, 40300, "access denied: namespace '"+namespace+"' is not in your allowed scope")
		return
	}

	var run model.ExperimentRunSpec
	if err := c.ShouldBindJSON(&run); err != nil {
		response.Failed(c, "invalid request: "+err.Error())
		return
	}
	if run.JobName == "" {
		response.Failed(c, "jobName is required")
		return
	}

	if err := h.svc.AddRun(name, namespace, &run); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "run added to experiment")
}

// Delete removes an experiment.
func (h *ExperimentHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}
	if !isNamespaceAllowed(c, namespace) {
		response.FailedWithCode(c, 40300, "access denied: namespace '"+namespace+"' is not in your allowed scope")
		return
	}
	if err := h.svc.Delete(name, namespace); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "experiment deleted")
}
