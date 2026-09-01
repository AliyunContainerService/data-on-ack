package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// podListLimit bounds cluster-wide pod listings so a single API call cannot
// stream an unbounded number of objects.
// TODO: for very large clusters, follow ListOptions.Continue pagination (or a
// metrics-based data source) instead of truncating.
const podListLimit = 2000

// CostService provides GPU usage and cost tracking.
type CostService struct {
	kubeClient *k8s.Client
}

func NewCostService(kubeClient *k8s.Client) *CostService {
	return &CostService{kubeClient: kubeClient}
}

// QuotaUsageInfo represents resource usage for a quota group.
type QuotaUsageInfo struct {
	Name       string            `json:"name"`
	Min        map[string]string `json:"min"`
	Max        map[string]string `json:"max"`
	Used       map[string]string `json:"used"`
	Namespaces []string          `json:"namespaces"`
	Children   []QuotaUsageInfo  `json:"children,omitempty"`
}

// GPUUsageRecord represents GPU usage for a user/team over a time period.
type GPUUsageRecord struct {
	User      string  `json:"user"`
	Namespace string  `json:"namespace"`
	GPUHours  float64 `json:"gpuHours"`
	JobCount  int     `json:"jobCount"`
}

// CostSummary is the response for the cost dashboard API.
type CostSummary struct {
	QuotaTree  *QuotaUsageInfo   `json:"quotaTree,omitempty"`
	GPUUsage   []GPUUsageRecord  `json:"gpuUsage"`
	TotalGPU   float64           `json:"totalGpuHours"`
	TimeRange  string            `json:"timeRange"`
}

// GetQuotaUsage returns the quota tree with current usage per group.
func (s *CostService) GetQuotaUsage() (*QuotaUsageInfo, error) {
	// Read ElasticQuotaTree from kube-system
	eqTreeGVR := schema.GroupVersionResource{
		Group: "scheduling.sigs.k8s.io", Version: "v1beta1", Resource: "elasticquotatrees",
	}
	trees, err := s.kubeClient.Dynamic().Resource(eqTreeGVR).Namespace("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list elastic quota trees: %w", err)
	}
	if len(trees.Items) == 0 {
		return nil, fmt.Errorf("no ElasticQuotaTree found")
	}

	tree := trees.Items[0]
	spec, _ := tree.Object["spec"].(map[string]interface{})
	if spec == nil {
		return nil, fmt.Errorf("quota tree has no spec")
	}
	root, _ := spec["root"].(map[string]interface{})
	if root == nil {
		return nil, fmt.Errorf("quota tree has no root")
	}

	// Get current pod resource usage per namespace
	nsUsage := s.getNamespaceResourceUsage()

	// Build usage tree
	usage := s.buildQuotaUsageTree(root, nsUsage)
	return usage, nil
}

// GetGPUUsage calculates GPU-hours per user over the given time range.
func (s *CostService) GetGPUUsage(timeRange string) (*CostSummary, error) {
	duration := parseDuration(timeRange)
	cutoff := time.Now().Add(-duration)

	// List completed and running training pods across all namespaces.
	// Bounded by podListLimit: the time-window filter below is computed from
	// pod timestamps, which cannot be expressed as a field selector, so very
	// large clusters may be truncated (see podListLimit comment).
	allPods, err := s.kubeClient.Typed().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		Limit: podListLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	// Calculate GPU-hours per user
	userUsage := make(map[string]*GPUUsageRecord)
	var totalGPUHours float64

	for _, pod := range allPods.Items {
		// Only count pods with GPU requests
		gpuCount := getPodGPUCount(pod)
		if gpuCount == 0 {
			continue
		}

		// Filter by time range
		startTime := pod.CreationTimestamp.Time
		if startTime.Before(cutoff) && !isPodRunning(pod) {
			continue
		}

		// Calculate run duration
		runDuration := calculatePodRunDuration(pod, cutoff)
		if runDuration <= 0 {
			continue
		}

		gpuHours := float64(gpuCount) * runDuration.Hours()
		totalGPUHours += gpuHours

		// Get user from labels
		user := getPodUser(pod)
		key := user + "/" + pod.Namespace
		if _, ok := userUsage[key]; !ok {
			userUsage[key] = &GPUUsageRecord{
				User:      user,
				Namespace: pod.Namespace,
			}
		}
		userUsage[key].GPUHours += gpuHours
		userUsage[key].JobCount++
	}

	// Convert to sorted slice
	records := make([]GPUUsageRecord, 0, len(userUsage))
	for _, r := range userUsage {
		r.GPUHours = roundFloat(r.GPUHours, 2)
		records = append(records, *r)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].GPUHours > records[j].GPUHours
	})

	// Get quota tree usage
	quotaUsage, _ := s.GetQuotaUsage()

	return &CostSummary{
		QuotaTree:  quotaUsage,
		GPUUsage:   records,
		TotalGPU:   roundFloat(totalGPUHours, 2),
		TimeRange:  timeRange,
	}, nil
}

// UpdateQuotaNode updates a quota node's min/max values.
func (s *CostService) UpdateQuotaNode(nodeName string, min, max map[string]string) error {
	eqTreeGVR := schema.GroupVersionResource{
		Group: "scheduling.sigs.k8s.io", Version: "v1beta1", Resource: "elasticquotatrees",
	}
	trees, err := s.kubeClient.Dynamic().Resource(eqTreeGVR).Namespace("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list elastic quota trees: %w", err)
	}
	if len(trees.Items) == 0 {
		return fmt.Errorf("no ElasticQuotaTree found")
	}

	tree := &trees.Items[0]
	spec, _ := tree.Object["spec"].(map[string]interface{})
	if spec == nil {
		return fmt.Errorf("quota tree has no spec")
	}
	root, _ := spec["root"].(map[string]interface{})
	if root == nil {
		return fmt.Errorf("quota tree has no root")
	}

	// Find and update the node
	if !updateNodeInTree(root, nodeName, min, max) {
		return fmt.Errorf("node %s not found in quota tree", nodeName)
	}

	_, err = s.kubeClient.Dynamic().Resource(eqTreeGVR).Namespace("kube-system").Update(context.TODO(), tree, metav1.UpdateOptions{})
	return err
}

