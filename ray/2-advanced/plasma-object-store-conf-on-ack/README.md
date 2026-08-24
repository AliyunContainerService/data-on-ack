# Configure the Ray Object Store and Object Spilling on ACK

This guide explains how to configure Ray's object store on Alibaba Cloud ACK: how the object store works, how to set its size, and how to make Ray spill objects to a Kubernetes volume (ESSD cloud disk as an example) once the store is full. It is aimed at users who are new to Ray on Kubernetes.

Ray's official documentation on object spilling is a good companion to this guide: [Object Spilling — Ray Core](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html).

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) installed in the cluster
- The cloud disk CSI plugin (`diskplugin.csi.alibabacloud.com`), which is installed by default on ACK managed clusters

## Background: How Ray's Object Store and Spilling Work

- Every Ray node runs a shared-memory **object store** for objects passed between tasks and actors. By default the store is 30% of the container's memory (capped at 200 GiB). It can be overridden with `object-store-memory` (bytes).
- When the object store is full, Ray **spills** objects to a directory in the local filesystem, and transparently **restores** them (reads them back) when they are needed again. The default spill directory is a temporary directory under `/tmp/ray/session_...`.
- On Kubernetes, `/tmp` inside a container is small and ephemeral: it is lost when the pod is recreated and it shares the pod's writable layer with everything else. For a RayCluster on ACK you should therefore mount a dedicated Kubernetes volume — an ESSD cloud disk is a good choice — and point Ray's spilling at it.
- Spilling is a per-node behavior: each raylet spills to its own local directory. A spilled object is still lost if its node dies; spilling is for absorbing capacity bursts, not for durability.

> See also the official [Object Spilling](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html) documentation for the exact semantics.

## 1. Set Object Spilling via ray.init()

In a local or standalone Ray cluster you can pass the spill directory directly to `ray.init()`:

```python
import ray

ray.init(object_spilling_directory="/path/to/spill/dir")
```

The same setting exists as a command line option when starting Ray manually:

```bash
ray start --head --object-spilling-directory=/path/to/spill/dir
```

> `object_spilling_directory` (and `object-store-memory`) are **raylet** parameters: they take effect when the Ray node is started. When your driver connects to an existing cluster with `ray.init(address="auto")`, these parameters are ignored — the configuration of the running nodes already exists. On KubeRay, nodes are started by the operator from the `rayStartParams` of the `RayCluster` (next section), so that is where the configuration goes.

## 2. Spill to an ESSD Volume in a RayCluster

This section deploys a RayCluster whose head and worker each mount an ESSD cloud disk at `/spill`, with `object-spilling-directory` set to that path. The object store is capped at 1 GiB so the sample program can fill it quickly.

### 2.1 Deploy the RayCluster

`ray-cluster.yaml` uses a Kubernetes generic ephemeral volume (`volumeClaimTemplate`) for the spill directory. Each pod gets its own PVC and ESSD disk, created from the template and deleted automatically together with the pod.

Before applying, replace `<IMAGE_REGISTRY>` in `ray-cluster.yaml` with your image registry address (any Ray image works; this guide reuses the image built in [Ray Data with OSS](../../1-user-guide/3-ray-data-with-oss/README.md)).

```bash
kubectl apply -f ray-cluster.yaml
```

Expected output:

```
raycluster.ray.io/raycluster-object-spill created
```

Wait for the pods to be ready:

```bash
kubectl get pods
```

Expected output (1 head pod and 1 worker pod, both Running):

```
NAME                                                      READY   STATUS    RESTARTS   AGE
raycluster-object-spill-head-mskhh                        1/1     Running   0          71s
raycluster-object-spill-worker-group-worker-4n8r2         1/1     Running   0          42s
```

The key parts of `ray-cluster.yaml`:

- `rayStartParams` on both the head group and the worker group:
  - `object-store-memory: "1073741824"` — cap the object store at 1 GiB (the default is 30% of the container memory, which is large enough that the demo would need a lot of data to trigger spilling).
  - `object-spilling-directory: /spill` — spill to the mounted ESSD volume instead of the ephemeral `/tmp`.
- `volumes[].ephemeral.volumeClaimTemplate` — each pod gets its own ESSD disk at `/spill`. The PVC is named `<pod-name>-spill` and is deleted when the pod terminates.
- `securityContext.fsGroup: 1000` — the Ray container runs as UID 1000, while a fresh ESSD filesystem is owned by root. `fsGroup` makes the volume writable by the Ray user.

> The StorageClass `alicloud-disk-topology-alltype` uses `WaitForFirstConsumer` binding. It provisions an **ESSD** (`cloud_essd.PL1`) in the availability zone where the pod is scheduled, which avoids the multi-AZ issues of `Immediate`-binding classes (like `alicloud-disk-essd`). On a single-AZ cluster both work; the topology-aware one is safer for multi-AZ.

