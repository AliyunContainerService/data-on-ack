package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	modelRegistryLabel = "app.kubernetes.io/component"
	modelRegistryValue = "model-registry"
	modelVersionLabel  = "model-version-registry"
)

// ModelAdminService manages model versions and lineage for the admin dashboard.
type ModelAdminService struct {
	kubeClient *k8s.Client
}

func NewModelAdminService(kubeClient *k8s.Client) *ModelAdminService {
	return &ModelAdminService{kubeClient: kubeClient}
}

// ModelInfo represents a model in the registry.
type ModelInfo struct {
	Name         string         `json:"name"`
	Namespace    string         `json:"namespace"`
	Framework    string         `json:"framework"`
	Path         string         `json:"path"`
	Source       string         `json:"source"`
	Size         string         `json:"size"`
	VersionCount int            `json:"versionCount"`
	LatestVersion string        `json:"latestVersion"`
	RegisterTime string         `json:"registerTime"`
	Versions     []ModelVersion `json:"versions,omitempty"`
}

// ModelVersion represents a version of a model.
type ModelVersion struct {
	Version     string             `json:"version"`
	TrainedFrom string             `json:"trainedFrom,omitempty"` // training job name
	DatasetRef  string             `json:"datasetRef,omitempty"`  // dataset used
	CreatedAt   string             `json:"createdAt"`
	Metrics     map[string]float64 `json:"metrics,omitempty"`
	Status      string             `json:"status"` // ready, training, failed
	ServingName string             `json:"servingName,omitempty"` // if deployed
}

// ModelVersionData is stored in ConfigMap.
type ModelVersionData struct {
	Versions []ModelVersion `json:"versions"`
}

// LineageNode represents a node in the model lineage DAG.
type LineageNode struct {
	ID   string `json:"id"`
	Type string `json:"type"` // dataset, training, model, serving
	Name string `json:"name"`
}

// LineageEdge connects two lineage nodes.
type LineageEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ModelLineage represents the full lineage graph for a model.
type ModelLineage struct {
	Nodes []LineageNode `json:"nodes"`
	Edges []LineageEdge `json:"edges"`
}

// ListModels returns all registered models across namespaces.
func (s *ModelAdminService) ListModels() ([]ModelInfo, error) {
	var results []ModelInfo
	selector := fmt.Sprintf("%s=%s", modelRegistryLabel, modelRegistryValue)

	cms, err := s.kubeClient.Typed().CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list model configmaps: %w", err)
	}

	for _, cm := range cms.Items {
		info := ModelInfo{
			Name:      cm.Data["name"],
			Namespace: cm.Namespace,
			Framework: cm.Data["framework"],
			Path:      cm.Data["path"],
			Source:    cm.Data["source"],
			Size:      cm.Data["size"],
		}
		if info.Name == "" {
			info.Name = cm.Labels["model-name"]
		}
		if !cm.CreationTimestamp.IsZero() {
			info.RegisterTime = cm.CreationTimestamp.Format(time.RFC3339)
		}

		// Get version count
		versions, _ := s.getVersionsForModel(info.Name, cm.Namespace)
		info.VersionCount = len(versions)
		if len(versions) > 0 {
			info.LatestVersion = versions[len(versions)-1].Version
		}

		results = append(results, info)
	}

	return results, nil
}

// GetModelVersions returns all versions of a specific model.
func (s *ModelAdminService) GetModelVersions(name, namespace string) ([]ModelVersion, error) {
	return s.getVersionsForModel(name, namespace)
}

// CreateModelVersion adds a new version to a model.
func (s *ModelAdminService) CreateModelVersion(name, namespace string, version *ModelVersion) error {
	cmName := "model-versions-" + name

	cm, err := s.kubeClient.Typed().CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		// Create new version ConfigMap
		data := ModelVersionData{
			Versions: []ModelVersion{*version},
		}
		dataJSON, _ := json.Marshal(data)

		newCM := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: namespace,
				Labels: map[string]string{
					modelRegistryLabel:             modelVersionLabel,
					"app.kubernetes.io/managed-by": "ai-dashboard",
					"model-name":                   name,
				},
			},
			Data: map[string]string{
				"versions": string(dataJSON),
			},
		}
		_, err = s.kubeClient.Typed().CoreV1().ConfigMaps(namespace).Create(context.TODO(), newCM, metav1.CreateOptions{})
		return err
	}

	// Append to existing
	var data ModelVersionData
	if versionsStr, ok := cm.Data["versions"]; ok {
		json.Unmarshal([]byte(versionsStr), &data)
	}

	// Check for duplicate version
	for _, v := range data.Versions {
		if v.Version == version.Version {
			return fmt.Errorf("version %s already exists for model %s", version.Version, name)
		}
	}

	data.Versions = append(data.Versions, *version)
	dataJSON, _ := json.Marshal(data)
	cm.Data["versions"] = string(dataJSON)

	_, err = s.kubeClient.Typed().CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

