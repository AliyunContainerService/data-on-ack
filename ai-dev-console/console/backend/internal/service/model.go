package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	modelRegistryLabel = "app.kubernetes.io/component"
	modelRegistryValue = "model-registry"
)

// ModelService manages a lightweight model registry backed by ConfigMaps.
type ModelService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
}

func NewModelService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *ModelService {
	return &ModelService{
		adminClient: adminClient,
		tenants:     tenants,
	}
}

// List returns all registered models from ConfigMaps in user namespaces.
func (s *ModelService) List(namespaces []string) ([]model.ModelInfo, error) {
	var results []model.ModelInfo
	selector := fmt.Sprintf("%s=%s", modelRegistryLabel, modelRegistryValue)

	for _, ns := range namespaces {
		cms, err := s.adminClient.Typed().CoreV1().ConfigMaps(ns).List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			continue
		}
		for _, cm := range cms.Items {
			info := parseModelConfigMap(cm)
			results = append(results, info)
		}
	}
	return results, nil
}

// Register creates a new model registry entry.
func (s *ModelService) Register(spec *model.ModelRegisterSpec, userName string) error {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return fmt.Errorf("get tenant client: %w", err)
	}

	// Serialize model metadata to JSON for storage in ConfigMap data
	metaJSON, _ := json.Marshal(spec)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "model-" + spec.Name,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				modelRegistryLabel:            modelRegistryValue,
				"app.kubernetes.io/managed-by": "ai-dev-console",
				"model-name":                  spec.Name,
				"model-framework":             spec.Framework,
			},
			Annotations: map[string]string{
				"model-version": spec.Version,
				"model-source":  spec.Source,
			},
		},
		Data: map[string]string{
			"metadata":  string(metaJSON),
			"name":      spec.Name,
			"version":   spec.Version,
			"framework": spec.Framework,
			"path":      spec.Path,
			"source":    spec.Source,
			"size":      spec.Size,
		},
	}

	_, err = tc.Typed.CoreV1().ConfigMaps(spec.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	return err
}

// Delete removes a model registry entry.
func (s *ModelService) Delete(name, namespace, userName string) error {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return fmt.Errorf("get tenant client: %w", err)
	}
	cmName := "model-" + name
	return tc.Typed.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), cmName, metav1.DeleteOptions{})
}

func parseModelConfigMap(cm corev1.ConfigMap) model.ModelInfo {
	info := model.ModelInfo{
		Name:      cm.Data["name"],
		Namespace: cm.Namespace,
		Version:   cm.Data["version"],
		Framework: cm.Data["framework"],
		Path:      cm.Data["path"],
		Source:    cm.Data["source"],
		Size:      cm.Data["size"],
	}
	if info.Name == "" {
		info.Name = cm.Labels["model-name"]
	}
	if !cm.CreationTimestamp.IsZero() {
		info.RegisterTime = cm.CreationTimestamp.Format("2006-01-02 15:04")
	}
	return info
}