> ESSD disks are `ReadWriteOnce` (one node at a time). Since each pod gets its own disk through `volumeClaimTemplate`, this works naturally. For a worker group with multiple replicas, simply set a higher `replicas` count — each replica pod gets its own independent ESSD disk.

Verify the volume is mounted and writable:

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec $HEAD_POD -- df -h /spill
```

Expected output:

```
Filesystem      Size  Used Avail Use% Mounted on
/dev/vdb         20G   24K   20G   1% /spill
```

The auto-created PVCs appear as well:

```bash
kubectl get pvc
```

Expected output (names vary by pod):

```
NAME                                                         STATUS   VOLUME                   CAPACITY   ACCESS MODES   STORAGECLASS                  AGE
raycluster-object-spill-head-mskhh-spill                     Bound    d-2zecrrxs0sak0hv6y0yg   20Gi       RWO            alicloud-disk-topology-alltype  60s
raycluster-object-spill-worker-group-worker-4n8r2-spill      Bound    d-2ze70w3khfxqavlc3772   20Gi       RWO            alicloud-disk-topology-alltype  60s
```

### 2.2 Run a Sample That Spills

`object_spill_sample.py` puts 240 objects of 8 MiB each (~1.88 GiB in total) into the object store with `ray.put`. Since the store is capped at 1 GiB, Ray has to spill the overflow to the ESSD volume. Copy it into the head pod and run it:

```bash
kubectl cp object_spill_sample.py $HEAD_POD:/tmp/object_spill_sample.py
kubectl exec $HEAD_POD -- python /tmp/object_spill_sample.py
```

Expected output (the Ray INFO lines are abbreviated):

```
Put 240 objects, ~1920 MiB total
All objects are alive. Check spill stats with 'ray memory --stats-only'.
```

### 2.3 Verify That Spilling Happened

Check the cluster-wide spill statistics:

```bash
kubectl exec $HEAD_POD -- ray memory --stats-only
```

Expected output:

```
======== Object references status: 2026-08-17 20:49:20.210899 ========
--- Aggregate object store stats across all nodes ---
Plasma memory usage 720 MiB, 90 objects, 35.16% full, 0.0% needed
Spilled 1144 MiB, 143 objects, avg write throughput 223 MiB/s
```

The spilled files are on the ESSD volume:

```bash
kubectl exec $HEAD_POD -- ls /spill
```

Expected output:

```
lost+found
ray_spilled_objects_37d6d74ae053d78680b44210f0f1ed7dd77010c97bf6582dad9a8e67
```

The raylet also prints INFO-level messages about spilling, as described in the [official docs](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html). Find them in the raylet log:

```bash
kubectl exec $HEAD_POD -- grep "Spilled" /tmp/ray/session_latest/logs/raylet.out
```

Expected output:

```
[2026-08-17 20:49:11,455 I 660 660] (raylet) local_object_manager.cc:267: :info_message:Spilled 112 MiB, 14 objects, write throughput 62 MiB/s.
[2026-08-17 20:49:14,781 I 660 660] (raylet) local_object_manager.cc:267: :info_message:Spilled 1144 MiB, 143 objects, write throughput 223 MiB/s.
```

When the spilled objects are accessed again (for example with `ray.get`), Ray restores them from the volume. The restore side of the stats looks like:

```
Spilled 3416 MiB, 427 objects, avg write throughput 194 MiB/s
Restored 1360 MiB, 170 objects, avg read throughput 1842 MiB/s
```

## Tuning Notes

- **Object store size** — set `object-store-memory` (bytes) per group in `rayStartParams`. A store that is too small spills constantly; a store that is too large wastes memory that tasks and actors could use. The default is 30% of the container memory.
- **Spilling threshold** — Ray starts spilling when the store is 80% full (config `object_spilling_threshold`). You can tune it with the `RAY_object_spilling_threshold` environment variable on the pods (for example `0.9` spills less eagerly, at the cost of a higher OOM risk).
- **Spill I/O** — spilling is asynchronous; each raylet uses up to 4 I/O workers (`RAY_max_io_workers`). If spilling throughput is a bottleneck, a faster disk (ESSD PL1/PL2) helps more than tuning these numbers.
- **Durability** — spilled objects are node-local and are lost if the pod dies. Object spilling is for absorbing temporary capacity bursts, not for making objects durable; use Ray's external storage or a checkpointing mechanism for that.
- **Don't spill to a network filesystem** — a shared NAS is convenient but its latency makes spilling slow. Prefer node-local storage such as ESSD.

## Cleanup

```bash
kubectl delete -f ray-cluster.yaml
```

Deleting the RayCluster terminates the pods, which triggers the automatic removal of the ephemeral PVCs and their ESSD disks (the StorageClass reclaim policy is `Delete`).

## References

- [Object Spilling — Ray Core official documentation](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html)
- [RayCluster Configuration — KubeRay official documentation](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html)
- [Using cloud disks in ACK](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-cloud-disks)
