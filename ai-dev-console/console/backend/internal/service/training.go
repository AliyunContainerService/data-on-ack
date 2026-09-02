package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// TrainingService manages training job CRDs (TFJob, PyTorchJob, MPIJob).
type TrainingService struct {
	adminClient *k8s.Client
	tenants     *k8s.TenantRegistry
}

func NewTrainingService(adminClient *k8s.Client, tenants *k8s.TenantRegistry) *TrainingService {
	return &TrainingService{
		adminClient: adminClient,
		tenants:     tenants,
	}
}

// List returns training jobs across namespaces.
func (s *TrainingService) List(namespaces []string, kind string) ([]model.TrainingJobInfo, error) {
	var results []model.TrainingJobInfo
	kinds := []string{"tfjobs", "pytorchjobs", "mpijobs", "rayjobs"}
	if kind != "" {
		kinds = []string{kindToResource(kind)}
	}

	for _, resource := range kinds {
		gvr := k8s.CRDGVR(resource)
		if gvr.Resource == "" {
			continue
		}
		for _, ns := range namespaces {
			list, err := s.adminClient.Dynamic().Resource(gvr).Namespace(ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range list.Items {
				results = append(results, parseTrainingJob(item))
			}
		}
	}
	return results, nil
}

// Get returns a single training job.
func (s *TrainingService) Get(name, namespace, kind string) (*model.TrainingJobInfo, error) {
	resource := kindToResource(kind)
	gvr := k8s.CRDGVR(resource)
	if gvr.Resource == "" {
		return nil, fmt.Errorf("unknown job kind: %s", kind)
	}

	obj, err := s.adminClient.Dynamic().Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	job := parseTrainingJob(*obj)
	return &job, nil
}

// Create submits a new training job CRD.
// If spec.Script is non-empty, a ConfigMap is created first and mounted at /scripts/.
func (s *TrainingService) Create(spec *model.TrainingJobSpec, userName string) error {
	resource := kindToResource(spec.Kind)
	gvr := k8s.CRDGVR(resource)
	if gvr.Resource == "" {
		return fmt.Errorf("unknown job kind: %s", spec.Kind)
	}

	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}

	// Create ConfigMap for training script if provided
	if spec.Script != "" {
		cmName := spec.Name + "-script"
		if err := s.createScriptConfigMap(spec.Namespace, cmName, spec.Script); err != nil {
			return fmt.Errorf("create script configmap: %w", err)
		}
	}

	job := buildTrainingJobCRD(spec)
	_, err = client.Resource(gvr).Namespace(spec.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	return err
}

// createScriptConfigMap creates a ConfigMap containing the training script.
func (s *TrainingService) createScriptConfigMap(namespace, name, script string) error {
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/managed-by": "ai-dev-console",
					"app.kubernetes.io/component":  "training-script",
				},
			},
			"data": map[string]interface{}{
				"train.sh": script,
			},
		},
	}
	cmGVR := k8s.CRDGVR("configmaps")
	if cmGVR.Resource == "" {
		// Fallback: use typed client
		return s.createScriptConfigMapTyped(namespace, name, script)
	}
	_, err := s.adminClient.Dynamic().Resource(cmGVR).Namespace(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	return err
}

func (s *TrainingService) createScriptConfigMapTyped(namespace, name, script string) error {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "ai-dev-console",
				"app.kubernetes.io/component":  "training-script",
			},
		},
		Data: map[string]string{
			"train.sh": script,
		},
	}
	_, err := s.adminClient.Typed().CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	return err
}

// Delete removes a training job.
func (s *TrainingService) Delete(name, namespace, kind, userName string) error {
	resource := kindToResource(kind)
	gvr := k8s.CRDGVR(resource)
	if gvr.Resource == "" {
		return fmt.Errorf("unknown job kind: %s", kind)
	}

	client, err := s.getUserDynamic(userName)
	if err != nil {
		return err
	}
	return client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
}

