package service

import (
	"context"
	"fmt"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// NotebookService manages Notebook CRDs (kubeflow.org/v1).
type NotebookService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
}

func NewNotebookService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *NotebookService {
	return &NotebookService{
		adminClient: adminClient,
		tenants:     tenants,
	}
}

// AdminClient exposes the admin k8s client for shared access.
func (s *NotebookService) AdminClient() *k8s.Client {
	return s.adminClient
}

// List returns notebooks in the given namespaces.
func (s *NotebookService) List(namespaces []string) ([]model.NotebookInfo, error) {
	gvr := k8s.CRDGVR("notebooks")
	var results []model.NotebookInfo

	for _, ns := range namespaces {
		list, err := s.adminClient.Dynamic().Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, item := range list.Items {
			results = append(results, parseNotebook(item))
		}
	}
	return results, nil
}

// Get returns a single notebook.
func (s *NotebookService) Get(name, namespace string) (*model.NotebookInfo, error) {
	gvr := k8s.CRDGVR("notebooks")
	obj, err := s.adminClient.Dynamic().Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	nb := parseNotebook(*obj)
	return &nb, nil
}

// Create creates a new Notebook CRD.
func (s *NotebookService) Create(spec *model.NotebookSpec, userName string) error {
	gvr := k8s.CRDGVR("notebooks")

	// Use per-user client for authorization
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	nb := buildNotebookCRD(spec)
	_, err = client.Resource(gvr).Namespace(spec.Namespace).Create(context.TODO(), nb, metav1.CreateOptions{})
	return err
}

// Delete deletes a Notebook CRD.
func (s *NotebookService) Delete(name, namespace, userName string) error {
	gvr := k8s.CRDGVR("notebooks")
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}
	return client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// Stop stops a notebook by setting replicas to 0 (annotation-based).
func (s *NotebookService) Stop(name, namespace, userName string) error {
	gvr := k8s.CRDGVR("notebooks")
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	obj, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Set stop annotation
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["kubeflow-resource-stopped"] = time.Now().UTC().Format(time.RFC3339)
	obj.SetAnnotations(annotations)

	_, err = client.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
	return err
}

// Start starts a stopped notebook by removing the stop annotation.
func (s *NotebookService) Start(name, namespace, userName string) error {
	gvr := k8s.CRDGVR("notebooks")
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	obj, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	annotations := obj.GetAnnotations()
	delete(annotations, "kubeflow-resource-stopped")
	obj.SetAnnotations(annotations)

	_, err = client.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
	return err
}

// Resize updates the resource limits of a stopped notebook.
// Must be stopped first, then start again after resize.
func (s *NotebookService) Resize(name, namespace, userName string, cpu, memory string, gpu int) error {
	gvr := k8s.CRDGVR("notebooks")
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	obj, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Verify it's stopped
	annotations := obj.GetAnnotations()
	if _, stopped := annotations["kubeflow-resource-stopped"]; !stopped {
		return fmt.Errorf("notebook must be stopped before resizing")
	}

	// Navigate to container resources and patch
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		return fmt.Errorf("invalid notebook spec")
	}
	template, _ := spec["template"].(map[string]interface{})
	if template == nil {
		return fmt.Errorf("invalid notebook template")
	}
	podSpec, _ := template["spec"].(map[string]interface{})
	if podSpec == nil {
		return fmt.Errorf("invalid notebook pod spec")
	}
	containers, _ := podSpec["containers"].([]interface{})
	if len(containers) == 0 {
		return fmt.Errorf("no containers in notebook")
	}
	container, _ := containers[0].(map[string]interface{})
	if container == nil {
		return fmt.Errorf("invalid container")
	}

	newResources := map[string]interface{}{
		"requests": map[string]interface{}{"cpu": cpu, "memory": memory},
		"limits":   map[string]interface{}{"cpu": cpu, "memory": memory},
	}
	if gpu > 0 {
		newResources["limits"].(map[string]interface{})["nvidia.com/gpu"] = fmt.Sprintf("%d", gpu)
		newResources["requests"].(map[string]interface{})["nvidia.com/gpu"] = fmt.Sprintf("%d", gpu)
	}
	container["resources"] = newResources
	containers[0] = container
	podSpec["containers"] = containers
	template["spec"] = podSpec
	spec["template"] = template
	obj.Object["spec"] = spec

	_, err = client.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
	return err
}

// GetSSHInfo returns connection info for SSH access to a notebook pod.
func (s *NotebookService) GetSSHInfo(name, namespace string) (*model.NotebookSSHInfo, error) {
	// Find the pod for this notebook
	pods, err := s.adminClient.Typed().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("notebook-name=%s", name),
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no running pod found for notebook %s", name)
	}

	pod := pods.Items[0]
	info := &model.NotebookSSHInfo{
		PodName:      pod.Name,
		Namespace:    namespace,
		Node:         pod.Spec.NodeName,
		IP:           pod.Status.PodIP,
		PortForward:  fmt.Sprintf("kubectl port-forward -n %s pod/%s 2222:22", namespace, pod.Name),
		ExecCommand:  fmt.Sprintf("kubectl exec -it -n %s %s -- /bin/bash", namespace, pod.Name),
	}
	return info, nil
}

