# 在 ACK 上配置 Ray 对象存储和对象溢出（Object Spilling）

本文介绍如何在阿里云 ACK 集群上配置 Ray 的对象存储（object store）及其溢出（spilling）行为：对象存储的工作原理、如何设置其大小、以及如何让 Ray 在对象存储满时将数据溢出到 Kubernetes 卷（以 ESSD 云盘为例）。本文面向 Ray on Kubernetes 的新手用户。

Ray 官方对象溢出文档是本文的重要参考：[Object Spilling — Ray Core](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html)。

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)
- 云盘 CSI 插件（`diskplugin.csi.alibabacloud.com`），ACK 托管版集群默认已安装

## 背景：Ray 对象存储与溢出机制

- 每个 Ray 节点运行一个基于共享内存的 **对象存储（object store）**，用于在任务和 Actor 之间传递对象。默认大小为容器内存的 30%（上限 200 GiB），可通过 `object-store-memory`（字节数）覆盖。
- 当对象存储满时，Ray 会将对象 **溢出（spill）** 到本地文件系统的目录中，并在需要时透明地 **恢复（restore）**（从磁盘读回）。默认溢出目录是 `/tmp/ray/session_...` 下的临时目录。
- 在 Kubernetes 上，容器内的 `/tmp` 空间小且是临时的：Pod 重建后数据丢失，且与其他数据共享容器的可写层。因此，在 ACK 上的 RayCluster 中，建议挂载一个专用的 Kubernetes 卷（例如：ESSD 云盘），并将溢出目录指向该卷。
- 溢出是**每个节点独立**的行为：每个 raylet 往自己的本地目录溢出。溢出对象在节点宕机时仍然会丢失——溢出是为了吸收容量突发，而不是为了持久性。

> 更多详细语义请参考官方 [Object Spilling](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html) 文档。

## 1. 通过 ray.init() 设置对象溢出目录

在本地或单机 Ray 集群中，可以直接在 `ray.init()` 中传入溢出目录路径：

```python
import ray

ray.init(object_spilling_directory="/path/to/spill/dir")
```

手动启动 Ray 时也可以通过命令行选项指定：

```bash
ray start --head --object-spilling-directory=/path/to/spill/dir
```

> `object_spilling_directory`（以及 `object-store-memory`）是 **raylet** 参数：它们在 Ray 节点启动时生效。当你的 Driver 通过 `ray.init(address="auto")` 连接已有集群时，这些参数会被忽略——运行中节点的配置已经固定。在 KubeRay 上，节点由 Operator 根据 RayCluster 的 `rayStartParams` 启动，因此配置应放在那里（见下一节）。

## 2. 在 RayCluster 中溢出到 ESSD 卷

本节部署一个 RayCluster，其 head 和 worker 节点都通过 `volumeClaimTemplate`（临时卷模板）挂载独立的 ESSD 云盘到 `/spill`，并将 `object-spilling-directory` 设置为该路径。在本示例中，对象存储大小限制为 1 GiB，以便示例程序能快速填满它并触发溢出。

### 2.1 部署 RayCluster

`ray-cluster.yaml` 使用 Kubernetes 的通用临时卷（`volumeClaimTemplate`）来声明溢出目录。每个 Pod 自动获得独立的 PVC 和 ESSD 云盘，PVC 和磁盘在 Pod 终止时自动删除。

应用前，将 `ray-cluster.yaml` 中的 `<IMAGE_REGISTRY>` 替换为你实际的镜像仓库地址（任何 Ray 镜像均可，本文复用 [基于 OSS 的 Ray Data 指南](../../1-user-guide/3-ray-data-with-oss/README_ZH.md) 中构建的镜像）。

```bash
kubectl apply -f ray-cluster.yaml
```

预期输出：

```
raycluster.ray.io/raycluster-object-spill created
```

等待 Pod 就绪：

```bash
kubectl get pods
```

预期输出（1 个 head Pod 和 1 个 worker Pod 均为 Running）：

```
NAME                                                      READY   STATUS    RESTARTS   AGE
raycluster-object-spill-head-mskhh                        1/1     Running   0          71s
raycluster-object-spill-worker-group-worker-4n8r2         1/1     Running   0          42s
```

`ray-cluster.yaml` 的关键配置：

- `rayStartParams`（head 组和 worker 组均有）：
  - `object-store-memory: "1073741824"` — 将对象存储上限设为 1 GiB（默认是容器内存的 30%，在演示环境下需要大量数据才能触发溢出）。
  - `object-spilling-directory: /spill` — 溢出到挂载的 ESSD 卷，而不是临时的 `/tmp`。
- `volumes[].ephemeral.volumeClaimTemplate` — 每个 Pod 获得独立的 ESSD 云盘挂载到 `/spill`。PVC 命名为 `<pod名>-spill`，Pod 删除时自动清理。
- `securityContext.fsGroup: 1000` — Ray 容器以 UID 1000 运行，而新创建的 ESSD 文件系统属主为 root。`fsGroup` 使卷对 Ray 用户可写。

> `alicloud-disk-topology-alltype` 存储类使用 `WaitForFirstConsumer` 绑定模式：云盘在 Pod 调度到的可用区创建，类型为 **ESSD**（`cloud_essd.PL1`）。这避免了 `Immediate` 绑定类（如 `alicloud-disk-essd`）在多可用区集群中可能出现的"盘在无节点可用区"问题。单可用区集群两种都可以用；拓扑感知类更适合多可用区。

