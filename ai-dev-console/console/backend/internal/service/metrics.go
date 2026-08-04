package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetricsService scrapes GPU metrics from node-gpu-exporter endpoints.
type MetricsService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
	httpClient  *http.Client
}

// MetricPoint represents a single metric data point.
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// PodMetrics holds GPU metrics for a single pod.
type PodMetrics struct {
	Pod     string        `json:"pod"`
	Node    string        `json:"node"`
	GPU     int           `json:"gpu"`
	Metrics []MetricPoint `json:"metrics"`
}

// GPUMetricsResponse is the response for the metrics API.
type GPUMetricsResponse struct {
	Available bool         `json:"available"`
	Message   string       `json:"message,omitempty"`
	Pods      []PodMetrics `json:"pods,omitempty"`
}

func NewMetricsService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *MetricsService {
	return &MetricsService{
		adminClient: adminClient,
		tenants:     tenants,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetTrainingJobMetrics fetches GPU metrics for pods of a training job.
// It scrapes the node-gpu-exporter service in arms-prom namespace for DCGM metrics.
func (s *MetricsService) GetTrainingJobMetrics(namespace, jobName, kind, metricName string) (*GPUMetricsResponse, error) {
	// 1. Find pods for this training job
	pods, err := s.getJobPods(namespace, jobName, kind)
	if err != nil {
		return &GPUMetricsResponse{
			Available: false,
			Message:   fmt.Sprintf("failed to list pods: %v", err),
		}, nil
	}

	if len(pods) == 0 {
		return &GPUMetricsResponse{
			Available: false,
			Message:   "no running pods found for this job",
		}, nil
	}

	// 2. Find the node-gpu-exporter service endpoint
	exporterEndpoints, err := s.getGPUExporterEndpoints()
	if err != nil {
		return &GPUMetricsResponse{
			Available: false,
			Message:   "GPU metrics not available: node-gpu-exporter service not found. Configure monitoring to enable GPU metrics.",
		}, nil
	}

	if len(exporterEndpoints) == 0 {
		return &GPUMetricsResponse{
			Available: false,
			Message:   "GPU metrics not available: no exporter endpoints found",
		}, nil
	}

	// 3. For each pod, find its node and scrape metrics from that node's exporter
	dcgmMetric := mapMetricName(metricName)
	var podMetrics []PodMetrics

	for _, pod := range pods {
		if pod.node == "" || pod.status != "Running" {
			continue
		}

		// Find exporter endpoint on this node
		endpoint, ok := exporterEndpoints[pod.node]
		if !ok {
			continue
		}

		// Scrape metrics from exporter
		values, err := s.scrapeMetricForPod(endpoint, dcgmMetric, pod.name, namespace)
		if err != nil {
			continue
		}

		podMetrics = append(podMetrics, PodMetrics{
			Pod:     pod.name,
			Node:    pod.node,
			GPU:     pod.gpu,
			Metrics: values,
		})
	}

	if len(podMetrics) == 0 {
		return &GPUMetricsResponse{
			Available: true,
			Message:   "no GPU metrics available yet (pods may not be using GPU or exporter not reporting)",
			Pods:      []PodMetrics{},
		}, nil
	}

	return &GPUMetricsResponse{
		Available: true,
		Pods:      podMetrics,
	}, nil
}

// GetAvailableMetrics returns the list of available GPU metric names.
func (s *MetricsService) GetAvailableMetrics() []string {
	return []string{
		"gpu_util",
		"fb_used",
		"sm_occupancy",
		"power_usage",
		"gpu_temp",
	}
}

type podBasicInfo struct {
	name   string
	node   string
	gpu    int
	status string
}

func (s *MetricsService) getJobPods(namespace, jobName, kind string) ([]podBasicInfo, error) {
	var labelSelector string
	if kind == "RayJob" {
		labelSelector = "ray.io/cluster"
	} else {
		labelSelector = fmt.Sprintf("job-name=%s", jobName)
	}

	pods, err := s.adminClient.Typed().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []podBasicInfo
	for _, pod := range pods.Items {
		// For RayJob, filter by job name prefix
		if kind == "RayJob" {
			clusterName := pod.Labels["ray.io/cluster"]
			if len(clusterName) <= len(jobName) || clusterName[:len(jobName)] != jobName {
				continue
			}
		}

		gpu := 0
		for _, c := range pod.Spec.Containers {
			if gpuRes, ok := c.Resources.Limits["nvidia.com/gpu"]; ok {
				gpu += int(gpuRes.Value())
			}
		}

		results = append(results, podBasicInfo{
			name:   pod.Name,
			node:   pod.Spec.NodeName,
			gpu:    gpu,
			status: string(pod.Status.Phase),
		})
	}
	return results, nil
}

// getGPUExporterEndpoints returns a map of nodeName → exporter endpoint URL.
// It looks for the node-gpu-exporter service/endpoints in arms-prom namespace.
func (s *MetricsService) getGPUExporterEndpoints() (map[string]string, error) {
	// Try arms-prom namespace first, then gpu-exporter, then monitoring
	for _, ns := range []string{"arms-prom", "gpu-exporter", "monitoring", "kube-system"} {
		endpoints, err := s.adminClient.Typed().CoreV1().Endpoints(ns).Get(
			context.TODO(), "node-gpu-exporter", metav1.GetOptions{})
		if err != nil {
			continue
		}

		result := make(map[string]string)
		for _, subset := range endpoints.Subsets {
			port := int32(9445)
			for _, p := range subset.Ports {
				if p.Name == "metrics" || p.Port == 9445 {
					port = p.Port
					break
				}
			}
			for _, addr := range subset.Addresses {
				nodeName := ""
				if addr.NodeName != nil {
					nodeName = *addr.NodeName
				} else if addr.TargetRef != nil && addr.TargetRef.Kind == "Pod" {
					// Try to get node from pod
					nodeName = addr.IP // fallback to IP
				}
				if nodeName != "" {
					result[nodeName] = fmt.Sprintf("http://%s:%d", addr.IP, port)
				}
			}
		}
		if len(result) > 0 {
			return result, nil
		}
	}
	return nil, fmt.Errorf("node-gpu-exporter endpoints not found")
}

// scrapeMetricForPod scrapes the DCGM exporter and returns current metric values.
// Since DCGM exporter only returns instant values (not time series), we return a single point.
func (s *MetricsService) scrapeMetricForPod(endpoint, dcgmMetric, podName, namespace string) ([]MetricPoint, error) {
	resp, err := s.httpClient.Get(endpoint + "/metrics")
	if err != nil {
		return nil, fmt.Errorf("scrape %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scrape returned %d", resp.StatusCode)
	}

	// Parse Prometheus text format
	values := parsePrometheusMetric(resp.Body, dcgmMetric, podName, namespace)
	return values, nil
}

// parsePrometheusMetric extracts metric values from Prometheus text format.
// Matches lines like: DCGM_FI_DEV_GPU_UTIL{gpu="0",pod="xxx",namespace="yyy"} 85.0
func parsePrometheusMetric(reader io.Reader, metricName, podName, namespace string) []MetricPoint {
	var points []MetricPoint
	now := time.Now().Unix()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, metricName) {
			continue
		}

		// Check if this line matches the pod/namespace (if labels contain pod info)
		// DCGM exporter may not have pod labels, so we collect all GPU values on the node
		// The caller already filtered by node where the pod runs
		value := extractMetricValue(line)
		if value >= 0 {
			points = append(points, MetricPoint{
				Timestamp: now,
				Value:     value,
			})
		}
	}
	return points
}

func extractMetricValue(line string) float64 {
	// Format: metric_name{labels} value [timestamp]
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return -1
	}
	// The value is the last or second-to-last field
	valueStr := parts[len(parts)-1]
	// Try to parse as float
	v, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		// Maybe there's a timestamp after value
		if len(parts) >= 3 {
			v, err = strconv.ParseFloat(parts[len(parts)-2], 64)
			if err != nil {
				return -1
			}
		} else {
			return -1
		}
	}
	return v
}

// mapMetricName converts our short metric names to DCGM metric names.
func mapMetricName(name string) string {
	switch name {
	case "gpu_util":
		return "DCGM_FI_DEV_GPU_UTIL"
	case "fb_used":
		return "DCGM_FI_DEV_FB_USED"
	case "sm_occupancy":
		return "DCGM_FI_DEV_SM_OCCUPANCY"
	case "power_usage":
		return "DCGM_FI_DEV_POWER_USAGE"
	case "gpu_temp":
		return "DCGM_FI_DEV_GPU_TEMP"
	default:
		return "DCGM_FI_DEV_GPU_UTIL"
	}
}