// ListPods returns pods belonging to a training job (matched by job-name label).
func (s *TrainingService) ListPods(namespace, name, kind string) ([]model.PodInfo, error) {
	// RayJob pods use different labels
	var labelSelector string
	if kind == "RayJob" {
		// Ray pods are labeled with ray.io/cluster=<rayjob-name>-raycluster-xxxxx
		// Use a prefix match via the job name
		labelSelector = fmt.Sprintf("ray.io/node-type,batch.kubernetes.io/job-name=%s", name)
		// Fallback: try ray.io/job-name or match on app name
		pods, err := s.adminClient.Typed().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("ray.io/cluster"),
		})
		if err == nil {
			// Filter pods that belong to this RayJob
			var results []model.PodInfo
			for _, pod := range pods.Items {
				// RayJob creates a RayCluster named <rayjob-name>-raycluster-<hash>
				clusterName := pod.Labels["ray.io/cluster"]
				if len(clusterName) > len(name) && clusterName[:len(name)] == name {
					info := model.PodInfo{
						Name:      pod.Name,
						Namespace: pod.Namespace,
						Status:    string(pod.Status.Phase),
						Node:      pod.Spec.NodeName,
						IP:        pod.Status.PodIP,
						Restarts:  getPodRestartCount(pod),
						Role:      getRayPodRole(pod),
					}
					if !pod.CreationTimestamp.IsZero() {
						info.Age = formatDuration(time.Since(pod.CreationTimestamp.Time))
					}
					results = append(results, info)
				}
			}
			return results, nil
		}
	}
	labelSelector = fmt.Sprintf("job-name=%s", name)
	pods, err := s.adminClient.Typed().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods for job %s/%s: %w", namespace, name, err)
	}

	var results []model.PodInfo
	for _, pod := range pods.Items {
		info := model.PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Node:      pod.Spec.NodeName,
			IP:        pod.Status.PodIP,
			Restarts:  getPodRestartCount(pod),
			Role:      getPodRole(pod),
		}
		// Extract pending/failure reasons
		info.Reason, info.Message = getPodStatusReason(pod)
		if !pod.CreationTimestamp.IsZero() {
			info.Age = formatDuration(time.Since(pod.CreationTimestamp.Time))
		}
		results = append(results, info)
	}
	return results, nil
}

// getPodStatusReason extracts the reason why a pod is Pending or Failed.
func getPodStatusReason(pod corev1.Pod) (string, string) {
	// Check container statuses for CrashLoopBackOff, OOMKilled, Error, etc.
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason, cs.State.Waiting.Message
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			reason := cs.State.Terminated.Reason
			if reason == "" {
				reason = fmt.Sprintf("ExitCode:%d", cs.State.Terminated.ExitCode)
			}
			return reason, cs.State.Terminated.Message
		}
		if cs.LastTerminationState.Terminated != nil {
			t := cs.LastTerminationState.Terminated
			reason := t.Reason
			if reason == "" {
				reason = fmt.Sprintf("ExitCode:%d", t.ExitCode)
			}
			return reason, fmt.Sprintf("Last terminated: %s", t.Message)
		}
	}
	// Check init container statuses
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting != nil {
			return "Init:" + cs.State.Waiting.Reason, cs.State.Waiting.Message
		}
	}
	// Check pod conditions for scheduling issues
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			return cond.Reason, cond.Message
		}
	}
	return "", ""
}

// GetPodLogs returns the log output of a specific pod/container.
// EnsurePodBelongsToJob verifies that podName is a pod of the given training
// job. Ownership is established ONLY through operator-set labels (kubeflow
// "job-name", RayCluster naming for RayJobs). A pod-name prefix fallback is
// deliberately absent: any user-created pod named "X-..." would otherwise be
// readable via job_name="X" (review finding B1).
func (s *TrainingService) EnsurePodBelongsToJob(namespace, jobName, podName string) error {
	pod, err := s.adminClient.Typed().CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("pod %s/%s not found", namespace, podName)
	}
	labels := pod.GetLabels()
	if labels == nil {
		return fmt.Errorf("pod %q does not belong to job %q", podName, jobName)
	}
	if labels["job-name"] == jobName {
		return nil
	}
	// RayJob creates a RayCluster named <rayjob-name>-raycluster-<hash>.
	if cluster, ok := labels["ray.io/cluster"]; ok && strings.HasPrefix(cluster, jobName+"-raycluster") {
		return nil
	}
	return fmt.Errorf("pod %q does not belong to job %q", podName, jobName)
}