> ESSD 云盘是 `ReadWriteOnce`（一次仅一个节点）。由于每个 Pod 通过 `volumeClaimTemplate` 获得自己的盘，这本来就是一对一的关系。如果 worker 组有多个副本，只需调高 `replicas` 数量——每个副本 Pod 都会获得独立的 ESSD 云盘。

验证卷已挂载并可写：

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec $HEAD_POD -- df -h /spill
```

预期输出：

```
Filesystem      Size  Used Avail Use% Mounted on
/dev/vdb         20G   24K   20G   1% /spill
```

自动创建的 PVC 也可以通过以下命令查看：

```bash
kubectl get pvc
```

预期输出（名称随 Pod 名变化）：

```
NAME                                                         STATUS   VOLUME                   CAPACITY   ACCESS MODES   STORAGECLASS                  AGE
raycluster-object-spill-head-mskhh-spill                     Bound    d-2zecrrxs0sak0hv6y0yg   20Gi       RWO            alicloud-disk-topology-alltype  60s
raycluster-object-spill-worker-group-worker-4n8r2-spill      Bound    d-2ze70w3khfxqavlc3772   20Gi       RWO            alicloud-disk-topology-alltype  60s
```

### 2.2 运行触发溢出的示例程序

`object_spill_sample.py` 通过 `ray.put` 向对象存储中放入 240 个大小为 8 MiB 的对象（总计约 1.88 GiB）。由于对象存储上限为 1 GiB，Ray 必须将超出部分溢出到 ESSD 卷。将该文件复制到 head Pod 并运行：

```bash
kubectl cp object_spill_sample.py $HEAD_POD:/tmp/object_spill_sample.py
kubectl exec $HEAD_POD -- python /tmp/object_spill_sample.py
```

预期输出（Ray INFO 日志已省略）：

```
Put 240 objects, ~1920 MiB total
All objects are alive. Check spill stats with 'ray memory --stats-only'.
```

### 2.3 验证溢出是否发生

查看集群维度的溢出统计：

```bash
kubectl exec $HEAD_POD -- ray memory --stats-only
```

预期输出：

```
======== Object references status: 2026-08-17 20:49:20.210899 ========
--- Aggregate object store stats across all nodes ---
Plasma memory usage 720 MiB, 90 objects, 35.16% full, 0.0% needed
Spilled 1144 MiB, 143 objects, avg write throughput 223 MiB/s
```

溢出的文件位于 ESSD 卷上：

```bash
kubectl exec $HEAD_POD -- ls /spill
```

预期输出：

```
lost+found
ray_spilled_objects_37d6d74ae053d78680b44210f0f1ed7dd77010c97bf6582dad9a8e67
```

Raylet 也会在日志中打印 INFO 级别的溢出消息（如[官方文档](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html)所述）。查看 raylet 日志：

```bash
kubectl exec $HEAD_POD -- grep "Spilled" /tmp/ray/session_latest/logs/raylet.out
```

预期输出：

```
[2026-08-17 20:49:11,455 I 660 660] (raylet) local_object_manager.cc:267: :info_message:Spilled 112 MiB, 14 objects, write throughput 62 MiB/s.
[2026-08-17 20:49:14,781 I 660 660] (raylet) local_object_manager.cc:267: :info_message:Spilled 1144 MiB, 143 objects, write throughput 223 MiB/s.
```

当溢出的对象再次被访问时（例如通过 `ray.get`），Ray 从卷中恢复它们。恢复部分的统计如下：

```
Spilled 3416 MiB, 427 objects, avg write throughput 194 MiB/s
Restored 1360 MiB, 170 objects, avg read throughput 1842 MiB/s
```

## 调优建议

- **对象存储大小** — 在 `rayStartParams` 中通过 `object-store-memory`（字节数）按组设置。过小会导致频繁溢出；过大会浪费任务和 Actor 可用的内存。默认值为容器内存的 30%。
- **溢出阈值** — Ray 在对象存储使用率达到 80% 时开始溢出（配置项 `object_spilling_threshold`）。可以通过 Pod 的环境变量 `RAY_object_spilling_threshold` 调整（例如设为 `0.9` 减少溢出频率，但 OOM 风险升高）。
- **溢出 I/O** — 溢出是异步的；每个 raylet 最多使用 4 个 I/O 工作线程（`RAY_max_io_workers`）。如果溢出吞吐成为瓶颈，更快的磁盘（ESSD PL1/PL2）比调整这些数值更有效。
- **持久性** — 溢出的对象是节点本地的，Pod 宕机后丢失。对象溢出用于吸收临时的容量突发，不应用于持久化——需要持久化时请使用 Ray 的外部存储或检查点机制。

## 清理

```bash
kubectl delete -f ray-cluster.yaml
```

删除 RayCluster 会终止所有 Pod，进而触发临时 PVC 及其 ESSD 云盘的自动删除（存储类的回收策略为 `Delete`）。

## 参考

- [Object Spilling — Ray Core 官方文档](https://docs.ray.io/en/latest/ray-core/objects/object-spilling.html)
- [RayCluster 配置 — KubeRay 官方文档](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/config.html)
- [在 ACK 中使用云盘](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-cloud-disks)