package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// ServingService manages inference/serving workloads.
// Supports KServe InferenceService CRD and fallback to K8s Deployment + Service.
type ServingService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
}

func NewServingService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *ServingService {
	return &ServingService{
		adminClient: adminClient,
		tenants:     tenants,
	}
}

// List returns serving jobs across namespaces.
func (s *ServingService) List(namespaces []string) ([]model.ServingInfo, error) {
	var results []model.ServingInfo

	// Try KServe InferenceService first
	gvr := k8s.CRDGVR("inferenceservices")
	for _, ns := range namespaces {
		list, err := s.adminClient.Dynamic().Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, item := range list.Items {
			results = append(results, parseInferenceService(item))
		}
	}

	// Also list Deployments with label app.kubernetes.io/component=serving
	for _, ns := range namespaces {
		deploys, err := s.adminClient.Typed().AppsV1().Deployments(ns).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/component=serving",
		})
		if err != nil {
			continue
		}
		for _, d := range deploys.Items {
			info := model.ServingInfo{
				Name:       d.Name,
				Namespace:  d.Namespace,
				Status:     getDeploymentStatus(d.Status.ReadyReplicas, *d.Spec.Replicas),
				Replicas:   fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, *d.Spec.Replicas),
				CreateTime: d.CreationTimestamp.Format(time.RFC3339),
				Framework:  d.Labels["app.kubernetes.io/framework"],
			}
			// Try to find endpoint from service
			svc, err := s.adminClient.Typed().CoreV1().Services(ns).Get(context.TODO(), d.Name, metav1.GetOptions{})
			if err == nil && svc != nil {
				if svc.Spec.Type == "LoadBalancer" && len(svc.Status.LoadBalancer.Ingress) > 0 {
					info.Endpoint = svc.Status.LoadBalancer.Ingress[0].IP
				} else {
					info.Endpoint = fmt.Sprintf("%s.%s.svc", svc.Name, svc.Namespace)
				}
			}
			results = append(results, info)
		}
	}

	return results, nil
}

// Get returns a single serving workload.
func (s *ServingService) Get(name, namespace string) (*model.ServingInfo, error) {
	gvr := k8s.CRDGVR("inferenceservices")
	obj, err := s.adminClient.Dynamic().Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		info := parseInferenceService(*obj)
		return &info, nil
	}

	// Fallback to Deployment
	deploy, err := s.adminClient.Typed().AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	info := model.ServingInfo{
		Name:       deploy.Name,
		Namespace:  deploy.Namespace,
		Status:     getDeploymentStatus(deploy.Status.ReadyReplicas, *deploy.Spec.Replicas),
		Replicas:   fmt.Sprintf("%d/%d", deploy.Status.ReadyReplicas, *deploy.Spec.Replicas),
		CreateTime: deploy.CreationTimestamp.Format(time.RFC3339),
	}
	return &info, nil
}

// Create deploys an inference service.
func (s *ServingService) Create(spec *model.ServingSpec, userName string) error {
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	// Try KServe InferenceService if framework matches
	if spec.Framework == "tensorflow" || spec.Framework == "pytorch" || spec.Framework == "sklearn" {
		return s.createInferenceService(client, spec)
	}

	// Fallback: create Deployment + Service
	return s.createDeploymentServing(spec, userName)
}

// Delete removes a serving workload.
func (s *ServingService) Delete(name, namespace, userName string) error {
	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	// Try InferenceService first
	gvr := k8s.CRDGVR("inferenceservices")
	err = client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err == nil {
		return nil
	}

	// Fallback: delete Deployment + Service
	tc, _ := s.tenants.GetClient(userName)
	_ = tc.Typed.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	_ = tc.Typed.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	return nil
}

// TestEndpoint proxies a chat/completion request to the serving service endpoint.
func (s *ServingService) TestEndpoint(namespace, name string, reqBody map[string]interface{}) (map[string]interface{}, error) {
	// Resolve the Service endpoint
	svc, err := s.adminClient.Typed().CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get service %s/%s: %w", namespace, name, err)
	}

	// Determine target URL (service ClusterIP + port)
	port := int32(8000) // default vLLM/SGLang port
	if len(svc.Spec.Ports) > 0 {
		port = svc.Spec.Ports[0].Port
	}
	targetURL := fmt.Sprintf("http://%s.%s.svc:%d/v1/chat/completions", name, namespace, port)

	// Marshal request body
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Make the HTTP request via the K8s service DNS
	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Post(targetURL, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("request to inference endpoint: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		// Return raw response as string
		return map[string]interface{}{"raw": string(respBytes)}, nil
	}
	return result, nil
}

// ListServingPods returns pods for a serving deployment.
func (s *ServingService) ListServingPods(namespace, name string) ([]model.PodInfo, error) {
	pods, err := s.adminClient.Typed().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", name),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods for serving %s/%s: %w", namespace, name, err)
	}

	var results []model.PodInfo
	for _, pod := range pods.Items {
		info := model.PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Node:      pod.Spec.NodeName,
			IP:        pod.Status.PodIP,
			Restarts:  getServingPodRestartCount(pod),
			Role:      "serving",
		}
		if !pod.CreationTimestamp.IsZero() {
			info.Age = formatDuration(time.Since(pod.CreationTimestamp.Time))
		}
		results = append(results, info)
	}
	return results, nil
}