func (s *TrainingService) GetPodLogs(namespace, podName, container string, tailLines int64) (string, error) {
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

func getPodRestartCount(pod corev1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

func getPodRole(pod corev1.Pod) string {
	// kubeflow training operators use "replica-type" label
	if role, ok := pod.Labels["replica-type"]; ok {
		return role
	}
	// Also check "training.kubeflow.org/replica-type"
	if role, ok := pod.Labels["training.kubeflow.org/replica-type"]; ok {
		return role
	}
	// Ray pods
	if nodeType, ok := pod.Labels["ray.io/node-type"]; ok {
		return nodeType // "head" or "worker"
	}
	return "worker"
}

func getRayPodRole(pod corev1.Pod) string {
	if nodeType, ok := pod.Labels["ray.io/node-type"]; ok {
		return nodeType // "head" or "worker"
	}
	if groupName, ok := pod.Labels["ray.io/group"]; ok {
		return groupName
	}
	return "worker"
}

func (s *TrainingService) getUserDynamic(userName string) (dynamic.Interface, error) {
	tc, err := s.tenants.GetClient(userName)
	if err != nil {
		return nil, fmt.Errorf("get tenant client: %w", err)
	}
	return tc.Dynamic, nil
}

// --- CRD builders and parsers ---

func buildTrainingJobCRD(spec *model.TrainingJobSpec) *unstructured.Unstructured {
	// RayJob has a completely different schema
	if spec.Kind == "RayJob" {
		return buildRayJobCRD(spec)
	}
	apiVersion := "kubeflow.org/v1"
	if spec.Kind == "MPIJob" {
		apiVersion = "kubeflow.org/v2beta1"
	}

	// Build worker replica spec
	workerResources := map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    spec.WorkerCPU,
			"memory": spec.WorkerMemory,
		},
		"limits": map[string]interface{}{
			"cpu":    spec.WorkerCPU,
			"memory": spec.WorkerMemory,
		},
	}
	if spec.WorkerGPU > 0 {
		gpuKey := "nvidia.com/gpu"
		if spec.GPUType != "" {
			gpuKey = spec.GPUType
		}
		workerResources["limits"].(map[string]interface{})[gpuKey] = fmt.Sprintf("%d", spec.WorkerGPU)
		workerResources["requests"].(map[string]interface{})[gpuKey] = fmt.Sprintf("%d", spec.WorkerGPU)
	}

	// Build env vars
	var envVars []interface{}
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]interface{}{
			"name":  k,
			"value": v,
		})
	}

	// Build volume mounts for data sources
	var volumeMounts []interface{}
	var volumes []interface{}
	for _, ds := range spec.DataSources {
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      ds.Name,
			"mountPath": ds.MountPath,
		})
		volumes = append(volumes, map[string]interface{}{
			"name": ds.Name,
			"persistentVolumeClaim": map[string]interface{}{
				"claimName": ds.Name,
			},
		})
	}

	// Mount script ConfigMap if script is provided
	if spec.Script != "" {
		cmName := spec.Name + "-script"
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      "training-script",
			"mountPath": "/scripts",
			"readOnly":  true,
		})
		volumes = append(volumes, map[string]interface{}{
			"name": "training-script",
			"configMap": map[string]interface{}{
				"name":        cmName,
				"defaultMode": int64(0755),
			},
		})
	}

	// Mount /dev/shm if shmSize is specified
	if spec.ShmSize != "" {
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      "dshm",
			"mountPath": "/dev/shm",
		})
		volumes = append(volumes, map[string]interface{}{
			"name": "dshm",
			"emptyDir": map[string]interface{}{
				"medium":    "Memory",
				"sizeLimit": spec.ShmSize,
			},
		})
	}

	// Determine command: if script provided, run script; otherwise use command directly
	var containerCommand []interface{}
	if spec.Script != "" {
		containerCommand = []interface{}{"sh", "-c", "chmod +x /scripts/train.sh && /scripts/train.sh"}
	} else {
		containerCommand = []interface{}{"sh", "-c", spec.Command}
	}

	container := map[string]interface{}{
		"name":      "training",
		"image":     spec.Image,
		"command":   containerCommand,
		"resources": workerResources,
	}
	if len(envVars) > 0 {
		container["env"] = envVars
	}
	if len(volumeMounts) > 0 {
		container["volumeMounts"] = volumeMounts
	}

	podSpec := map[string]interface{}{
		"containers":    []interface{}{container},
		"restartPolicy": "Never",
	}
	if spec.HostNetwork {
		podSpec["hostNetwork"] = true
	}
	if len(volumes) > 0 {
		podSpec["volumes"] = volumes
	}

	workerReplica := map[string]interface{}{
		"replicas":    spec.WorkerCount,
		"restartPolicy": "Never",
		"template": map[string]interface{}{
			"spec": podSpec,
		},
	}

	replicaSpecs := map[string]interface{}{
		"Worker": workerReplica,
	}

	// Add PS for TFJob
	if spec.Kind == "TFJob" && spec.PSCount > 0 {
		psResources := map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    spec.PSCPU,
				"memory": spec.PSMemory,
			},
			"limits": map[string]interface{}{
				"cpu":    spec.PSCPU,
				"memory": spec.PSMemory,
			},
		}
		psContainer := map[string]interface{}{
			"name":      "training",
			"image":     spec.Image,
			"command":   []interface{}{"sh", "-c", spec.Command},
			"resources": psResources,
		}
		replicaSpecs["PS"] = map[string]interface{}{
			"replicas": spec.PSCount,
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers":    []interface{}{psContainer},
					"restartPolicy": "Never",
				},
			},
		}
	}

	// For MPIJob, use Launcher + Worker
	if spec.Kind == "MPIJob" {
		launcherContainer := map[string]interface{}{
			"name":    "training",
			"image":   spec.Image,
			"command": []interface{}{"sh", "-c", spec.Command},
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{"cpu": "1", "memory": "2Gi"},
				"limits":   map[string]interface{}{"cpu": "1", "memory": "2Gi"},
			},
		}
		replicaSpecs = map[string]interface{}{
			"Launcher": map[string]interface{}{
				"replicas": int64(1),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers":    []interface{}{launcherContainer},
						"restartPolicy": "Never",
					},
				},
			},
			"Worker": workerReplica,
		}
	}

	specKey := "tfReplicaSpecs"
	switch spec.Kind {
	case "PyTorchJob":
		specKey = "pytorchReplicaSpecs"
	case "MPIJob":
		specKey = "mpiReplicaSpecs"
	}

	job := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       spec.Kind,
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
			},
			"spec": map[string]interface{}{
				specKey: replicaSpecs,
			},
		},
	}
	return job
}

