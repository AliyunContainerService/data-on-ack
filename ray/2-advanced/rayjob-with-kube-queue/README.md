# Queue RayJobs with kube-queue on ACK

English | [简体中文](README_ZH.md)

Use **ack-kube-queue** to admit RayJobs against team quotas before KubeRay creates their RayClusters. This guide covers the following topics in order:

- **Component responsibilities**: distinguish queue admission, Pod scheduling by the scheduler, and task execution by Ray.
- **Basic queuing workflow**: use a CPU RayJob to observe quota-based waiting, admission after a quota increase, result verification, and cleanup. No GPU, custom image, or persistent volume is required.
- **GPU sharing across teams**: use two teams sharing eight GPUs to explain guarantees, borrowing and reclamation of idle resources, and configuration of queue priority, Pod priority, and Head protection.

## How it works

| Component | Responsibility |
| --- | --- |
| ElasticQuotaTree | Defines team quotas and binds namespaces to leaf nodes. `min` is the guaranteed allocation; `max` is the usage ceiling. |
| ack-kube-queue | Creates Queues from the quota tree and a QueueUnit for each suspended RayJob; sets `spec.suspend` to `false` when admission conditions are met. |
| KubeRay Operator | Creates the RayCluster and submitter Job, tracks execution, and cleans up the cluster. |
| ACK scheduler | Assigns the resulting Pods to nodes. Quota admission does not guarantee that Pods can fit on available nodes. |

This guide uses ACK kube-queue resources: `Queue`, `QueueUnit`, and `ElasticQuotaTree`. It does not use Kubernetes Kueue's `ClusterQueue`, `LocalQueue`, or `kueue.x-k8s.io/queue-name` label. Namespace bindings select the quota queue.

Each RayJob creates its own cluster through `rayClusterSpec`. The example does not use `clusterSelector` to target an existing cluster, whose resources have already been allocated before job submission.

### How the queue and scheduler cooperate

A submitted job passes through these stages. Components cooperate through resource state in the Kubernetes API:

| Stage | Object and decision | What to observe |
| --- | --- | --- |
| 1. Enqueue | kube-queue creates a QueueUnit for a RayJob with `suspend: true` and orders it according to its Queue policy. | QueueUnit and job order in the Queue. |
| 2. Admit | kube-queue checks admission conditions, including quota, then sets `suspend: false`. The default quota check concerns `max`; configured AdmissionChecks add further conditions. | `spec.suspend`, QueueUnit messages, and admission status. |
| 3. Create resources | KubeRay observes the resumed RayJob, creates the RayCluster, and submits the job once the cluster is ready. | RayCluster, Head/Worker Pods, and submitter Job. |
| 4. Schedule Pods | ACK scheduler checks node capacity, affinity, taints, and quotas. With the appropriate policy enabled, it can reclaim resources borrowed by other quota groups. | Pod `PodScheduled` conditions and scheduling events. |
| 5. Execute | Ray places tasks, actors, and placement groups on the running Head/Workers. | Ray task state, logical resources, and application logs. |

**Admission permits the job to proceed to scheduling.** It does not select nodes or reserve a particular physical GPU. An admitted job can still have Pending Pods because node capacity or topology does not fit. Conversely, if a RayJob is suspended and has no Pods, the scheduler has no Pod on whose behalf to preempt. See [queue admission checks](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/add-custom-admission-checks-to-ack-kube-queue) and [Capacity Scheduling](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/use-capacity-scheduling/).

Whole-job admission also does not provide Gang Scheduling. If a workload needs a minimum group of Pods to start together, configure and validate gang behavior separately. The shared-GPU example below assumes batch work can make progress independently and recover from Worker replacement.

## Prerequisites