// GetModelLineage returns the lineage DAG for a model.
func (s *ModelAdminService) GetModelLineage(name, namespace string) (*ModelLineage, error) {
	lineage := &ModelLineage{}

	// Model node
	modelNodeID := "model-" + name
	lineage.Nodes = append(lineage.Nodes, LineageNode{
		ID: modelNodeID, Type: "model", Name: name,
	})

	// Get versions to find training jobs and datasets
	versions, _ := s.getVersionsForModel(name, namespace)
	seenDatasets := map[string]bool{}
	seenJobs := map[string]bool{}

	for _, v := range versions {
		// Training job node
		if v.TrainedFrom != "" && !seenJobs[v.TrainedFrom] {
			jobNodeID := "training-" + v.TrainedFrom
			lineage.Nodes = append(lineage.Nodes, LineageNode{
				ID: jobNodeID, Type: "training", Name: v.TrainedFrom,
			})
			lineage.Edges = append(lineage.Edges, LineageEdge{
				From: jobNodeID, To: modelNodeID,
			})
			seenJobs[v.TrainedFrom] = true

			// Dataset node
			if v.DatasetRef != "" && !seenDatasets[v.DatasetRef] {
				dsNodeID := "dataset-" + v.DatasetRef
				lineage.Nodes = append(lineage.Nodes, LineageNode{
					ID: dsNodeID, Type: "dataset", Name: v.DatasetRef,
				})
				lineage.Edges = append(lineage.Edges, LineageEdge{
					From: dsNodeID, To: jobNodeID,
				})
				seenDatasets[v.DatasetRef] = true
			}
		}

		// Serving node
		if v.ServingName != "" {
			servingNodeID := "serving-" + v.ServingName
			lineage.Nodes = append(lineage.Nodes, LineageNode{
				ID: servingNodeID, Type: "serving", Name: v.ServingName,
			})
			lineage.Edges = append(lineage.Edges, LineageEdge{
				From: modelNodeID, To: servingNodeID,
			})
		}
	}

	// Also check for InferenceServices referencing this model
	isvcGVR := schema.GroupVersionResource{
		Group: "serving.kserve.io", Version: "v1beta1", Resource: "inferenceservices",
	}
	isvcs, err := s.kubeClient.Dynamic().Resource(isvcGVR).Namespace(namespace).List(context.TODO(), metav1.ListOptions{})
	if err == nil {
		for _, isvc := range isvcs.Items {
			annotations := isvc.GetAnnotations()
			if annotations["model-lineage/model-name"] == name {
				servingNodeID := "serving-" + isvc.GetName()
				// Avoid duplicates
				found := false
				for _, n := range lineage.Nodes {
					if n.ID == servingNodeID {
						found = true
						break
					}
				}
				if !found {
					lineage.Nodes = append(lineage.Nodes, LineageNode{
						ID: servingNodeID, Type: "serving", Name: isvc.GetName(),
					})
					lineage.Edges = append(lineage.Edges, LineageEdge{
						From: modelNodeID, To: servingNodeID,
					})
				}
			}
		}
	}

	return lineage, nil
}

func (s *ModelAdminService) getVersionsForModel(name, namespace string) ([]ModelVersion, error) {
	cmName := "model-versions-" + name
	cm, err := s.kubeClient.Typed().CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, nil // No versions yet
	}

	var data ModelVersionData
	if versionsStr, ok := cm.Data["versions"]; ok {
		if err := json.Unmarshal([]byte(versionsStr), &data); err != nil {
			return nil, fmt.Errorf("parse versions: %w", err)
		}
	}

	// Sort by createdAt
	sort.Slice(data.Versions, func(i, j int) bool {
		return data.Versions[i].CreatedAt < data.Versions[j].CreatedAt
	})

	return data.Versions, nil
}