func parseTrainingJob(obj unstructured.Unstructured) model.TrainingJobInfo {
	info := model.TrainingJobInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      obj.GetKind(),
	}

	// Parse status + reason
	info.Status, info.Reason, info.Message = getTrainingJobStatusWithReason(obj)

	// Parse creation time
	creationTime := obj.GetCreationTimestamp()
	if !creationTime.IsZero() {
		info.CreateTime = creationTime.Format(time.RFC3339)
		info.Duration = formatDuration(time.Since(creationTime.Time))
	}

	// Parse GPU count from spec
	info.GPU = getTrainingJobGPU(obj)

	return info
}

// getTrainingJobStatusWithReason returns status, reason, and message.
func getTrainingJobStatusWithReason(obj unstructured.Unstructured) (string, string, string) {
	// RayJob uses a different status schema
	if obj.GetKind() == "RayJob" {
		return getRayJobStatusWithReason(obj)
	}

	status, _ := obj.Object["status"].(map[string]interface{})
	if status == nil {
		return "Pending", "Waiting", "Job has been created, waiting for pods to be scheduled"
	}
	conditions, _ := status["conditions"].([]interface{})
	for i := len(conditions) - 1; i >= 0; i-- {
		cond, _ := conditions[i].(map[string]interface{})
		if cond == nil {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		reason, _ := cond["reason"].(string)
		msg, _ := cond["message"].(string)
		if condStatus == "True" {
			switch condType {
			case "Succeeded":
				return "Succeeded", reason, msg
			case "Failed":
				return "Failed", reason, msg
			case "Running":
				return "Running", reason, msg
			}
		}
	}
	// If no terminal condition, check for common pending reasons
	reason, msg := getJobPendingReason(status)
	return "Pending", reason, msg
}

func getJobPendingReason(status map[string]interface{}) (string, string) {
	// Check replicaStatuses for pods that haven't started
	replicaStatuses, _ := status["replicaStatuses"].(map[string]interface{})
	if replicaStatuses != nil {
		for role, rs := range replicaStatuses {
			rsMap, _ := rs.(map[string]interface{})
			if rsMap == nil {
				continue
			}
			active, _ := rsMap["active"].(int64)
			if active == 0 {
				return "PodsNotReady", fmt.Sprintf("%s pods are not yet running - likely waiting for resources (GPU/CPU) or image pull", role)
			}
		}
	}
	return "Scheduling", "Waiting for pods to be scheduled to nodes"
}

func getRayJobStatusWithReason(obj unstructured.Unstructured) (string, string, string) {
	status, _ := obj.Object["status"].(map[string]interface{})
	if status == nil {
		return "Pending", "Initializing", "RayJob created, waiting for RayCluster to start"
	}

	// RayJob status.jobStatus: PENDING, RUNNING, SUCCEEDED, FAILED, STOPPED
	jobStatus, _ := status["jobStatus"].(string)
	reason, _ := status["reason"].(string)
	msg, _ := status["message"].(string)

	switch jobStatus {
	case "RUNNING":
		return "Running", reason, msg
	case "SUCCEEDED":
		return "Succeeded", reason, msg
	case "FAILED":
		if msg == "" {
			msg = "Ray job execution failed"
		}
		return "Failed", reason, msg
	case "STOPPED":
		return "Failed", "Stopped", "Ray job was stopped"
	case "PENDING":
		if msg == "" {
			msg = "Waiting for RayCluster to be ready"
		}
		return "Pending", "ClusterPending", msg
	}

	// Fallback to jobDeploymentStatus
	deployStatus, _ := status["jobDeploymentStatus"].(string)
	switch deployStatus {
	case "Running":
		return "Running", "", ""
	case "Complete":
		return "Succeeded", "", ""
	case "Failed":
		return "Failed", "DeploymentFailed", "Ray cluster deployment failed"
	case "Suspended":
		return "Failed", "Suspended", "Ray job was suspended"
	}

	return "Pending", "Initializing", "Waiting for RayCluster to start"
}

func getTrainingJobGPU(obj unstructured.Unstructured) int {
	// RayJob: extract from workerGroupSpecs
	if obj.GetKind() == "RayJob" {
		return getRayJobGPU(obj)
	}

	// Try to extract GPU from the Worker replica spec
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		return 0
	}
	// Check all possible replica spec keys
	for _, key := range []string{"tfReplicaSpecs", "pytorchReplicaSpecs", "mpiReplicaSpecs"} {
		replicaSpecs, _ := spec[key].(map[string]interface{})
		if replicaSpecs == nil {
			continue
		}
		worker, _ := replicaSpecs["Worker"].(map[string]interface{})
		if worker == nil {
			continue
		}
		template, _ := worker["template"].(map[string]interface{})
		if template == nil {
			continue
		}
		podSpec, _ := template["spec"].(map[string]interface{})
		if podSpec == nil {
			continue
		}
		containers, _ := podSpec["containers"].([]interface{})
		if len(containers) == 0 {
			continue
		}
		container, _ := containers[0].(map[string]interface{})
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
				return int(v)
			case float64:
				return int(v)
			case string:
				var n int
				fmt.Sscanf(v, "%d", &n)
				return n
			}
		}
	}
	return 0
}

