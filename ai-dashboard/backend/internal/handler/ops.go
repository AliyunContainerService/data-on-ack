package handler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// eventListLimit bounds the cluster-wide warning-event listing. The list is
	// sorted client-side and truncated to the newest 100 events; the limit only
	// protects the API server round-trip.
	// TODO: follow ListOptions.Continue pagination or a time-windowed query
	// (e.g. via events.k8s.io/v1) for clusters with very long event history.
	eventListLimit = 500
	// workloadListLimit bounds each per-CRD cluster-wide listing below.
	workloadListLimit = 500
)

// OpsHandler provides operations/monitoring APIs for the admin dashboard.
type OpsHandler struct {
	kubeClient *k8s.Client
}

func newOpsHandler(kubeClient *k8s.Client) *OpsHandler {
	return &OpsHandler{kubeClient: kubeClient}
}

func (h *OpsHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// Cluster operations/monitoring views are cluster-admin capabilities.
	rg.GET("/ops/nodes", adminOnly, h.ListNodes)
	rg.GET("/ops/cluster-summary", adminOnly, h.ClusterSummary)
	rg.GET("/ops/events", adminOnly, h.ListWarningEvents)
	rg.GET("/ops/workloads", adminOnly, h.ListWorkloads)
}

// --- Feature 1: Node Condition Monitoring ---

type NodeInfo struct {
	Name          string            `json:"name"`
	IP            string            `json:"ip"`
	Status        string            `json:"status"` // Ready / NotReady
	Roles         string            `json:"roles"`
	Version       string            `json:"version"`
	Runtime       string            `json:"runtime"`
	OS            string            `json:"os"`
	GPUCapacity   int64             `json:"gpuCapacity"`
	GPUAllocatable int64            `json:"gpuAllocatable"`
	GPUHealthy    bool              `json:"gpuHealthy"`
	Conditions    []NodeCondition   `json:"conditions"`
	Labels        map[string]string `json:"labels"`
}

type NodeCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

func (h *OpsHandler) ListNodes(c *gin.Context) {
	nodes, err := h.kubeClient.Typed().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		response.Failed(c, response.CodeK8sError, "list nodes: "+err.Error())
		return
	}

	result := make([]NodeInfo, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		info := NodeInfo{
			Name:    node.Name,
			Version: node.Status.NodeInfo.KubeletVersion,
			Runtime: node.Status.NodeInfo.ContainerRuntimeVersion,
			OS:      node.Status.NodeInfo.OSImage,
			Labels:  node.Labels,
		}

		// IP
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				info.IP = addr.Address
				break
			}
		}

		// Roles
		roles := []string{}
		for k := range node.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				roles = append(roles, strings.TrimPrefix(k, "node-role.kubernetes.io/"))
			}
		}
		if len(roles) == 0 {
			roles = append(roles, "worker")
		}
		info.Roles = strings.Join(roles, ",")

		// GPU capacity
		if gpuCap, ok := node.Status.Capacity["nvidia.com/gpu"]; ok {
			info.GPUCapacity = gpuCap.Value()
		}
		if gpuAlloc, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			info.GPUAllocatable = gpuAlloc.Value()
		}
		info.GPUHealthy = info.GPUCapacity == info.GPUAllocatable

		// Conditions
		info.Status = "NotReady"
		for _, cond := range node.Status.Conditions {
			info.Conditions = append(info.Conditions, NodeCondition{
				Type:    string(cond.Type),
				Status:  string(cond.Status),
				Reason:  cond.Reason,
				Message: cond.Message,
			})
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				info.Status = "Ready"
			}
		}

		result = append(result, info)
	}

	response.OK(c, result)
}

// --- Feature 3: Cluster Resource Summary ---

type ClusterSummary struct {
	CPU    ResourceSummary `json:"cpu"`
	Memory ResourceSummary `json:"memory"`
	GPU    ResourceSummary `json:"gpu"`
	Nodes  NodeSummary     `json:"nodes"`
}

type ResourceSummary struct {
	Capacity  int64 `json:"capacity"`
	Allocated int64 `json:"allocated"`
}

type NodeSummary struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	NotReady int `json:"notReady"`
	GPU      int `json:"gpu"`
}

