# 在 ACK 上使用 kube-queue 为 RayJob 排队

[English](README.md) | 简体中文

多个团队同时提交 RayJob 时，可以使用 ACK 的 **ack-kube-queue**，先按团队配额排队，再由 KubeRay 创建任务所需的 RayCluster。本文提供一个只使用 CPU 的完整示例：主动限制配额使任务等待，随后提高配额，观察任务执行并验证计算结果。示例不需要 GPU、业务镜像、模型或存储卷。

## 工作原理

| 组件 | 职责 |
| --- | --- |
| ElasticQuotaTree | 为团队定义资源配额，通过叶子节点关联任务命名空间。`min` 表示最低保障，`max` 表示使用上限。 |
| ack-kube-queue | 根据配额树创建 Queue，为挂起的 RayJob 创建 QueueUnit，满足出队条件后将 `spec.suspend` 改为 `false`。 |
| KubeRay Operator | 观察 RayJob，创建 RayCluster 和提交器 Job，跟踪执行状态并清理集群。 |
| ACK 调度器 | 为已经创建的 Pod 选择实际节点。配额允许出队不代表节点一定能容纳 Pod。 |

本文使用 ACK 的 kube-queue，资源为 `Queue`、`QueueUnit` 和 `ElasticQuotaTree`，不使用 Kubernetes Kueue 的 `ClusterQueue`、`LocalQueue` 或 `kueue.x-k8s.io/queue-name` 标签。队列关联来自命名空间与配额叶子节点的绑定。

本例让每个 RayJob 通过 `rayClusterSpec` 创建独立集群；不使用 `clusterSelector` 向已有集群提交。后者的集群资源在任务提交前就已分配，不适用本文的集群创建前排队流程。

## 前提条件

- ACK Pro 或 ACK 灵骏集群，Kubernetes 1.24 或以上，已安装 KubeRay Operator。组件适用范围以 [ACK RayJob 调度文档](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/how-to-automate-ray-job-execution-in-ack-cluster)为准。
- ack-kube-queue v1.21.4 或以上，并启用 RayJob 扩展。配额能力按 [ElasticQuotaTree 与任务队列配置指南](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/improve-resource-utilization-with-elasticquotatree-and-task-queues)准备，包括 ack-koordinator。
- 已配置 `kubectl`，具有创建命名空间、配额树、ConfigMap 和 RayJob 的权限。
- 节点有足够资源运行 Head、Worker 和提交器，并为系统组件留有余量。Head 和 Worker 合计请求 **2 CPU、4 GiB 内存**；提交器额外请求 **100m CPU、512 MiB 内存**。
- 节点能拉取 `rayproject/ray:2.48.0`。若需镜像加速，将清单中的三个镜像地址统一替换为自己的 ACR 镜像；保持 `rayVersion` 与镜像中的 Ray 版本一致。私有镜像的拉取凭据需在任务命名空间中配置。

下载本目录的 [elastic-quota-tree.yaml](elastic-quota-tree.yaml) 和 [rayjob.yaml](rayjob.yaml)，在文件所在目录执行下面的命令。

## 1. 安装组件并启用 RayJob 扩展

