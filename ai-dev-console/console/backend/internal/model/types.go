package model

// --- Session / Auth types ---

type UserInfo struct {
	Aid        string   `json:"aid"`        // Alibaba Cloud primary account ID
	Uid        string   `json:"uid"`        // RAM sub-account ID
	Name       string   `json:"name"`       // Normalized login name
	LoginName  string   `json:"loginName"`  // Original login name
	Role       string   `json:"role"`       // "admin" or "researcher"
	Namespaces []string `json:"namespaces"` // Bound K8s namespaces
	Token      string   `json:"token"`      // K8s SA token
}

type OAuthApp struct {
	AppID     string `json:"appId"`
	AppSecret string `json:"appSecret"`
	AppName   string `json:"appName"`
}

type RamUserInfo struct {
	Sub       string `json:"sub"`
	Uid       string `json:"uid"`
	LoginName string `json:"login_name"`
	Aid       string `json:"aid"`
	Upn       string `json:"upn"` // empty for primary account (admin)
	Name      string `json:"name"`
}

// --- Notebook types ---

type NotebookSpec struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Image     string            `json:"image"`
	CPU       string            `json:"cpu"`
	Memory    string            `json:"memory"`
	GPU       int               `json:"gpu"`
	GPUType   string            `json:"gpuType,omitempty"`
	Storage   string            `json:"storage,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

type NotebookInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Image     string `json:"image"`
	Status    string `json:"status"` // Running, Stopped, Pending, Failed
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
	GPU       string `json:"gpu"`
	Age       string `json:"age"`
	URL       string `json:"url,omitempty"`
}

type NotebookSSHInfo struct {
	PodName     string `json:"podName"`
	Namespace   string `json:"namespace"`
	Node        string `json:"node"`
	IP          string `json:"ip"`
	PortForward string `json:"portForward"` // kubectl port-forward command
	ExecCommand string `json:"execCommand"` // kubectl exec command
}

type NotebookResizeSpec struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	GPU    int    `json:"gpu"`
}

// --- Training Job types ---

type TrainingJobSpec struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	Kind          string            `json:"kind"` // TFJob, PyTorchJob, MPIJob, RayJob
	Image         string            `json:"image"`
	Command       string            `json:"command"`
	Script        string            `json:"script,omitempty"` // Multi-line script content → ConfigMap mount
	WorkerCount   int32             `json:"workerCount"`
	WorkerCPU     string            `json:"workerCpu"`
	WorkerMemory  string            `json:"workerMemory"`
	WorkerGPU     int               `json:"workerGpu"`
	PSCount       int32             `json:"psCount,omitempty"`
	PSCPU         string            `json:"psCpu,omitempty"`
	PSMemory      string            `json:"psMemory,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	DataSources   []DataSource      `json:"dataSources,omitempty"`
	GPUType       string            `json:"gpuType,omitempty"`
	ShmSize       string            `json:"shmSize,omitempty"`       // /dev/shm size
	EnableRDMA    bool              `json:"enableRdma,omitempty"`    // inject RDMA env vars
	HostNetwork   bool              `json:"hostNetwork,omitempty"`   // enable host networking
	Queue         string            `json:"queue,omitempty"`         // ElasticQuota queue
	Priority      string            `json:"priority,omitempty"`      // scheduling priority
}

type DataSource struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}

type TrainingJobInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`  // Why it's in this status (e.g., Unschedulable, OOMKilled)
	Message    string `json:"message,omitempty"` // Detailed human-readable message
	Duration   string `json:"duration"`
	CreateTime string `json:"createTime"`
	GPU        int    `json:"gpu"`
}

// EventInfo represents a K8s event.
type EventInfo struct {
	Type    string `json:"type"`    // Normal, Warning
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Object  string `json:"object"`  // Kind/Name
	Time    string `json:"time"`
	Count   int32  `json:"count"`
}

// PodInfo represents a pod belonging to a training job.
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`  // Why pod is Pending/Failed
	Message   string `json:"message,omitempty"` // Detailed reason message
	Node      string `json:"node"`
	IP        string `json:"ip"`
	Restarts  int32  `json:"restarts"`
	Role      string `json:"role"` // master, worker, ps, launcher
	Age       string `json:"age"`
}