func (h *OpsHandler) ClusterSummary(c *gin.Context) {
	nodes, err := h.kubeClient.Typed().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		response.Failed(c, response.CodeK8sError, "list nodes: "+err.Error())
		return
	}

	summary := ClusterSummary{}
	summary.Nodes.Total = len(nodes.Items)

	for _, node := range nodes.Items {
		// Capacity
		if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
			summary.CPU.Capacity += cpu.Value()
		}
		if mem, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
			summary.Memory.Capacity += mem.Value() / (1024 * 1024 * 1024) // GiB
		}
		if gpu, ok := node.Status.Allocatable["nvidia.com/gpu"]; ok {
			summary.GPU.Capacity += gpu.Value()
			summary.Nodes.GPU++
		}

		// Node status
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				if cond.Status == corev1.ConditionTrue {
					summary.Nodes.Ready++
				} else {
					summary.Nodes.NotReady++
				}
			}
		}
	}

	// Allocated: sum resource requests from all running pods
	pods, err := h.kubeClient.Typed().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err == nil {
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
					summary.CPU.Allocated += cpu.MilliValue() / 1000
				}
				if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
					summary.Memory.Allocated += mem.Value() / (1024 * 1024 * 1024)
				}
				if gpu, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
					summary.GPU.Allocated += gpu.Value()
				}
			}
		}
	}

	response.OK(c, summary)
}

// --- Feature 7: Warning Events ---

type EventInfo struct {
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Source        string `json:"source"`
	Count         int32  `json:"count"`
	LastTimestamp string `json:"lastTimestamp"`
}

func (h *OpsHandler) ListWarningEvents(c *gin.Context) {
	events, err := h.kubeClient.Typed().CoreV1().Events("").List(context.TODO(), metav1.ListOptions{
		FieldSelector: "type=Warning",
		Limit:         eventListLimit,
	})
	if err != nil {
		response.Failed(c, response.CodeK8sError, "list events: "+err.Error())
		return
	}

	// Sort by last timestamp descending, take top 100
	items := events.Items
	sort.Slice(items, func(i, j int) bool {
		ti := items[i].LastTimestamp.Time
		tj := items[j].LastTimestamp.Time
		if ti.IsZero() {
			ti = items[i].CreationTimestamp.Time
		}
		if tj.IsZero() {
			tj = items[j].CreationTimestamp.Time
		}
		return ti.After(tj)
	})

	limit := 100
	if len(items) > limit {
		items = items[:limit]
	}

	result := make([]EventInfo, 0, len(items))
	for _, ev := range items {
		ts := ev.LastTimestamp.Time
		if ts.IsZero() {
			ts = ev.CreationTimestamp.Time
		}
		result = append(result, EventInfo{
			Namespace:     ev.Namespace,
			Name:          ev.InvolvedObject.Name,
			Kind:          ev.InvolvedObject.Kind,
			Reason:        ev.Reason,
			Message:       ev.Message,
			Source:        ev.Source.Component,
			Count:         ev.Count,
			LastTimestamp:  ts.Format(time.RFC3339),
		})
	}

	response.OK(c, result)
}

// --- Feature 5: Workload Global View ---

type WorkloadInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Owner     string `json:"owner"`
	GPU       int64  `json:"gpu"`
	Created   string `json:"created"`
}

func (h *OpsHandler) ListWorkloads(c *gin.Context) {
	var workloads []WorkloadInfo

	// Notebooks
	nbGVR := schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "notebooks"}
	nbs, err := h.kubeClient.Dynamic().Resource(nbGVR).Namespace("").List(context.TODO(), metav1.ListOptions{Limit: workloadListLimit})
	if err == nil {
		for _, nb := range nbs.Items {
			workloads = append(workloads, parseWorkload(nb, "Notebook"))
		}
	}

	// PyTorchJobs
	ptGVR := schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "pytorchjobs"}
	pts, err := h.kubeClient.Dynamic().Resource(ptGVR).Namespace("").List(context.TODO(), metav1.ListOptions{Limit: workloadListLimit})
	if err == nil {
		for _, pt := range pts.Items {
			workloads = append(workloads, parseWorkload(pt, "PyTorchJob"))
		}
	}

	// TFJobs
	tfGVR := schema.GroupVersionResource{Group: "kubeflow.org", Version: "v1", Resource: "tfjobs"}
	tfs, err := h.kubeClient.Dynamic().Resource(tfGVR).Namespace("").List(context.TODO(), metav1.ListOptions{Limit: workloadListLimit})
	if err == nil {
		for _, tf := range tfs.Items {
			workloads = append(workloads, parseWorkload(tf, "TFJob"))
		}
	}

	// MPIJobs
	mpiGVR := schema.GroupVersionResource{Group: "kubeflow.org", Version: "v2beta1", Resource: "mpijobs"}
	mpis, err := h.kubeClient.Dynamic().Resource(mpiGVR).Namespace("").List(context.TODO(), metav1.ListOptions{Limit: workloadListLimit})
	if err == nil {
		for _, mpi := range mpis.Items {
			workloads = append(workloads, parseWorkload(mpi, "MPIJob"))
		}
	}

	// Sort by creation time descending
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].Created > workloads[j].Created
	})

	response.OK(c, workloads)
}

