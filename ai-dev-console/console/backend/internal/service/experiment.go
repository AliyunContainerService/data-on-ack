package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	experimentLabel = "app.kubernetes.io/component"
	experimentValue = "experiment"
)

// ExperimentService manages experiments backed by ConfigMaps.
type ExperimentService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
}

func NewExperimentService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *ExperimentService {
	return &ExperimentService{
		adminClient: adminClient,
		tenants:     tenants,
	}
}

// List returns all experiments across given namespaces.
func (s *ExperimentService) List(namespaces []string) ([]model.ExperimentInfo, error) {
	var results []model.ExperimentInfo
	selector := fmt.Sprintf("%s=%s", experimentLabel, experimentValue)

	for _, ns := range namespaces {
		cms, err := s.adminClient.Typed().CoreV1().ConfigMaps(ns).List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			continue
		}
		for _, cm := range cms.Items {
			info := parseExperimentConfigMap(cm)
			results = append(results, info)
		}
	}

	// Sort by creation time descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreateTime > results[j].CreateTime
	})

	return results, nil
}

// Get returns a single experiment with its runs.
func (s *ExperimentService) Get(name, namespace string) (*model.ExperimentInfo, error) {
	cmName := "experiment-" + name
	cm, err := s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get experiment %s/%s: %w", namespace, name, err)
	}
	info := parseExperimentConfigMap(*cm)
	return &info, nil
}

// Create creates a new experiment.
func (s *ExperimentService) Create(spec *model.ExperimentCreateSpec, userName string) error {
	cmName := "experiment-" + spec.Name

	// Check if already exists
	_, err := s.adminClient.Typed().CoreV1().ConfigMaps(spec.Namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("experiment '%s' already exists in namespace '%s'", spec.Name, spec.Namespace)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("check experiment existence: %w", err)
	}

	// Build experiment data
	expData := model.ExperimentData{
		Name:        spec.Name,
		Description: spec.Description,
		Runs:        []model.ExperimentRun{},
		CreatedBy:   userName,
	}
	dataJSON, err := json.Marshal(expData)
	if err != nil {
		return fmt.Errorf("marshal experiment data: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: spec.Namespace,
			Labels: map[string]string{
				experimentLabel:                experimentValue,
				"app.kubernetes.io/managed-by": "ai-dev-console",
				"experiment-name":              spec.Name,
			},
		},
		Data: map[string]string{
			"experiment": string(dataJSON),
			"name":       spec.Name,
		},
	}

	_, err = s.adminClient.Typed().CoreV1().ConfigMaps(spec.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	return err
}

// AddRun associates a training job run with an experiment.
func (s *ExperimentService) AddRun(experimentName, namespace string, run *model.ExperimentRunSpec) error {
	cmName := "experiment-" + experimentName
	cm, err := s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get experiment %s: %w", experimentName, err)
	}

	// Parse existing data
	var expData model.ExperimentData
	if dataStr, ok := cm.Data["experiment"]; ok {
		if err := json.Unmarshal([]byte(dataStr), &expData); err != nil {
			return fmt.Errorf("parse experiment data: %w", err)
		}
	}

	// Add the new run
	newRun := model.ExperimentRun{
		JobName:   run.JobName,
		JobKind:   run.JobKind,
		Params:    run.Params,
		Metrics:   run.Metrics,
		Status:    run.Status,
		AddedAt:   time.Now().Format(time.RFC3339),
	}
	expData.Runs = append(expData.Runs, newRun)

	// Save back
	dataJSON, err := json.Marshal(expData)
	if err != nil {
		return fmt.Errorf("marshal experiment data: %w", err)
	}
	cm.Data["experiment"] = string(dataJSON)

	_, err = s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

// UpdateRunMetrics updates the metrics/status of a specific run in an experiment.
func (s *ExperimentService) UpdateRunMetrics(experimentName, namespace, jobName string, metrics map[string]float64, status string) error {
	cmName := "experiment-" + experimentName
	cm, err := s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Get(context.TODO(), cmName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get experiment %s: %w", experimentName, err)
	}

	var expData model.ExperimentData
	if dataStr, ok := cm.Data["experiment"]; ok {
		if err := json.Unmarshal([]byte(dataStr), &expData); err != nil {
			return fmt.Errorf("parse experiment data: %w", err)
		}
	}

	// Find and update the run
	for i := range expData.Runs {
		if expData.Runs[i].JobName == jobName {
			if metrics != nil {
				expData.Runs[i].Metrics = metrics
			}
			if status != "" {
				expData.Runs[i].Status = status
			}
			break
		}
	}

	dataJSON, err := json.Marshal(expData)
	if err != nil {
		return fmt.Errorf("marshal experiment data: %w", err)
	}
	cm.Data["experiment"] = string(dataJSON)

	_, err = s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

// Delete removes an experiment.
func (s *ExperimentService) Delete(name, namespace string) error {
	cmName := "experiment-" + name
	return s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Delete(context.TODO(), cmName, metav1.DeleteOptions{})
}

func parseExperimentConfigMap(cm corev1.ConfigMap) model.ExperimentInfo {
	info := model.ExperimentInfo{
		Name:      cm.Labels["experiment-name"],
		Namespace: cm.Namespace,
	}

	if info.Name == "" {
		info.Name = cm.Data["name"]
	}

	if !cm.CreationTimestamp.IsZero() {
		info.CreateTime = cm.CreationTimestamp.Format(time.RFC3339)
	}

	// Parse full experiment data
	if dataStr, ok := cm.Data["experiment"]; ok {
		var expData model.ExperimentData
		if err := json.Unmarshal([]byte(dataStr), &expData); err == nil {
			info.Description = expData.Description
			info.CreatedBy = expData.CreatedBy
			info.RunCount = len(expData.Runs)
			info.Runs = expData.Runs

			// Find best loss
			for _, run := range expData.Runs {
				if loss, ok := run.Metrics["loss"]; ok {
					if info.BestLoss == 0 || loss < info.BestLoss {
						info.BestLoss = loss
					}
				}
			}
		}
	}

	return info
}