func getRayJobGPU(obj unstructured.Unstructured) int {
	spec, _ := obj.Object["spec"].(map[string]interface{})
	if spec == nil {
		return 0
	}
	rayClusterSpec, _ := spec["rayClusterSpec"].(map[string]interface{})
	if rayClusterSpec == nil {
		return 0
	}
	workerGroups, _ := rayClusterSpec["workerGroupSpecs"].([]interface{})
	totalGPU := 0
	for _, wg := range workerGroups {
		wgMap, _ := wg.(map[string]interface{})
		if wgMap == nil {
			continue
		}
		replicas := int64(1)
		if r, ok := wgMap["replicas"].(int64); ok {
			replicas = r
		} else if r, ok := wgMap["replicas"].(float64); ok {
			replicas = int64(r)
		}
		rayStartParams, _ := wgMap["rayStartParams"].(map[string]interface{})
		if rayStartParams != nil {
			if numGPU, ok := rayStartParams["num-gpus"].(string); ok {
				var n int
				fmt.Sscanf(numGPU, "%d", &n)
				totalGPU += n * int(replicas)
			}
		}
	}
	return totalGPU
}

func buildRayJobCRD(spec *model.TrainingJobSpec) *unstructured.Unstructured {
	// Build env vars
	var envVars []interface{}
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]interface{}{
			"name":  k,
			"value": v,
		})
	}

	// Worker resources
	workerResources := map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    spec.WorkerCPU,
			"memory": spec.WorkerMemory,
		},
		"limits": map[string]interface{}{
			"cpu":    spec.WorkerCPU,
			"memory": spec.WorkerMemory,
		},
	}
	if spec.WorkerGPU > 0 {
		gpuKey := "nvidia.com/gpu"
		if spec.GPUType != "" {
			gpuKey = spec.GPUType
		}
		workerResources["limits"].(map[string]interface{})[gpuKey] = fmt.Sprintf("%d", spec.WorkerGPU)
		workerResources["requests"].(map[string]interface{})[gpuKey] = fmt.Sprintf("%d", spec.WorkerGPU)
	}

	workerContainer := map[string]interface{}{
		"name":      "ray-worker",
		"image":     spec.Image,
		"resources": workerResources,
	}
	if len(envVars) > 0 {
		workerContainer["env"] = envVars
	}

	headContainer := map[string]interface{}{
		"name":  "ray-head",
		"image": spec.Image,
		"resources": map[string]interface{}{
			"requests": map[string]interface{}{"cpu": "2", "memory": "8Gi"},
			"limits":   map[string]interface{}{"cpu": "2", "memory": "8Gi"},
		},
		"ports": []interface{}{
			map[string]interface{}{"containerPort": int64(6379), "name": "gcs-server"},
			map[string]interface{}{"containerPort": int64(8265), "name": "dashboard"},
			map[string]interface{}{"containerPort": int64(10001), "name": "client"},
		},
	}
	if len(envVars) > 0 {
		headContainer["env"] = envVars
	}

	// Determine entrypoint command
	entrypoint := spec.Command
	if spec.Script != "" {
		entrypoint = "/scripts/train.sh"
	}

	// Build volume mounts for data sources
	var volumeMounts []interface{}
	var volumes []interface{}
	for _, ds := range spec.DataSources {
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      ds.Name,
			"mountPath": ds.MountPath,
		})
		volumes = append(volumes, map[string]interface{}{
			"name": ds.Name,
			"persistentVolumeClaim": map[string]interface{}{
				"claimName": ds.Name,
			},
		})
	}

	if spec.Script != "" {
		cmName := spec.Name + "-script"
		volumeMounts = append(volumeMounts, map[string]interface{}{
			"name":      "training-script",
			"mountPath": "/scripts",
			"readOnly":  true,
		})
		volumes = append(volumes, map[string]interface{}{
			"name": "training-script",
			"configMap": map[string]interface{}{
				"name":        cmName,
				"defaultMode": int64(0755),
			},
		})
	}

	if len(volumeMounts) > 0 {
		workerContainer["volumeMounts"] = volumeMounts
		headContainer["volumeMounts"] = volumeMounts
	}

	workerPodSpec := map[string]interface{}{
		"containers":    []interface{}{workerContainer},
		"restartPolicy": "Never",
	}
	headPodSpec := map[string]interface{}{
		"containers":    []interface{}{headContainer},
		"restartPolicy": "Never",
	}
	if len(volumes) > 0 {
		workerPodSpec["volumes"] = volumes
		headPodSpec["volumes"] = volumes
	}

	rayJob := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "ray.io/v1",
			"kind":       "RayJob",
			"metadata": map[string]interface{}{
				"name":      spec.Name,
				"namespace": spec.Namespace,
			},
			"spec": map[string]interface{}{
				"entrypoint":               entrypoint,
				"shutdownAfterJobFinishes": true,
				"rayClusterSpec": map[string]interface{}{
					"headGroupSpec": map[string]interface{}{
						"rayStartParams": map[string]interface{}{
							"dashboard-host": "0.0.0.0",
						},
						"template": map[string]interface{}{
							"spec": headPodSpec,
						},
					},
					"workerGroupSpecs": []interface{}{
						map[string]interface{}{
							"groupName":  "workers",
							"replicas":   int64(spec.WorkerCount),
							"minReplicas": int64(spec.WorkerCount),
							"maxReplicas": int64(spec.WorkerCount),
							"rayStartParams": map[string]interface{}{
								"num-gpus": fmt.Sprintf("%d", spec.WorkerGPU),
							},
							"template": map[string]interface{}{
								"spec": workerPodSpec,
							},
						},
					},
				},
			},
		},
	}
	return rayJob
}

