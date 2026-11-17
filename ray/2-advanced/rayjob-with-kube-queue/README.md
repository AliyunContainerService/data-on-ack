# Queue RayJobs with kube-queue on ACK

English | [简体中文](README_ZH.md)

Use **ack-kube-queue** to admit RayJobs against team quotas before KubeRay creates their RayClusters. This guide provides a CPU-only example: hold a job with a small quota, increase the quota, and verify the result after admission. No GPU, custom image, model, or persistent volume is required.

## How it works

| Component | Responsibility |
| --- | --- |
| ElasticQuotaTree | Defines team quotas and binds namespaces to leaf nodes. `min` is the guaranteed allocation; `max` is the usage ceiling. |
| ack-kube-queue | Creates Queues from the quota tree and a QueueUnit for each suspended RayJob; sets `spec.suspend` to `false` when admission conditions are met. |
| KubeRay Operator | Creates the RayCluster and submitter Job, tracks execution, and cleans up the cluster. |
| ACK scheduler | Assigns the resulting Pods to nodes. Quota admission does not guarantee that Pods can fit on available nodes. |

This guide uses ACK kube-queue resources: `Queue`, `QueueUnit`, and `ElasticQuotaTree`. It does not use Kubernetes Kueue's `ClusterQueue`, `LocalQueue`, or `kueue.x-k8s.io/queue-name` label. Namespace bindings select the quota queue.

Each RayJob creates its own cluster through `rayClusterSpec`. The example does not use `clusterSelector` to target an existing cluster, whose resources have already been allocated before job submission.

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
