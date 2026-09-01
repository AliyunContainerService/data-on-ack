package handler

import (
	"strconv"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type TrainingHandler struct {
	svc *service.TrainingService
}

func (h *TrainingHandler) List(c *gin.Context) {
	namespaces := getUserNamespaces(c)
	kind := c.Query("kind") // optional filter: TFJob, PyTorchJob, MPIJob
	jobs, err := h.svc.List(namespaces, kind)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, jobs)
}

func (h *TrainingHandler) Get(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	kind := c.Query("kind")
	if kind == "" {
		kind = "PyTorchJob" // default
	}
	job, err := h.svc.Get(name, namespace, kind)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, job)
}

func (h *TrainingHandler) Create(c *gin.Context) {
	var spec model.TrainingJobSpec
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
	response.OKMsg(c, "training job created")
}

func (h *TrainingHandler) Delete(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	kind := c.Query("kind")
	if kind == "" {
		kind = "PyTorchJob"
	}
	userName := auth.GetCurrentUser(c)
	if err := h.svc.Delete(name, namespace, kind, userName); err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OKMsg(c, "training job deleted")
}

// Pods returns the list of pods belonging to a training job.
func (h *TrainingHandler) Pods(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	kind := c.Query("kind")
	if kind == "" {
		kind = "PyTorchJob"
	}
	pods, err := h.svc.ListPods(namespace, name, kind)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, pods)
}

// Events returns K8s events for a training job.
func (h *TrainingHandler) Events(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	events, err := h.svc.GetEvents(namespace, name)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, events)
}

// YAML returns the raw unstructured representation of a training job.
func (h *TrainingHandler) YAML(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	kind := c.Query("kind")
	if kind == "" {
		kind = "PyTorchJob"
	}
	raw, err := h.svc.GetRawYAML(name, namespace, kind)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, raw)
}

// Checkpoints lists checkpoint directories for a training job (from PVC output paths).
func (h *TrainingHandler) Checkpoints(c *gin.Context) {
	name := c.Param("name")
	namespace := c.Param("namespace")
	checkpoints, err := h.svc.ListCheckpoints(namespace, name)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, checkpoints)
}

// Logs returns log output for a specific pod of a training job.
func (h *TrainingHandler) Logs(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")
	pod := c.Query("pod")
	container := c.Query("container")
	tailStr := c.Query("tail")

	if pod == "" {
		response.Failed(c, "query parameter 'pod' is required")
		return
	}

	var tailLines int64 = 500 // default
	if tailStr != "" {
		if n, err := strconv.ParseInt(tailStr, 10, 64); err == nil && n > 0 {
			tailLines = n
		}
	}

	// The pod parameter must refer to a pod that actually belongs to the
	// addressed job; otherwise any user with namespace access could read
	// arbitrary pods' logs through this endpoint.
	if err := h.svc.EnsurePodBelongsToJob(namespace, name, pod); err != nil {
		response.FailedWithCode(c, 40300, "access denied: "+err.Error())
		return
	}
	logs, err := h.svc.GetPodLogs(namespace, pod, container, tailLines)
	if err != nil {
		response.Failed(c, err.Error())
		return
	}
	response.OK(c, logs)
}