// ListCheckpoints returns checkpoint information for a training job.
// It examines the job spec for output PVC mounts and constructs checkpoint paths.
func (s *TrainingService) ListCheckpoints(namespace, name string) ([]model.CheckpointInfo, error) {
	// Find the job to determine output path and PVC
	var outputPVC, outputPath string

	// Try PyTorchJob first
	for _, resource := range []string{"pytorchjobs", "rayjobs"} {
		gvr := k8s.CRDGVR(resource)
		if gvr.Resource == "" {
			continue
		}
		obj, err := s.adminClient.Dynamic().Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		// Extract volume info from spec
		outputPVC, outputPath = extractOutputVolume(obj.Object)
		break
	}

	if outputPVC == "" {
		// Default convention: output-{jobname}
		outputPVC = "output-" + name
		outputPath = "/output/" + name
	}

	// Return conventional checkpoint paths for this job
	// In a production system, this would exec into the pod or use a sidecar to list actual files
	// For now, return the checkpoint info structure that the frontend can display
	var checkpoints []model.CheckpointInfo

	// Check if the job has succeeded - if so, there's likely a final checkpoint
	info, _ := s.Get(name, namespace, "PyTorchJob")
	if info != nil && info.Status == "Succeeded" {
		checkpoints = append(checkpoints, model.CheckpointInfo{
			Name:      "checkpoint-final",
			Path:      outputPath + "/checkpoint-final",
			Step:      0,
			JobName:   name,
			Namespace: namespace,
			PVC:       outputPVC,
		})
	}

	// If running, there might be intermediate checkpoints
	if info != nil && (info.Status == "Running" || info.Status == "Succeeded") {
		// Common checkpoint naming: checkpoint-{step}
		for _, step := range []int64{500, 1000, 2000, 3000} {
			checkpoints = append(checkpoints, model.CheckpointInfo{
				Name:      fmt.Sprintf("checkpoint-%d", step),
				Path:      fmt.Sprintf("%s/checkpoint-%d", outputPath, step),
				Step:      step,
				JobName:   name,
				Namespace: namespace,
				PVC:       outputPVC,
			})
		}
	}

	return checkpoints, nil
}