- An ACK Pro or ACK Lingjun cluster running Kubernetes 1.24 or later, with KubeRay Operator installed. See the [ACK RayJob scheduling guide](https://help.aliyun.com/en/ack/cloud-native-ai-suite/use-cases/how-to-automate-ray-job-execution-in-ack-cluster) for supported configurations.
- ack-kube-queue v1.21.4 or later with the RayJob extension enabled. Prepare quota support, including ack-koordinator, as described in the [ElasticQuotaTree and task queue guide](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/improve-resource-utilization-with-elasticquotatree-and-task-queues).
- Configured `kubectl` and permission to manage namespaces, the quota tree, ConfigMaps, and RayJobs.
- Available node capacity for the Head, Worker, submitter, and system components. The Head and Worker request **2 CPUs and 4 GiB memory** in total; the submitter additionally requests **100m CPU and 512 MiB memory**.
- Nodes can pull `rayproject/ray:2.48.0`. If necessary, replace all three image references with an ACR mirror, keeping `rayVersion` consistent with the image. Configure pull credentials in the job namespace for private images.

Download [elastic-quota-tree.yaml](elastic-quota-tree.yaml) and [rayjob.yaml](rayjob.yaml) from this directory and run the commands alongside those files.

## 1. Install the component and enable RayJob support

In the [ACK console](https://cs.console.aliyun.com/), open your cluster and install ack-kube-queue through **Applications → Cloud-native AI Suite**. Follow the complete [installation guide](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues).

Only native Job queuing is enabled by default. Under **Applications → Helm**, update ack-kube-queue, preserve its existing parameters, and enable:

```yaml
extension:
  ray:
    enable: true
```

Confirm that the component is deployed and all required CRDs exist. Replace `kube-queue` if you installed the component in another namespace:

```bash
kubectl -n kube-queue get deploy
kubectl get crd rayjobs.ray.io \
  elasticquotatrees.scheduling.sigs.k8s.io \
  queues.scheduling.x-k8s.io queueunits.scheduling.x-k8s.io
```

## 2. Create the demo quota

Check for an existing quota tree first:

```bash
kubectl get elasticquotatree -n kube-system
kubectl create namespace ray-queue-demo
```

**Only one ElasticQuotaTree, in `kube-system`, is supported per cluster.** The supplied manifest is for a demo cluster without an existing tree. Otherwise, ask the administrator to add a leaf bound to `ray-queue-demo` in the existing tree and check ancestor capacity. Do not apply a second tree or overwrite existing team quotas.

In [elastic-quota-tree.yaml](elastic-quota-tree.yaml), the root has 4 CPUs and 8 GiB memory. The leaf starts with both `min` and `max` set to 1 CPU and 2 GiB, below this example's RayCluster request, so the job must wait. Adapt quotas to actual cluster capacity; the sum of child guarantees must not exceed the parent's guarantee.

```bash
# Only for a demo cluster without an existing quota tree.
kubectl apply -f elastic-quota-tree.yaml
kubectl get queue -n kube-queue
kubectl get queue -n kube-queue -o yaml
```

Inspect the generated queue's `kube-queue/quota-fullname` annotation to identify the demo leaf. Queue names may contain generated suffixes; use the actual name when investigating a queue.

## 3. Submit the RayJob and observe queuing

[rayjob.yaml](rayjob.yaml) includes a ConfigMap containing Python code and a RayJob. Ray remote tasks calculate the squares of 1 through 5 and sum the results. The Head declares `num-cpus: "0"`, so the tasks execute on the Worker.

The full manifest contains these key settings:

| Setting | Value and purpose |
| --- | --- |
| `metadata.namespace` | `ray-queue-demo`; the namespace selects the quota queue. |
| `spec.suspend` | `true`; wait for admission before creating the RayCluster. |
| `spec.submissionMode` | `K8sJobMode`; submit through a Kubernetes Job. |
| `spec.shutdownAfterJobFinishes` | `true`; reclaim the RayCluster after execution. |
| `spec.ttlSecondsAfterFinished` | `300`; retain the cluster for five minutes before cleanup. |
| Worker replicas | Fixed at 1 with autoscaling disabled to keep resource requests deterministic. |

```bash
kubectl apply -f rayjob.yaml
kubectl get rayjob ray-queue-demo -n ray-queue-demo \
  -o jsonpath='{.spec.suspend}{"\n"}'
kubectl get queueunit -n ray-queue-demo
kubectl describe queueunit -n ray-queue-demo
kubectl get raycluster,pods -n ray-queue-demo
```

Expect `spec.suspend` to remain `true`, a QueueUnit message indicating insufficient quota (such as `Insufficient quota`), and no RayCluster or Pods for this job. Generated QueueUnit names and status messages can vary by component version.

The queued resource request is **Head requests + the sum of each WorkerGroup's replica count × per-Worker requests**: 2 CPUs and 4 GiB here. Setting the Head's logical Ray CPU count to zero does not remove its Kubernetes CPU request. Reserve additional node capacity for the submitter.

## 4. Increase the quota to admit the job

Edit `elastic-quota-tree.yaml`, changing only the demo leaf's `min` and `max` to the following values. Preserve the root and namespace binding:

```yaml
min:
  cpu: "4"
  memory: 8Gi
max:
  cpu: "4"
  memory: 8Gi
```

```bash
kubectl apply -f elastic-quota-tree.yaml
kubectl wait rayjob/ray-queue-demo -n ray-queue-demo \
  --for=jsonpath='{.spec.suspend}'=false --timeout=300s
kubectl get queueunit -n ray-queue-demo
kubectl get rayjob,raycluster,pods -n ray-queue-demo
```

For an existing quota tree, update the demo leaf and any necessary ancestors in that tree instead of applying the standalone manifest. Let ack-kube-queue change `suspend` to `false`; do not bypass admission by manually resuming the job.

KubeRay then creates the Head, Worker, and submitter to execute the job. Reapplying the RayJob manifest containing `suspend: true` requests suspension again and can delete an already-created cluster. Update only the quota file in this step.

## 5. Verify execution and resource cleanup

```bash
kubectl wait rayjob/ray-queue-demo -n ray-queue-demo \
  --for=jsonpath='{.status.jobStatus}'=SUCCEEDED --timeout=600s
kubectl logs -n ray-queue-demo job/ray-queue-demo
kubectl get rayjob ray-queue-demo -n ray-queue-demo \
  -o jsonpath='{.status.jobDeploymentStatus}{" "}{.status.jobStatus}{"\n"}'
```

Expected application output:

```text
squares: [1, 4, 9, 16, 25]
total: 55
```

`jobStatus: SUCCEEDED` indicates that computation finished, and `jobDeploymentStatus` should eventually become `Complete`. Job completion and cluster deletion are separate observations. The example retains the RayCluster for 300 seconds before cleanup; it continues to consume resources during that interval. Collect logs before cleanup, and persist logs, models, and data outputs for production workloads. Here, `ttlSecondsAfterFinished` controls the delay before RayCluster cleanup; it does not imply that the RayJob object will necessarily be deleted after that interval. See the [RayJob lifecycle documentation](https://docs.ray.io/en/latest/cluster/kubernetes/getting-started/rayjob-quick-start.html).

## Advanced case: guaranteed jobs and elastic batch processing share eight GPUs

### Requirements and quotas

Assume a pool provides **32 CPUs, 64 GiB memory, and 8 GPUs** for application workloads, with sufficient CPU, memory, and suitable node placement for the jobs below. All names and quantities are illustrative:

| Team | Namespace | GPU min / max | Pod policy |
| --- | --- | --- | --- |
| A: guaranteed jobs | `ray-guaranteed` | 2 / 2 | Protect both Head and Workers from quota preemption and assign higher Pod priority. |
| B: elastic batch | `ray-batch` | 6 / 8 | Protect the Head; allow lower-priority Workers to be preempted. |

When A is idle, B can borrow its unused guarantee and run up to eight single-GPU Workers. When A submits a two-GPU job, the intended behavior is to reclaim the two borrowed GPUs while preserving B's six-GPU guarantee. `min` is a reclaimable guarantee, not a requirement to leave GPUs idle. Setting `min=max` alone does not make Pods non-preemptible.

[multi-team-quota.yaml](multi-team-quota.yaml) supplies the full quota tree. A has equal `min` and `max` for CPU, memory, and GPUs. B has a `min` of 12 CPUs, 24 GiB, and 6 GPUs, and a `max` of 24 CPUs, 48 GiB, and 8 GPUs. Child guarantees sum to no more than the root guarantee for every resource. Child maxima can overlap; this does not allow 2 + 8 GPUs to run simultaneously.

This advanced configuration is a separate environment and must not be installed alongside the earlier demo tree. If a tree already exists, merge the two leaves into its hierarchy and size ancestor quotas for actual capacity. In a test cluster without an existing tree:

```bash
kubectl create namespace ray-guaranteed
kubectl create namespace ray-batch
kubectl apply -f multi-team-quota.yaml
kubectl apply -f priority-classes.yaml
kubectl get queue -n kube-queue -o yaml
```

Both leaves set `attributes.kube-queue/queue-policy: Block` for strict priority. While the leading job waits for resources, later lower-priority jobs cannot borrow the resources it is waiting for in that quota. This can cause head-of-line blocking, including for small jobs. The attribute controls its queue; it does not enable cross-team preemption. See [queue policies](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/configure-the-queue-policy-for-ack-kube-queue).

### Configure queue priority and Pod priority separately

| Setting | Location | Scope |
| --- | --- | --- |
| `scheduling.x-k8s.io/priority: "9"` | RayJob `metadata.annotations` | Job ordering within one Queue. Each team has its own queue here; A's 9 versus B's 3 does not define cross-queue preemption. |
| `priorityClassName` | Head/Worker Pod template `spec` | Pod priority used by the scheduler for scheduling and preemption decisions. |
| `quota.scheduling.alibabacloud.com/preemptible: "false"` | Head/Worker Pod template `metadata.labels` | Protection from ACK quota preemption. Aggregate requests of all protected Pods in a quota group must fit within its `min`. |

[priority-classes.yaml](priority-classes.yaml) creates two cluster-scoped PriorityClasses: `ray-protected=10000` and `ray-batch-worker=1000`. Neither becomes the cluster default. Check that these names are unused before creating them. Higher Pod priority alone does not guarantee immunity from preemption. `preemptionPolicy: Never` prevents a Pod from preempting others; it does not protect that Pod from being preempted.

Merge the following fragments into **complete application RayJob manifests**; do not apply these fragments directly. Preserve images, entrypoints, containers, resource requests, and volumes. Labels must be on Pod templates: placing them only on the RayJob's top-level metadata does not automatically protect Pods.

```yaml
# Team A: merge into the complete RayJob manifest.
metadata:
  name: guaranteed-job
  namespace: ray-guaranteed
  annotations:
    scheduling.x-k8s.io/priority: "9"
spec:
  suspend: true
  shutdownAfterJobFinishes: true
  rayClusterSpec:
    enableInTreeAutoscaling: false
    headGroupSpec:
      template:
        metadata:
          labels:
            quota.scheduling.alibabacloud.com/preemptible: "false"
        spec:
          priorityClassName: ray-protected
    workerGroupSpecs:
      - groupName: gpu-worker
        replicas: 2
        minReplicas: 2
        maxReplicas: 2
        template:
          metadata:
            labels:
              quota.scheduling.alibabacloud.com/preemptible: "false"
          spec:
            priorityClassName: ray-protected
```

```yaml
# Team B: merge into the complete RayJob manifest.
metadata:
  name: batch-job
  namespace: ray-batch
  annotations:
    scheduling.x-k8s.io/priority: "3"
spec:
  suspend: true
  shutdownAfterJobFinishes: true
  rayClusterSpec:
    enableInTreeAutoscaling: false
    headGroupSpec:
      template:
        metadata:
          labels:
            quota.scheduling.alibabacloud.com/preemptible: "false"
        spec:
          priorityClassName: ray-protected
    workerGroupSpecs:
      - groupName: gpu-worker
        replicas: 8
        minReplicas: 8
        maxReplicas: 8
        template:
          metadata:
            labels:
              quota.scheduling.alibabacloud.com/preemptible: "true"
          spec:
            priorityClassName: ray-batch-worker
```

For resource accounting, each Head requests 1 CPU, 2 GiB, and no GPU; each Worker requests 2 CPUs, 4 GiB, and one GPU, with both GPU `requests` and `limits` set to 1. A's Head and two Workers total **5 CPUs, 10 GiB, and 2 GPUs**, all within A's `min`. B's Head and eight Workers total **17 CPUs, 34 GiB, and 8 GPUs**, within B's `max`. Protecting only B's Head consumes 1 CPU and 2 GiB of its guarantee. Include submitters, sidecars, and other concurrent workloads in the relevant actual resource budgets.

**Do not mark all of B's Workers non-preemptible.** Their eight-GPU protected request would exceed B's six-GPU `min`, defeating the intended borrowing and reclamation behavior. The restriction applies to aggregate usage in the quota group across CPU, memory, and GPUs, not to each Pod individually. See [non-preemptible Pod quota limits](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/configure-elastic-quota-lendlimit).

### Reclamation and acceptance criteria

Before testing, have the administrator verify that ACK scheduler has Capacity Scheduling enabled and configure `preemptionAlgorithm: ElasticQuota` following the [preemption guide](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/enable-preemptive-scheduling). Installing kube-queue or increasing job priority alone does not enable these reclamation rules. This is a cluster-wide scheduler setting to validate in a test environment; the earlier CPU example does not require changing it.

With **B running before A submits**, no other workloads, and one GPU requested per Worker, the target progression is:

| Stage | A's GPU usage | B's GPU usage | Explanation |
| --- | --- | --- | --- |
| A idle; B admitted and running | 0 | 8 | B uses its six-GPU guarantee and borrows A's two idle GPUs. |
| A submitted and admitted | Pending scheduling | 8 | KubeRay creates A's Pods, giving the scheduler pending objects to act on. |
| Quota preemption and scheduling complete | 2 | 6 | The scheduler reclaims two single-GPU Workers above B's guarantee; B's Head remains. |
| A finishes and releases resources | 0 | Up to 8 | B's replacement Workers can schedule when capacity becomes available again. |

These are scheduling targets, not unconditional immediate outcomes. ElasticQuota preemption requires A's pending demand to be within its guarantee, B to have reclaimable usage above its guarantee, and the resulting node capacity to fit A's Pods. Reclamation should not push B below its guarantee. B's remaining Head and six Workers request 13 CPUs, 26 GiB, and 6 GPUs, satisfying its `min` in every dimension. Pod resource granularity, affinity, and node layout can prevent reclamation from succeeding.

Use these commands to inspect the transition; follow each team's Pods in separate terminals if needed:

```bash
kubectl get queueunit -n ray-guaranteed -o yaml
kubectl get queueunit -n ray-batch -o yaml
kubectl get rayjob guaranteed-job -n ray-guaranteed \
  -o jsonpath='{.spec.suspend}{"\n"}'
kubectl get pods -n ray-guaranteed -o wide
kubectl get pods -n ray-batch -o wide
kubectl get events -n ray-guaranteed --sort-by=.metadata.creationTimestamp
kubectl get events -n ray-batch --sort-by=.metadata.creationTimestamp
```

First verify that A has been admitted. Then inspect A's scheduling events, B's Worker deletion/recreation events, and whether B's Head UID stays unchanged. If A still has `suspend: true`, inspect QueueUnit quota, head-of-line blocking, or AdmissionCheck messages first. The scheduler cannot preempt for Pods that have not been created; do not manually resume the job to hide an admission problem.

B's `replicas: 8` remains its desired count; preemption does not automatically change it to six. KubeRay may replace evicted Workers, leaving replacement Pods Pending while resources are unavailable. Head protection only reduces disruption from quota preemption: actors, training state, or the whole RayJob can still fail when Workers disappear. Batch work needs retries, idempotent output, or checkpoint recovery. Fixed-world-size distributed training cannot assume it will continue when eight Workers drop to six. If adding Gang Scheduling, obey the priority constraints within a PodGroup rather than copying this example's differing Head/Worker priorities unchanged.

After validation, remove the two application RayJobs and their application resources, then remove the example quota leaves and namespaces. Delete the entire quota tree or PriorityClasses only if they are dedicated to this example and have no other users.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| No QueueUnit | Check `extension.ray.enable`, `suspend: true`, the namespace's leaf quota binding, and queue component events. |
| QueueUnit keeps waiting | Inspect `kubectl describe queueunit -n ray-queue-demo`; total the Head and all Workers' requests and check leaf/ancestor quotas and current usage. |
| Admitted, but Pods remain Pending | Run `kubectl describe pod -n ray-queue-demo <pod-name>`; check allocatable resources, taints, affinity, storage, and submitter requirements. |
| `ImagePullBackOff` | Check all three image references, node connectivity, and `imagePullSecrets` in the job namespace. |
| Pods run, but Ray tasks do not | Check whether a single Worker can satisfy each task's `num_cpus`/`num_gpus`; quota admission does not add logical Ray resources. |
| A wait command times out | Inspect RayJob status and submitter logs to distinguish queuing, cluster startup, and application failure. Do not simply clear suspension to retry. |

## Cleanup

Collect logs, then delete the example RayJob and ConfigMap:

```bash
kubectl delete -f rayjob.yaml --ignore-not-found
```

If you created the standalone demo quota tree and no other workloads use it:

```bash
kubectl delete -f elastic-quota-tree.yaml --ignore-not-found
kubectl delete namespace ray-queue-demo
```

For a shared tree, remove only the leaf or namespace binding added for this example. Delete the namespace only after confirming it contains no other workloads; do not delete the shared quota tree.

## Adapt to GPUs and application workloads

Replace the ConfigMap, entrypoint, and images for your application. Add equal `nvidia.com/gpu` quantities to Worker `requests` and `limits`, and provide matching GPU quota at the leaf and ancestor nodes. Calculate CPU, memory, and GPU requests from the actual Worker replica counts before submitting a suspended RayJob. If enabling autoscaling, reassess quota and node capacity for scale-out.

For a business workload with models, OSS PVCs, and output validation, continue with the [VLA queue guide](../../3-e2e-examples/vla-pipeline-on-ack/README_KUBE_QUEUE.md).
