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
- On Kubernetes, `/tmp` inside a container is small and ephemeral: it is lost when the pod is recreated and it shares the pod's writable layer with everything else. For a RayCluster on ACK you should therefore mount a dedicated Kubernetes volume (for example, an ESSD cloud disk) and point Ray's spilling at it.
- Spilling is a per-node behavior: each raylet spills to its own local directory. A spilled object is still lost if its node dies; spilling is for absorbing capacity bursts, not for durability.

> See also the official [Object Spilling](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html) documentation for the exact semantics.

## 1. Spill to an ESSD Volume in a RayCluster

This section deploys a RayCluster whose head and worker each mount an ESSD cloud disk at `/spill`, with `object-spilling-directory` set to that path. In this example, the object store is capped at 1 GiB so the sample program can fill it quickly.

### 1.1 Deploy the RayCluster

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

### 1.2 Run a Sample That Spills

`object_spill_sample.py` puts 240 objects of 8 MiB each (~1.88 GiB in total) into the object store with `ray.put`. Since the store is capped at 1 GiB, Ray has to spill the overflow to the ESSD volume. Copy it into the head pod and run it (`-u` keeps the output in order):

```bash
kubectl cp object_spill_sample.py $HEAD_POD:/tmp/object_spill_sample.py
kubectl exec $HEAD_POD -- python -u /tmp/object_spill_sample.py
```

The script puts the objects, waits a few seconds for the asynchronous spilling to settle, measures how much of the spill volume is occupied, and then holds the references for 30 seconds so you can inspect the cluster from another terminal. It takes about a minute to finish.

Expected output (Ray INFO lines are abbreviated; the `/spill` usage varies):

```
Put 240 objects, ~1920 MiB total
Spill directory usage while the objects are alive:
1.7G	/spill
Holding the references for 30 seconds; inspect the cluster from another terminal...
Driver exiting; references released.
The spilled objects under /spill are deleted once the script exits.
```

While the references are alive, the objects that do not fit into the in-memory object store (the store keeps roughly the most recent 1 GiB in RAM) sit on the ESSD volume as spill files.

### 1.3 Observe the Spilled Objects' Lifecycle

During the 30-second hold window, run the checks below from another terminal: the objects are split between the in-memory object store and the spill volume.

```bash
kubectl exec $HEAD_POD -- du -sh /spill
kubectl exec $HEAD_POD -- ray memory --stats-only
```

Expected output (numbers vary with the cluster):

```
1.7G	/spill
======== Object references status: 2026-08-24 00:52:28.365640 ========
--- Aggregate object store stats across all nodes ---
Plasma memory usage 984 MiB, 123 objects, 32.03% full, 8.59% needed
Spilled 6728 MiB, 841 objects, avg write throughput 214 MiB/s
```

- `Plasma memory usage` is the current in-memory usage of the object store.
- `Spilled ...` is a **cumulative counter** since the raylet started (the test cluster had run the sample several times), not the current disk usage. The current usage is what `du -sh /spill` shows.

Once the script exits, the driver releases the references and Ray deletes the spill files within a few seconds. Verify it:

```bash
kubectl exec $HEAD_POD -- du -sh /spill
kubectl exec $HEAD_POD -- ray memory --stats-only
```

Expected output:

```
24K	/spill
======== Object references status: 2026-08-24 00:52:54.909951 ========
--- Aggregate object store stats across all nodes ---
Plasma memory usage 0 MiB, 0 objects, 0.0% full, 0.0% needed
Spilled 1856 MiB, 232 objects, avg write throughput 184 MiB/s
```

The spill directory only keeps the empty `ray_spilled_objects_...` subdirectory. If the objects are accessed again (for example with `ray.get`) while they are alive, Ray transparently restores them from the spill volume first; see the [official Object Spilling](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html) documentation for the exact semantics.

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
- [Using cloud disks in ACK](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/disk-volume-overview-3)