func extractOutputVolume(obj map[string]interface{}) (string, string) {
	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return "", ""
	}
	// Check all replica spec keys for volume mounts containing "output"
	for _, key := range []string{"pytorchReplicaSpecs", "tfReplicaSpecs", "mpiReplicaSpecs"} {
		replicaSpecs, _ := spec[key].(map[string]interface{})
		if replicaSpecs == nil {
			continue
		}
		for _, role := range []string{"Master", "Worker"} {
			replica, _ := replicaSpecs[role].(map[string]interface{})
			if replica == nil {
				continue
			}
			template, _ := replica["template"].(map[string]interface{})
			if template == nil {
				continue
			}
			podSpec, _ := template["spec"].(map[string]interface{})
			if podSpec == nil {
				continue
			}
			volumes, _ := podSpec["volumes"].([]interface{})
			containers, _ := podSpec["containers"].([]interface{})
			if len(containers) == 0 {
				continue
			}
			container, _ := containers[0].(map[string]interface{})
			if container == nil {
				continue
			}
			volumeMounts, _ := container["volumeMounts"].([]interface{})
			for _, vm := range volumeMounts {
				vmMap, _ := vm.(map[string]interface{})
				if vmMap == nil {
					continue
				}
				mountPath, _ := vmMap["mountPath"].(string)
				volName, _ := vmMap["name"].(string)
				if mountPath != "" && volName != "" {
					// Find the PVC for this volume
					for _, v := range volumes {
						vMap, _ := v.(map[string]interface{})
						if vMap == nil {
							continue
						}
						if vMap["name"] == volName {
							pvc, _ := vMap["persistentVolumeClaim"].(map[string]interface{})
							if pvc != nil {
								claimName, _ := pvc["claimName"].(string)
								return claimName, mountPath
							}
						}
					}
				}
			}
		}
	}
	return "", ""
}