func (s *CostService) getNamespaceResourceUsage() map[string]map[string]int64 {
	nsUsage := make(map[string]map[string]int64)

	pods, err := s.kubeClient.Typed().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "status.phase=Running",
		Limit:         podListLimit,
	})
	if err != nil {
		return nsUsage
	}

	for _, pod := range pods.Items {
		ns := pod.Namespace
		if _, ok := nsUsage[ns]; !ok {
			nsUsage[ns] = map[string]int64{"cpu": 0, "memory": 0, "gpu": 0}
		}
		for _, c := range pod.Spec.Containers {
			if cpu, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
				nsUsage[ns]["cpu"] += cpu.MilliValue()
			}
			if mem, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
				nsUsage[ns]["memory"] += mem.Value() / (1024 * 1024 * 1024)
			}
			if gpu, ok := c.Resources.Requests["nvidia.com/gpu"]; ok {
				nsUsage[ns]["gpu"] += gpu.Value()
			}
		}
	}
	return nsUsage
}

func (s *CostService) buildQuotaUsageTree(node map[string]interface{}, nsUsage map[string]map[string]int64) *QuotaUsageInfo {
	name, _ := node["name"].(string)
	info := &QuotaUsageInfo{
		Name: name,
		Min:  extractResourceMap(node, "min"),
		Max:  extractResourceMap(node, "max"),
		Used: make(map[string]string),
	}

	// Get namespaces
	if nsList, ok := node["namespaces"].([]interface{}); ok {
		for _, ns := range nsList {
			if nsStr, ok := ns.(string); ok {
				info.Namespaces = append(info.Namespaces, nsStr)
			}
		}
	}

	// Calculate used from namespace usage
	var cpuUsed, memUsed, gpuUsed int64
	for _, ns := range info.Namespaces {
		if usage, ok := nsUsage[ns]; ok {
			cpuUsed += usage["cpu"]
			memUsed += usage["memory"]
			gpuUsed += usage["gpu"]
		}
	}

	// Process children
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			childMap, _ := child.(map[string]interface{})
			if childMap == nil {
				continue
			}
			childInfo := s.buildQuotaUsageTree(childMap, nsUsage)
			info.Children = append(info.Children, *childInfo)

			// Aggregate child usage
			for _, cns := range childInfo.Namespaces {
				if usage, ok := nsUsage[cns]; ok {
					cpuUsed += usage["cpu"]
					memUsed += usage["memory"]
					gpuUsed += usage["gpu"]
				}
			}
		}
	}

	info.Used["cpu"] = fmt.Sprintf("%dm", cpuUsed)
	info.Used["memory"] = fmt.Sprintf("%dGi", memUsed)
	info.Used["nvidia.com/gpu"] = fmt.Sprintf("%d", gpuUsed)

	return info
}

func extractResourceMap(node map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	m, _ := node[key].(map[string]interface{})
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

func updateNodeInTree(node map[string]interface{}, targetName string, min, max map[string]string) bool {
	name, _ := node["name"].(string)
	if name == targetName {
		if min != nil {
			minMap := make(map[string]interface{})
			for k, v := range min {
				minMap[k] = v
			}
			node["min"] = minMap
		}
		if max != nil {
			maxMap := make(map[string]interface{})
			for k, v := range max {
				maxMap[k] = v
			}
			node["max"] = maxMap
		}
		return true
	}
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			childMap, _ := child.(map[string]interface{})
			if childMap != nil && updateNodeInTree(childMap, targetName, min, max) {
				return true
			}
		}
	}
	return false
}

func getPodGPUCount(pod corev1.Pod) int64 {
	var total int64
	for _, c := range pod.Spec.Containers {
		if gpu, ok := c.Resources.Requests["nvidia.com/gpu"]; ok {
			total += gpu.Value()
		}
	}
	return total
}

func isPodRunning(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

func calculatePodRunDuration(pod corev1.Pod, cutoff time.Time) time.Duration {
	start := pod.CreationTimestamp.Time
	if start.Before(cutoff) {
		start = cutoff
	}

	var end time.Time
	if pod.Status.Phase == corev1.PodRunning {
		end = time.Now()
	} else {
		// Use the last container termination time
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Terminated != nil {
				t := cs.State.Terminated.FinishedAt.Time
				if t.After(end) {
					end = t
				}
			}
		}
		if end.IsZero() {
			end = time.Now()
		}
	}

	if end.Before(start) {
		return 0
	}
	return end.Sub(start)
}

func getPodUser(pod corev1.Pod) string {
	// Check common user labels
	for _, key := range []string{
		"arena.kubeflow.org/console-user",
		"User",
		"user",
		"app.kubernetes.io/created-by",
	} {
		if v, ok := pod.Labels[key]; ok && v != "" {
			return v
		}
	}
	return "unknown"
}

func parseDuration(timeRange string) time.Duration {
	switch timeRange {
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

func roundFloat(val float64, precision int) float64 {
	p := 1.0
	for i := 0; i < precision; i++ {
		p *= 10
	}
	return float64(int(val*p+0.5)) / p
}