// --- Serving/Inference types ---

type ServingSpec struct {
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Image          string            `json:"image"`
	Command        string            `json:"command,omitempty"`
	Replicas       int32             `json:"replicas"`
	CPU            string            `json:"cpu"`
	Memory         string            `json:"memory"`
	GPU            int               `json:"gpu"`
	ModelPath      string            `json:"modelPath,omitempty"`
	Port           int32             `json:"port"`
	Env            map[string]string `json:"env,omitempty"`
	Framework      string            `json:"framework,omitempty"` // tensorflow, pytorch, custom
}

type ServingInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Status     string `json:"status"`
	Replicas   string `json:"replicas"` // "ready/desired"
	Endpoint   string `json:"endpoint"`
	CreateTime string `json:"createTime"`
	Framework  string `json:"framework"`
}

// --- Model registry types ---

type ModelInfo struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Version      string `json:"version"`
	Framework    string `json:"framework"` // pytorch, tensorflow, onnx, safetensors
	Path         string `json:"path"`      // PVC path or OSS/S3 URI
	Source       string `json:"source"`    // huggingface, modelscope, custom
	Size         string `json:"size"`      // e.g. "7B", "14B", "72B"
	RegisterTime string `json:"registerTime"`
}

type ModelRegisterSpec struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
	Framework string `json:"framework"`
	Path      string `json:"path"`
	Source    string `json:"source"`
	Size      string `json:"size"`
}

// --- Checkpoint types ---

type CheckpointInfo struct {
	Name      string `json:"name"`      // e.g. "checkpoint-1000"
	Path      string `json:"path"`      // Full path in PVC
	Step      int64  `json:"step"`      // Training step number
	JobName   string `json:"jobName"`   // Parent training job
	Namespace string `json:"namespace"`
	PVC       string `json:"pvc"`       // PVC name containing the checkpoint
}

// --- Dataset types ---

type DatasetInfo struct {
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	Status       string   `json:"status"` // Bound, Pending, Lost
	StorageClass string   `json:"storageClass"`
	Capacity     string   `json:"capacity"`
	AccessModes  []string `json:"accessModes"`
	FluidCached  bool     `json:"fluidCached"`
	Age          string   `json:"age"`
}

type DatasetCreateSpec struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	StorageClass string `json:"storageClass"`
	Capacity     string `json:"capacity"`
	AccessMode   string `json:"accessMode"` // ReadWriteOnce, ReadWriteMany, ReadOnlyMany
}

// --- Experiment types ---

type ExperimentCreateSpec struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Description string `json:"description"`
}

type ExperimentInfo struct {
	Name        string          `json:"name"`
	Namespace   string          `json:"namespace"`
	Description string          `json:"description"`
	CreatedBy   string          `json:"createdBy"`
	CreateTime  string          `json:"createTime"`
	RunCount    int             `json:"runCount"`
	BestLoss    float64         `json:"bestLoss,omitempty"`
	Runs        []ExperimentRun `json:"runs,omitempty"`
}

type ExperimentData struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CreatedBy   string          `json:"createdBy"`
	Runs        []ExperimentRun `json:"runs"`
}

type ExperimentRun struct {
	JobName string             `json:"jobName"`
	JobKind string             `json:"jobKind"`
	Params  map[string]string  `json:"params,omitempty"`
	Metrics map[string]float64 `json:"metrics,omitempty"`
	Status  string             `json:"status"`
	AddedAt string             `json:"addedAt"`
}

type ExperimentRunSpec struct {
	JobName string             `json:"jobName"`
	JobKind string             `json:"jobKind"`
	Params  map[string]string  `json:"params,omitempty"`
	Metrics map[string]float64 `json:"metrics,omitempty"`
	Status  string             `json:"status"`
}

// --- Dashboard overview ---

type DashboardOverview struct {
	Notebooks    ResourceCount `json:"notebooks"`
	TrainingJobs ResourceCount `json:"trainingJobs"`
	ServingJobs  ResourceCount `json:"servingJobs"`
}

type ResourceCount struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
}