func parseWorkload(obj unstructured.Unstructured, kind string) WorkloadInfo {
	w := WorkloadInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		Created:   obj.GetCreationTimestamp().Format(time.RFC3339),
	}

	// Owner from labels
	labels := obj.GetLabels()
	if owner, ok := labels["arena.kubeflow.org/console-user"]; ok {
		w.Owner = owner
	} else if user, ok := labels["User"]; ok {
		w.Owner = user
	}

	// Status
	w.Status = getWorkloadStatus(obj)

	// GPU count
	w.GPU = getWorkloadGPU(obj)

	return w
}

func getWorkloadStatus(obj unstructured.Unstructured) string {
	// Check stop annotation for notebooks
	annotations := obj.GetAnnotations()
	if _, stopped := annotations["kubeflow-resource-stopped"]; stopped {
		return "Stopped"
	}

	status, _ := obj.Object["status"].(map[string]interface{})
	if status == nil {
		return "Pending"
	}

	// Check conditions
	conditions, _ := status["conditions"].([]interface{})
	for i := len(conditions) - 1; i >= 0; i-- {
		cond, _ := conditions[i].(map[string]interface{})
		if cond == nil {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		if condStatus == "True" {
			switch condType {
			case "Running":
				return "Running"
			case "Succeeded":
				return "Succeeded"
			case "Failed":
				return "Failed"
			}
		}
	}

	// Notebook readyReplicas
	if ready, ok := status["readyReplicas"].(int64); ok && ready > 0 {
		return "Running"
	}

	return "Pending"
}

func getWorkloadGPU(obj unstructured.Unstructured) int64 {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		return 0
	}

	// Notebook: spec.template.spec.containers[0].resources.limits
	template, _ := spec["template"].(map[string]interface{})
	if template != nil {
		return extractGPUFromPodSpec(template)
	}

	// TrainingJob: check replica specs
	for _, key := range []string{"pytorchReplicaSpecs", "tfReplicaSpecs", "mpiReplicaSpecs"} {
		replicaSpecs, _ := spec[key].(map[string]interface{})
		if replicaSpecs == nil {
			continue
		}
		var total int64
		for _, replicaSpec := range replicaSpecs {
			rs, _ := replicaSpec.(map[string]interface{})
			if rs == nil {
				continue
			}
			replicas := int64(1)
			if r, ok := rs["replicas"].(int64); ok {
				replicas = r
			} else if r, ok := rs["replicas"].(float64); ok {
				replicas = int64(r)
			}
			tmpl, _ := rs["template"].(map[string]interface{})
			gpuPerReplica := extractGPUFromPodSpec(tmpl)
			total += gpuPerReplica * replicas
		}
		if total > 0 {
			return total
		}
	}

	return 0
}

func extractGPUFromPodSpec(template map[string]interface{}) int64 {
	if template == nil {
		return 0
	}
	podSpec, _ := template["spec"].(map[string]interface{})
	if podSpec == nil {
		return 0
	}
	containers, _ := podSpec["containers"].([]interface{})
	for _, c := range containers {
		container, _ := c.(map[string]interface{})
		if container == nil {
			continue
		}
		resources, _ := container["resources"].(map[string]interface{})
		if resources == nil {
			continue
		}
		limits, _ := resources["limits"].(map[string]interface{})
		if limits == nil {
			continue
		}
		if gpu, ok := limits["nvidia.com/gpu"]; ok {
			switch v := gpu.(type) {
			case int64:
				return v
			case float64:
				return int64(v)
			case string:
				var n int64
				fmt.Sscanf(v, "%d", &n)
				return n
			}
		}
	}
	return 0
}