// GetServingPodLogs returns logs for a serving pod.
func (s *ServingService) GetServingPodLogs(namespace, podName, container string, tailLines int64) (string, error) {
	opts := &corev1.PodLogOptions{}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}
	if container != "" {
		opts.Container = container
	}

	req := s.adminClient.Typed().CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("get logs for pod %s/%s: %w", namespace, podName, err)
	}
	defer stream.Close()

	logBytes, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("read log stream: %w", err)
	}
	return string(logBytes), nil
}

func getServingPodRestartCount(pod corev1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

func (s *ServingService) getUserDynamic(userName string) (dynamic.Interface, error) {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return nil, fmt.Errorf("get tenant client: %w", err)
	}
	return tc.Dynamic, nil
}

func (s *ServingService) createInferenceService(client dynamic.Interface, spec *model.ServingSpec) error {
	gvr := k8s.CRDGVR("inferenceservices")

	container := map[string]interface{}{
		"image": spec.Image,
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    spec.CPU,
				"memory": spec.Memory,
			},
			"limits": map[string]interface{}{
				"cpu":    spec.CPU,
				"memory": spec.Memory,
			},
		},
	}
	if spec.GPU > 0 {
		container["resources"].(map[string]interface{})["limits"].(map[string]interface{})["nvidia.com/gpu"] = fmt.Sprintf("%d", spec.GPU)
	}
	if spec.ModelPath != "" {
		container["storageUri"] = spec.ModelPath
	}

	// Build env
	var envVars []interface{}
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]interface{}{"name": k, "value": v})
	}
	if len(envVars) > 0 {
		container["env"] = envVars
	}

	predictor := map[string]interface{}{
		"containers": []interface{}{container},
	}

	isvc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "serving.kserve.io/v1beta1",
			"kind":       "InferenceService",
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/component":         "serving",
					"app.kubernetes.io/framework":         spec.Framework,
					"alibabacloud.com/inference-workload": spec.Name,
					"alibabacloud.com/inference_backend":  spec.Framework,
				},
			},
			"spec": map[string]interface{}{
				"predictor": predictor,
			},
		},
	}
	_, err := client.Resource(gvr).Namespace(spec.Namespace).Create(context.TODO(), isvc, metav1.CreateOptions{})
	return err
}

func (s *ServingService) createDeploymentServing(spec *model.ServingSpec, userName string) error {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return err
	}

	labels := map[string]string{
		"app":                                  spec.Name,
		"app.kubernetes.io/component":          "serving",
		"app.kubernetes.io/framework":          spec.Framework,
		"alibabacloud.com/inference-workload":  spec.Name,
		"alibabacloud.com/inference_backend":   spec.Framework,
	}

	// Build container
	resources := map[string]interface{}{
		"requests": map[string]interface{}{"cpu": spec.CPU, "memory": spec.Memory},
		"limits":   map[string]interface{}{"cpu": spec.CPU, "memory": spec.Memory},
	}
	if spec.GPU > 0 {
		resources["limits"].(map[string]interface{})["nvidia.com/gpu"] = fmt.Sprintf("%d", spec.GPU)
	}

	container := map[string]interface{}{
		"name":      spec.Name,
		"image":     spec.Image,
		"resources": resources,
		"ports":     []interface{}{map[string]interface{}{"containerPort": spec.Port}},
	}
	if spec.Command != "" {
		container["command"] = []interface{}{"sh", "-c", spec.Command}
	}
	var envVars []interface{}
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]interface{}{"name": k, "value": v})
	}
	if len(envVars) > 0 {
		container["env"] = envVars
	}

	// Create Deployment via unstructured
	deploy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"replicas": spec.Replicas,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{"app": spec.Name},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{"labels": labels},
					"spec": map[string]interface{}{
						"containers": []interface{}{container},
					},
				},
			},
		},
	}

	deployGVR := k8s.DeploymentGVR()
	_, err = tc.Dynamic.Resource(deployGVR).Namespace(spec.Namespace).Create(context.TODO(), deploy, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Create Service
	svc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
				"labels":    labels,
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{"app": spec.Name},
				"ports": []interface{}{
					map[string]interface{}{
						"port":       spec.Port,
						"targetPort": spec.Port,
						"protocol":   "TCP",
					},
				},
				"type": "ClusterIP",
			},
		},
	}

	svcGVR := k8s.ServiceGVR()
	_, err = tc.Dynamic.Resource(svcGVR).Namespace(spec.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	return err
}

// --- Parsers ---

func parseInferenceService(obj unstructured.Unstructured) model.ServingInfo {
	info := model.ServingInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Framework: obj.GetLabels()["app.kubernetes.io/framework"],
	}

	// Parse status
	status, _ := obj.Object["status"].(map[string]interface{})
	if status != nil {
		conditions, _ := status["conditions"].([]interface{})
		info.Status = "Pending"
		for _, cond := range conditions {
			condMap, _ := cond.(map[string]interface{})
			if condMap == nil {
				continue
			}
			if condMap["type"] == "Ready" {
				if condMap["status"] == "True" {
					info.Status = "Ready"
				} else {
					info.Status = "NotReady"
				}
			}
		}
		if urlStr, ok := status["url"].(string); ok {
			info.Endpoint = urlStr
		}
	} else {
		info.Status = "Pending"
	}

	creationTime := obj.GetCreationTimestamp()
	if !creationTime.IsZero() {
		info.CreateTime = creationTime.Format(time.RFC3339)
	}

	return info
}

func getDeploymentStatus(ready, desired int32) string {
	if ready == desired && desired > 0 {
		return "Ready"
	}
	if ready == 0 {
		return "Pending"
	}
	return "Progressing"
}
