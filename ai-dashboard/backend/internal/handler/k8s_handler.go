package handler

import (
	"context"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sHandler struct {
	kubeClient *k8s.Client
}

func newK8sHandler(kubeClient *k8s.Client) *K8sHandler {
	return &K8sHandler{kubeClient: kubeClient}
}

func (h *K8sHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/k8s/pvc/list", h.ListPVCs)
	rg.GET("/k8s/secret/list", h.ListSecrets)
	rg.GET("/k8s/namespace/list", h.ListNamespaces)
	rg.GET("/k8s/rbac/options", h.ListRBACOptions)
}

// ListRBACOptions returns available Roles and ClusterRoles for user group configuration.
func (h *K8sHandler) ListRBACOptions(c *gin.Context) {
	// List ClusterRoles with kubeai prefix
	clusterRoles, err := h.kubeClient.Typed().RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
	var crNames []string
	if err == nil {
		for _, cr := range clusterRoles.Items {
			name := cr.Name
			// Include kubeai roles and common admin/viewer roles
			if len(name) > 0 && (contains(name, "kubeai") || contains(name, "researcher") || contains(name, "admin")) {
				if !contains(name, "system:") && !contains(name, "kubeai:kube-ai:") {
					crNames = append(crNames, name)
				}
			}
		}
	}

	// List Roles in user-related namespaces
	var roleNames []string
	for _, ns := range []string{"default-group", "kube-ai", "default"} {
		roles, err := h.kubeClient.Typed().RbacV1().Roles(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, r := range roles.Items {
			if contains(r.Name, "kubeai") || contains(r.Name, "researcher") {
				roleNames = append(roleNames, r.Name)
			}
		}
	}

	// Deduplicate
	roleNames = unique(roleNames)
	crNames = unique(crNames)

	response.OK(c, map[string]interface{}{
		"roles":        roleNames,
		"clusterRoles": crNames,
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func unique(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func (h *K8sHandler) ListPVCs(c *gin.Context) {
	namespace := c.Query("namespace")
	pvcs, err := h.kubeClient.Typed().CoreV1().PersistentVolumeClaims(namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		response.Failed(c, response.CodeUserNotFound, "list pvcs failed: "+err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(pvcs.Items))
	for _, pvc := range pvcs.Items {
		items = append(items, map[string]interface{}{
			"name":      pvc.Name,
			"namespace": pvc.Namespace,
			"status":    string(pvc.Status.Phase),
			"capacity":  pvc.Spec.Resources.Requests,
		})
	}
	response.OK(c, response.Pagination{Total: int64(len(items)), Items: items})
}

func (h *K8sHandler) ListSecrets(c *gin.Context) {
	namespace := c.Query("namespace")
	secrets, err := h.kubeClient.Typed().CoreV1().Secrets(namespace).List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		response.Failed(c, response.CodeUserNotFound, "list secrets failed: "+err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(secrets.Items))
	for _, s := range secrets.Items {
		keys := make([]string, 0, len(s.Data))
		for k := range s.Data {
			keys = append(keys, k)
		}
		items = append(items, map[string]interface{}{
			"name":      s.Name,
			"namespace": s.Namespace,
			"type":      string(s.Type),
			"keys":      keys,
		})
	}
	response.OK(c, response.Pagination{Total: int64(len(items)), Items: items})
}

func (h *K8sHandler) ListNamespaces(c *gin.Context) {
	nss, err := h.kubeClient.Typed().CoreV1().Namespaces().List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		response.Failed(c, response.CodeUserNotFound, "list namespaces failed: "+err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(nss.Items))
	for _, ns := range nss.Items {
		if ns.Status.Phase == corev1.NamespaceActive {
			items = append(items, map[string]interface{}{
				"name":   ns.Name,
				"status": string(ns.Status.Phase),
			})
		}
	}
	response.OK(c, response.Pagination{Total: int64(len(items)), Items: items})
}