在 [ACK 控制台](https://cs.console.aliyun.com/)进入目标集群，通过**应用 → 云原生 AI 套件**安装 ack-kube-queue。完整安装流程参见 [使用任务队列 ack-kube-queue 管理 AI/ML 工作负载](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues)。

默认只开启原生 Job 排队。进入**应用 → Helm**，更新 ack-kube-queue，保留原有参数并将以下配置设为 `true`：

```yaml
extension:
  ray:
    enable: true
```

确认组件已部署，且以下 CRD 均存在；若安装时自定义了组件命名空间，相应调整 `kube-queue`：

```bash
kubectl -n kube-queue get deploy
kubectl get crd rayjobs.ray.io \
  elasticquotatrees.scheduling.sigs.k8s.io \
  queues.scheduling.x-k8s.io queueunits.scheduling.x-k8s.io
```

## 2. 创建演示配额

先查看是否已有配额树：

```bash
kubectl get elasticquotatree -n kube-system
kubectl create namespace ray-queue-demo
```

**一个集群只支持 `kube-system` 中的一棵 ElasticQuotaTree。** 随附清单仅适用于没有配额树的演示集群。若已有配额树，应由管理员在原树中增加关联 `ray-queue-demo` 命名空间的叶子节点，并检查父节点容量；不要应用第二棵树或覆盖现有团队配额。

[elastic-quota-tree.yaml](elastic-quota-tree.yaml) 的根节点为 4 CPU、8 GiB，叶子节点初始 `min` 和 `max` 都为 1 CPU、2 GiB。叶子上限小于本例 RayCluster 的总请求，用于稳定复现等待配额的状态。实际环境应根据集群容量设置配额，并保持子节点保障量之和不超过父节点保障量。

```bash
# 仅在没有现有配额树的演示集群中执行。
kubectl apply -f elastic-quota-tree.yaml
kubectl get queue -n kube-queue
kubectl get queue -n kube-queue -o yaml
```

检查自动生成队列的 `kube-queue/quota-fullname` 注解，确认其关联本例叶子节点。队列名可能带随机后缀，后续排查使用实际名称。

## 3. 提交 RayJob 并观察排队

[rayjob.yaml](rayjob.yaml) 包含一个保存 Python 代码的 ConfigMap 和一个 RayJob。代码使用 Ray 远程任务计算 1 到 5 的平方并求和；Head 声明 `num-cpus: "0"`，计算任务在 Worker 上执行。

关键设置如下，完整可运行配置见清单：

| 设置 | 本例取值与作用 |
| --- | --- |
| `metadata.namespace` | `ray-queue-demo`，通过命名空间进入对应配额队列。 |
| `spec.suspend` | `true`，等待队列放行后再创建 RayCluster。 |
| `spec.submissionMode` | `K8sJobMode`，由 Kubernetes Job 向 Ray 提交任务。 |
| `spec.shutdownAfterJobFinishes` | `true`，任务结束后回收 RayCluster。 |
| `spec.ttlSecondsAfterFinished` | `300`，在回收前留出五分钟查看集群和日志。 |
| Worker 副本数 | 固定为 1，关闭自动扩缩容，使示例的资源请求保持确定。 |

```bash
kubectl apply -f rayjob.yaml
kubectl get rayjob ray-queue-demo -n ray-queue-demo \
  -o jsonpath='{.spec.suspend}{"\n"}'
kubectl get queueunit -n ray-queue-demo
kubectl describe queueunit -n ray-queue-demo
kubectl get raycluster,pods -n ray-queue-demo
```

预期 `spec.suspend` 保持 `true`，QueueUnit 的消息说明配额不足（例如 `Insufficient quota`），命名空间中尚无该任务的 RayCluster 和 Pod。QueueUnit 名称和状态信息随组件版本可能不同，应以实际输出为准。

RayJob 的排队资源量按 **Head 的 requests + 各 WorkerGroup 的副本数 × 单 Worker 的 requests** 计算，本例为 2 CPU、4 GiB。Head 的 Ray 逻辑 CPU 为 0 并不会消除其 Kubernetes CPU 请求；提交器还需要额外的节点资源。

## 4. 提高配额，让队列放行

编辑 `elastic-quota-tree.yaml`，仅将演示叶子节点的 `min` 和 `max` 改为下列值，保留根节点和命名空间绑定：

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

使用已有配额树时，在原树中调整演示叶子及必要的父节点配额，不要应用随附的独立配额树。等待 ack-kube-queue 自行将 `suspend` 改为 `false`，不要手动取消挂起来绕过排队。

出队后，KubeRay 创建 Head、Worker 和提交器，任务开始执行。若再次应用含 `suspend: true` 的 RayJob 清单，会重新请求挂起，可能导致已创建的集群被删除；因此本步骤只更新配额文件。

## 5. 检查执行结果与资源回收

```bash
kubectl wait rayjob/ray-queue-demo -n ray-queue-demo \
  --for=jsonpath='{.status.jobStatus}'=SUCCEEDED --timeout=600s
kubectl logs -n ray-queue-demo job/ray-queue-demo
kubectl get rayjob ray-queue-demo -n ray-queue-demo \
  -o jsonpath='{.status.jobDeploymentStatus}{" "}{.status.jobStatus}{"\n"}'
```

预期业务输出如下：

```text
squares: [1, 4, 9, 16, 25]
total: 55
```

`jobStatus: SUCCEEDED` 表示计算完成，`jobDeploymentStatus` 最终应为 `Complete`；任务完成与集群资源删除是不同的观察点。本例给 RayCluster 保留 300 秒再清理，期间仍会占用资源。请在清理前收集所需日志；生产任务应将日志、模型和数据输出持久化。`ttlSecondsAfterFinished` 在此控制 RayCluster 的清理等待时间，不应将它理解为 RayJob 对象必然在该时间后删除，详见 [RayJob 生命周期说明](https://docs.ray.io/en/latest/cluster/kubernetes/getting-started/rayjob-quick-start.html)。

## 常见问题

| 现象 | 检查项 |
| --- | --- |
| 没有 QueueUnit | 检查 `extension.ray.enable`、RayJob 的 `suspend: true`、命名空间与叶子配额绑定，以及队列组件事件。 |
| QueueUnit 一直等待 | 使用 `kubectl describe queueunit -n ray-queue-demo` 查看原因；核算 Head 和全部 Worker 的 requests，检查叶子及祖先配额与已占用资源。 |
| 已出队，Pod 仍 Pending | 查看 `kubectl describe pod -n ray-queue-demo <pod-name>`：检查可分配资源、污点、亲和性、存储和提交器的资源需求。 |
| Pod 为 `ImagePullBackOff` | 检查三个容器的镜像地址、节点网络和任务命名空间中的 `imagePullSecrets`。 |
| Pod Running，但 Ray 任务不执行 | 检查任务声明的 `num_cpus`/`num_gpus` 是否能在单个 Worker 上满足；配额放行不会增加 Ray 的逻辑资源。 |
| 等待命令超时 | 查看 RayJob 的状态和提交器日志，区分排队、集群启动和程序失败，不要仅通过取消挂起来重试。 |

## 清理

先收集日志，再删除本例的 RayJob 和 ConfigMap：

```bash
kubectl delete -f rayjob.yaml --ignore-not-found
```

如果创建的是本文独立演示配额树，且没有其他使用者，执行：

```bash
kubectl delete -f elastic-quota-tree.yaml --ignore-not-found
kubectl delete namespace ray-queue-demo
```

若使用的是共享配额树，只从原树中移除本次新增的叶子或命名空间绑定，确认命名空间无其他工作负载后再删除命名空间；不要删除整棵共享配额树。

## 扩展到 GPU 和业务任务

接入自己的任务时，替换 ConfigMap、入口命令和镜像，在 Worker 的 `requests` 与 `limits` 中加入相同数量的 `nvidia.com/gpu`，同时为叶子及父节点配置相应 GPU 配额。先按实际 Worker 副本数核算 CPU、内存和 GPU，再提交挂起的 RayJob。若启用自动扩缩容，还需重新评估扩容时的配额和节点容量。

需要模型、OSS PVC 和数据产物检查的完整业务示例，可继续阅读 [VLA 任务队列指南](../../3-e2e-examples/vla-pipeline-on-ack/README_KUBE_QUEUE_ZH.md)。