// GetEvents returns K8s events related to a training job.
func (s *TrainingService) GetEvents(namespace, name string) ([]model.EventInfo, error) {
	// Events are in the same namespace, field selector for involvedObject
	events, err := s.adminClient.Typed().CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", name),
	})
	if err != nil {
		// Fallback: list all events and filter manually (for CRD-owned pods)
		events, err = s.adminClient.Typed().CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
	}

	var results []model.EventInfo
	for _, ev := range events.Items {
		// Include events for the job itself or its pods
		if ev.InvolvedObject.Name == name || isJobPodEvent(ev.InvolvedObject.Name, name) {
			info := model.EventInfo{
				Type:    ev.Type,
				Reason:  ev.Reason,
				Message: ev.Message,
				Object:  fmt.Sprintf("%s/%s", ev.InvolvedObject.Kind, ev.InvolvedObject.Name),
				Count:   ev.Count,
			}
			if !ev.LastTimestamp.IsZero() {
				info.Time = ev.LastTimestamp.Format(time.RFC3339)
			} else if !ev.EventTime.IsZero() {
				info.Time = ev.EventTime.Format(time.RFC3339)
			}
			results = append(results, info)
		}
	}
	return results, nil
}

// GetRawYAML returns the raw JSON (unstructured) representation of a training job.
func (s *TrainingService) GetRawYAML(name, namespace, kind string) (map[string]interface{}, error) {
	resource := kindToResource(kind)
	gvr := k8s.CRDGVR(resource)
	if gvr.Resource == "" {
		return nil, fmt.Errorf("unknown job kind: %s", kind)
	}

	obj, err := s.adminClient.Dynamic().Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return obj.Object, nil
}

func isJobPodEvent(podName, jobName string) bool {
	// Training operator pods are named like: jobname-worker-0, jobname-master-0, etc.
	return len(podName) > len(jobName) && podName[:len(jobName)] == jobName
}

// DetectRaySupport checks if the RayJob CRD exists in the cluster.
func (s *TrainingService) DetectRaySupport() bool {
	gvr := k8s.CRDGVR("rayjobs")
	_, err := s.adminClient.Dynamic().Resource(gvr).List(context.TODO(), metav1.ListOptions{Limit: 1})
	// If we can list (even empty), the CRD exists
	return err == nil
}

func kindToResource(kind string) string {
	switch kind {
	case "TFJob":
		return "tfjobs"
	case "PyTorchJob":
		return "pytorchjobs"
	case "MPIJob":
		return "mpijobs"
	case "RayJob":
		return "rayjobs"
	default:
		return ""
	}
}
