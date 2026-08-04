package handler

import (
	"context"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DatasetHandler struct {
	datasetService *service.DatasetService
	kubeClient     *k8s.Client
}

func newDatasetHandler(dsSvc *service.DatasetService, kubeClient *k8s.Client) *DatasetHandler {
	return &DatasetHandler{datasetService: dsSvc, kubeClient: kubeClient}
}

func (h *DatasetHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/dataset/list", h.List)
	rg.POST("/dataset/create", h.Create)
	rg.PUT("/dataset/delete", h.Delete)
	rg.POST("/dataset/update", h.Update)
}

type DatasetItem struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Type       string `json:"type"`   // "pvc" or "fluid"
	Status     string `json:"status"`
	Capacity   string `json:"capacity"`
	AccessMode string `json:"accessMode"`
	StorageClass string `json:"storageClass"`
}

func (h *DatasetHandler) List(c *gin.Context) {
	namespace := c.Query("namespace")

	var items []DatasetItem

	// 1. List all PVCs (these are the primary "datasets" in a K8s AI platform)
	pvcs, err := h.kubeClient.Typed().CoreV1().PersistentVolumeClaims(namespace).List(context.TODO(), metav1.ListOptions{})
	if err == nil {
		for _, pvc := range pvcs.Items {
			accessMode := ""
			if len(pvc.Spec.AccessModes) > 0 {
				accessMode = string(pvc.Spec.AccessModes[0])
			}
			capacity := ""
			if storage, ok := pvc.Status.Capacity["storage"]; ok {
				capacity = storage.String()
			}
			sc := ""
			if pvc.Spec.StorageClassName != nil {
				sc = *pvc.Spec.StorageClassName
			}
			items = append(items, DatasetItem{
				Name:         pvc.Name,
				Namespace:    pvc.Namespace,
				Type:         "pvc",
				Status:       string(pvc.Status.Phase),
				Capacity:     capacity,
				AccessMode:   accessMode,
				StorageClass: sc,
			})
		}
	}

	// 2. List Fluid Datasets (if available)
	fluidDatasets, fluidErr := h.datasetService.ListDatasets(namespace)
	if fluidErr == nil && fluidDatasets != nil {
		for _, fd := range fluidDatasets.Items {
			status := "Unknown"
			if s, ok := fd.Object["status"].(map[string]interface{}); ok {
				if phase, ok := s["phase"].(string); ok {
					status = phase
				}
			}
			items = append(items, DatasetItem{
				Name:      fd.GetName(),
				Namespace: fd.GetNamespace(),
				Type:      "fluid",
				Status:    status,
			})
		}
	}

	response.OK(c, response.Pagination{Total: int64(len(items)), Items: items})
}

func (h *DatasetHandler) Create(c *gin.Context) {
	var req model.CreateDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeDatasetError, "invalid request: "+err.Error())
		return
	}
	if err := h.datasetService.CreateDataset(req.Name, req.Namespace, req.DatasetConf, req.RuntimeConf); err != nil {
		response.Failed(c, response.CodeDatasetError, "create dataset failed: "+err.Error())
		return
	}
	response.OKMsg(c, "dataset created", req.Name)
}

func (h *DatasetHandler) Delete(c *gin.Context) {
	name := c.Query("name")
	namespace := c.Query("namespace")
	if name == "" {
		response.Failed(c, response.CodeDatasetError, "name is required")
		return
	}
	if err := h.datasetService.DeleteDataset(name, namespace); err != nil {
		response.Failed(c, response.CodeDatasetError, "delete dataset failed: "+err.Error())
		return
	}
	response.OKMsg(c, "dataset deleted", name)
}

func (h *DatasetHandler) Update(c *gin.Context) {
	var req model.CreateDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeDatasetError, "invalid request: "+err.Error())
		return
	}
	_ = h.datasetService.DeleteDataset(req.Name, req.Namespace)
	if err := h.datasetService.CreateDataset(req.Name, req.Namespace, req.DatasetConf, req.RuntimeConf); err != nil {
		response.Failed(c, response.CodeDatasetError, "update dataset failed: "+err.Error())
		return
	}
	response.OKMsg(c, "dataset updated", req.Name)
}