func (s *NotebookService) getUserDynamic(userName string) (dynamic.Interface, error) {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return nil, fmt.Errorf("get tenant client: %w", err)
	}
	return tc.Dynamic, nil
}

// --- CRD builders and parsers ---

func buildNotebookCRD(spec *model.NotebookSpec) *unstructured.Unstructured {
	// Build container resources
	resources := map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    spec.CPU,
			"memory": spec.Memory,
		},
		"limits": map[string]interface{}{
			"cpu":    spec.CPU,
			"memory": spec.Memory,
		},
	}
	if spec.GPU > 0 {
		gpuResource := "nvidia.com/gpu"
		if spec.GPUType != "" {
			gpuResource = spec.GPUType
		}
		resources["limits"].(map[string]interface{})[gpuResource] = fmt.Sprintf("%d", spec.GPU)
		resources["requests"].(map[string]interface{})[gpuResource] = fmt.Sprintf("%d", spec.GPU)
	}

	// Build env vars
	var envVars []interface{}
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]interface{}{
			"name":  k,
			"value": v,
		})
	}

	container := map[string]interface{}{
		"name":      spec.Name,
		"image":     spec.Image,
		"resources": resources,
	}
	if len(envVars) > 0 {
		container["env"] = envVars
	}

	// Build volume mounts for storage
	var volumeMounts []interface{}
	var volumes []interface{}
	if spec.Storage != "" {
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      "workspace",
			"mountPath": "/home/jovyan",
		})
		volumes = append(volumes, map[string]interface{}{
			"name": "workspace",
			"persistentVolumeClaim": map[string]interface{}{
				"claimName": spec.Name + "-workspace",
			},
		})
		container["volumeMounts"] = volumeMounts
	}

	podSpec := map[string]interface{}{
		"containers": []interface{}{container},
	}
	if len(volumes) > 0 {
		podSpec["volumes"] = volumes
	}

	nb := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubeflow.org/v1",
			"kind":       "Notebook",
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
			},
			"spec": map[string]interface{}{
				"template": map[string]interface{}{
					"spec": podSpec,
				},
			},
		},
	}
	return nb
}

func parseNotebook(obj unstructured.Unstructured) model.NotebookInfo {
	info := model.NotebookInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	// Parse status
	annotations := obj.GetAnnotations()
	if _, stopped := annotations["kubeflow-resource-stopped"]; stopped {
		info.Status = "Stopped"
	} else {
		info.Status = getNotebookStatus(obj)
	}

	// Parse spec for image/resources
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec != nil {
		template, _ := spec["template"].(map[string]interface{})
		if template != nil {
			podSpec, _ := template["spec"].(map[string]interface{})
			if podSpec != nil {
				containers, _ := podSpec["containers"].([]interface{})
				if len(containers) > 0 {
					container, _ := containers[0].(map[string]interface{})
					if container != nil {
						info.Image, _ = container["image"].(string)
						res, _ := container["resources"].(map[string]interface{})
						if res != nil {
							limits, _ := res["limits"].(map[string]interface{})
							if limits != nil {
								info.CPU, _ = limits["cpu"].(string)
								info.Memory, _ = limits["memory"].(string)
								if gpu, ok := limits["nvidia.com/gpu"]; ok {
									info.GPU = fmt.Sprintf("%v", gpu)
								}
							}
						}
					}
				}
			}
		}
	}

	// Calculate age
	creationTime := obj.GetCreationTimestamp()
	if !creationTime.IsZero() {
		info.Age = formatDuration(time.Since(creationTime.Time))
	}

	// Build URL based on notebook type
	if info.Status == "Running" {
		labels := obj.GetLabels()
		nbType := labels["notebook-type"]
		switch nbType {
		case "vscode":
			info.URL = fmt.Sprintf("/vscode/%s/%s/", info.Namespace, info.Name)
		default:
			// Jupyter: /notebook/{ns}/{name}/lab
			info.URL = fmt.Sprintf("/notebook/%s/%s/lab", info.Namespace, info.Name)
		}
	}

	return info
}

func getNotebookStatus(obj unstructured.Unstructured) string {
	status, _ := obj.Object["status"].(map[string]interface{})
	if status == nil {
		return "Pending"
	}
	conditions, _ := status["conditions"].([]interface{})
	for _, cond := range conditions {
		condMap, _ := cond.(map[string]interface{})
		if condMap == nil {
			continue
		}
		condType, _ := condMap["type"].(string)
		condStatus, _ := condMap["status"].(string)
		if condType == "Ready" && condStatus == "True" {
			return "Running"
		}
	}
	readyReplicas, _ := status["readyReplicas"].(int64)
	if readyReplicas > 0 {
		return "Running"
	}
	return "Pending"
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
