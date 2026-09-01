package handler

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	settingsConfigMap = "kubeai-platform-config"
	settingsNamespace = "kube-ai"
)

type SettingsHandler struct {
	kubeClient *k8s.Client
}

func newSettingsHandler(kubeClient *k8s.Client) *SettingsHandler {
	return &SettingsHandler{kubeClient: kubeClient}
}

func (h *SettingsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// All /ops/* endpoints are admin-only platform management operations.
	rg.GET("/ops/settings", adminOnly, h.GetSettings)
	rg.POST("/ops/settings", adminOnly, h.SaveSettings)
	rg.GET("/ops/images", adminOnly, h.ListImages)
	rg.POST("/ops/images/delete", adminOnly, h.DeleteImage)
}

type PlatformConfig struct {
	CullingEnabled    bool   `json:"cullingEnabled"`
	CullingIdleTime   int    `json:"cullingIdleTime"`   // minutes
	CullingCheckPeriod int   `json:"cullingCheckPeriod"` // minutes
	DefaultImages     string `json:"defaultImages"`     // newline-separated
	CommitRegistry    string `json:"commitRegistry"`
}

func (h *SettingsHandler) GetSettings(c *gin.Context) {
	cm, err := h.kubeClient.Typed().CoreV1().ConfigMaps(settingsNamespace).Get(
		context.TODO(), settingsConfigMap, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Return defaults
			response.OK(c, PlatformConfig{
				CullingEnabled:    true,
				CullingIdleTime:   1440,
				CullingCheckPeriod: 1,
				DefaultImages: "registry.cn-hangzhou.aliyuncs.com/acs/jupyter-pytorch:2.1-gpu-cuda12.1\nregistry.cn-hangzhou.aliyuncs.com/acs/jupyter-tensorflow:2.14-gpu-cuda12.1\nregistry.cn-hangzhou.aliyuncs.com/acs/jupyter-scipy:latest",
				CommitRegistry: "registry.cn-beijing.aliyuncs.com/kubeai",
			})
			return
		}
		response.Failed(c, response.CodeK8sError, err.Error())
		return
	}

	cfg := PlatformConfig{
		CullingEnabled:    cm.Data["cullingEnabled"] == "true",
		CullingIdleTime:   parseInt(cm.Data["cullingIdleTime"], 1440),
		CullingCheckPeriod: parseInt(cm.Data["cullingCheckPeriod"], 1),
		DefaultImages:     cm.Data["defaultImages"],
		CommitRegistry:    cm.Data["commitRegistry"],
	}
	response.OK(c, cfg)
}

func (h *SettingsHandler) SaveSettings(c *gin.Context) {
	var cfg PlatformConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.Failed(c, response.CodeK8sError, "invalid body: "+err.Error())
		return
	}

	data := map[string]string{
		"cullingEnabled":    boolStr(cfg.CullingEnabled),
		"cullingIdleTime":   intStr(cfg.CullingIdleTime),
		"cullingCheckPeriod": intStr(cfg.CullingCheckPeriod),
		"defaultImages":     cfg.DefaultImages,
		"commitRegistry":    cfg.CommitRegistry,
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      settingsConfigMap,
			Namespace: settingsNamespace,
		},
		Data: data,
	}

	_, err := h.kubeClient.Typed().CoreV1().ConfigMaps(settingsNamespace).Get(
		context.TODO(), settingsConfigMap, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = h.kubeClient.Typed().CoreV1().ConfigMaps(settingsNamespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	} else if err == nil {
		_, err = h.kubeClient.Typed().CoreV1().ConfigMaps(settingsNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	}
	if err != nil {
		response.Failed(c, response.CodeK8sError, err.Error())
		return
	}
	response.OK(c, nil)
}

// ListImages returns AI-related images cached on each node (from node.status.images).
// This helps admins know which nodes already have notebook/training images (faster scheduling).
func (h *SettingsHandler) ListImages(c *gin.Context) {
	type NodeImage struct {
		Image    string   `json:"image"`
		SizeMB   int64    `json:"sizeMB"`
		Tags     []string `json:"tags"`
	}
	type NodeImageInfo struct {
		Node   string      `json:"node"`
		IP     string      `json:"ip"`
		GPU    int64       `json:"gpu"`
		Images []NodeImage `json:"images"`
	}

	// AI-related image keywords to filter
	aiKeywords := []string{
		"jupyter", "pytorch", "tensorflow", "vscode", "cuda", "notebook",
		"vllm", "sglang", "triton", "torchserve", "tgi", "nvcr.io",
		"commit-agent", "ai-dev", "ai-dashboard", "ac2/", "training",
		"arena", "deepspeed", "transformers", "diffusers",
	}

	nodes, err := h.kubeClient.Typed().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		response.Failed(c, response.CodeK8sError, err.Error())
		return
	}

	var result []NodeImageInfo
	for _, node := range nodes.Items {
		info := NodeImageInfo{
			Node: node.Name,
		}
		// IP
		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				info.IP = addr.Address
				break
			}
		}
		// GPU
		if gpu, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			info.GPU = gpu.Value()
		}

		// Filter AI images from node.status.images
		for _, img := range node.Status.Images {
			if len(img.Names) == 0 {
				continue
			}
			name := img.Names[0]
			// Check if matches AI keywords
			nameLower := strings.ToLower(name)
			isAI := false
			for _, kw := range aiKeywords {
				if strings.Contains(nameLower, kw) {
					isAI = true
					break
				}
			}
			if !isAI {
				continue
			}
			nodeImg := NodeImage{
				Image:  name,
				SizeMB: img.SizeBytes / (1024 * 1024),
				Tags:   img.Names[1:], // additional tags
			}
			info.Images = append(info.Images, nodeImg)
		}

		result = append(result, info)
	}

	response.OK(c, result)
}

// DeleteImage removes an image from a specific node using crictl via node-shell.
func (h *SettingsHandler) DeleteImage(c *gin.Context) {
	var req struct {
		NodeName string `json:"nodeName" binding:"required"`
		Image    string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeK8sError, "invalid request: "+err.Error())
		return
	}

	// Defense in depth: image references never contain whitespace or shell
	// metacharacters; reject anything suspicious even though the command is
	// executed in argv form below.
	if strings.ContainsAny(req.Image, " \t\n;&|<>$`\"'") {
		response.Failed(c, response.CodeK8sError, "invalid image name")
		return
	}

	// Use node-shell handler to exec crictl rmi on the target node.
	// argv form (no shell) prevents command injection via the image name.
	ctx, cancel := context.WithTimeout(c.Request.Context(), nodeShellExecLimit)
	defer cancel()
	shellHandler := newNodeShellHandler(h.kubeClient)
	result := shellHandler.execOnNodeCmd(ctx, req.NodeName, []string{"crictl", "rmi", req.Image})
	if result.Error != "" {
		response.Failed(c, response.CodeK8sError, "delete image failed: "+result.Error+" "+result.Stderr)
		return
	}
	response.OK(c, result)
}

func parseInt(s string, def int) int {
	var n int
	_, err := json.Number(s).Int64()
	if err != nil {
		return def
	}
	json.Unmarshal([]byte(s), &n)
	return n
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func intStr(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

// Ensure imports are used
var _ = strings.Join
