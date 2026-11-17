# 使用 ACK Koord Queue 为 RayJob 排队

[English](README.md) | 简体中文

多个团队同时提交 RayJob 时，可以使用 **ACK Koord Queue（原名 ack-kube-queue）**，先按团队配额排队，再由 KubeRay 创建任务所需的 RayCluster。下文将依次说明和演示：

- **组件如何配合**：区分队列准入、scheduler 的 Pod 调度与 Ray 的任务执行。
- **基础排队流程**：用 CPU RayJob 演示配额不足时等待、提高配额后放行，以及结果检查和资源清理，无需 GPU、业务镜像或存储卷。
- **多团队 GPU 共享**：以两个团队共享 8 GPU 为例，说明最低保障、空闲资源借用与回收，以及队列优先级、Pod 优先级和 Head 保护的配置。

## 工作原理

| 组件 | 职责 |
| --- | --- |
| ElasticQuotaTree | 为团队定义资源配额，通过叶子节点关联任务命名空间。`min` 表示最低保障，`max` 表示使用上限。 |
| ACK Koord Queue | 根据配额树创建 Queue，为挂起的 RayJob 创建 QueueUnit，满足出队条件后将 `spec.suspend` 改为 `false`。 |
| KubeRay Operator | 观察 RayJob，创建 RayCluster 和提交器 Job，跟踪执行状态并清理集群。 |
| ACK 调度器 | 为已经创建的 Pod 选择实际节点。配额允许出队不代表节点一定能容纳 Pod。 |

本文使用 ACK Koord Queue，资源为 `Queue`、`QueueUnit` 和 `ElasticQuotaTree`，不使用 Kubernetes Kueue 的 `ClusterQueue`、`LocalQueue` 或 `kueue.x-k8s.io/queue-name` 标签。队列关联来自命名空间与配额叶子节点的绑定。

本例让每个 RayJob 通过 `rayClusterSpec` 创建独立集群；不使用 `clusterSelector` 向已有集群提交。后者的集群资源在任务提交前就已分配，不适用本文的集群创建前排队流程。

### 队列与 scheduler 如何配合

一次任务提交依次经过以下阶段，各组件通过 Kubernetes API 中的资源状态协作：

| 阶段 | 处理对象与决策 | 观察点 |
| --- | --- | --- |
| 1. 入队 | ACK Koord Queue 为 `suspend: true` 的 RayJob 创建 QueueUnit，按所属 Queue 的策略排序。 | QueueUnit、Queue 中的任务顺序。 |
| 2. 准入 | ACK Koord Queue 检查配额等出队条件，满足后更新 RayJob 为 `suspend: false`。默认配额检查关注 `max`，配置 AdmissionCheck 后还需通过额外检查。 | `spec.suspend`、QueueUnit 的消息和准入状态。 |
| 3. 创建资源 | KubeRay 观察到解除挂起，创建 RayCluster，并在集群就绪后提交任务。 | RayCluster、Head/Worker Pod、提交器 Job。 |
| 4. 调度 Pod | ACK scheduler 检查节点资源、亲和性、污点和配额约束；启用相应策略时，可回收其他配额组借用的资源。 | Pod 的 `PodScheduled` 条件和调度事件。 |
| 5. 执行计算 | Ray 在已启动的 Head/Worker 上分配 task、actor 和 placement group。 | Ray 任务状态、逻辑资源和业务日志。 |

**出队只意味着允许进入后续调度流程。** 队列不替代节点选择，也不因出队而预留某块物理 GPU；任务可能已经出队，但 Pod 仍因节点资源或拓扑不匹配而 Pending。反过来，若 RayJob 仍挂起且尚无 Pod，scheduler 无法为这个任务执行 Pod 抢占。参见 [队列准入检查](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/add-custom-admission-checks-to-ack-kube-queue)与 [Capacity Scheduling](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-capacity-scheduling/)。

队列整体准入也不等于 Gang Scheduling：若业务要求一组 Pod 满足最小成员数后才能启动，还需单独配置并验证 Gang 策略。下面的共享 GPU 案例使用可以独立推进、允许 Worker 重建的批处理任务。

## 前提条件

- ACK Pro 或 ACK 灵骏集群，Kubernetes 1.24 或以上，已安装 KubeRay Operator。组件适用范围以 [ACK RayJob 调度文档](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/how-to-automate-ray-job-execution-in-ack-cluster)为准。
- ACK Koord Queue v1.21.4 或以上，并启用 RayJob 扩展。配额能力按 [ElasticQuotaTree 与任务队列配置指南](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/improve-resource-utilization-with-elasticquotatree-and-task-queues)准备，包括 ack-koordinator。
- 已配置 `kubectl`，具有创建命名空间、配额树、ConfigMap 和 RayJob 的权限。
- 节点有足够资源运行 Head、Worker 和提交器，并为系统组件留有余量。Head 和 Worker 合计请求 **2 CPU、4 GiB 内存**；提交器额外请求 **100m CPU、512 MiB 内存**。
- 节点能拉取 `rayproject/ray:2.48.0`。若需镜像加速，将清单中的三个镜像地址统一替换为自己的 ACR 镜像；保持 `rayVersion` 与镜像中的 Ray 版本一致。私有镜像的拉取凭据需在任务命名空间中配置。

下载本目录的 [elastic-quota-tree.yaml](elastic-quota-tree.yaml) 和 [rayjob.yaml](rayjob.yaml)，在文件所在目录执行下面的命令。

## 1. 安装组件并启用 RayJob 扩展

在 [ACK 控制台](https://cs.console.aliyun.com/)进入目标集群，通过**应用 → 云原生 AI 套件**安装 ACK Koord Queue。完整安装流程参见 [使用任务队列 ACK Koord Queue 管理 AI/ML 工作负载](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues)。

默认只开启原生 Job 排队。进入**应用 → Helm**，更新 ACK Koord Queue 对应的 Helm 应用，保留原有参数并将以下配置设为 `true`：

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

使用已有配额树时，在原树中调整演示叶子及必要的父节点配额，不要应用随附的独立配额树。等待 ACK Koord Queue 自行将 `suspend` 改为 `false`，不要手动取消挂起来绕过排队。

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

## 进阶案例：保障任务与弹性批处理共享 8 GPU

### 目标与配额设计

假设一个资源池可供业务使用的容量为 **32 CPU、64 GiB、8 GPU**，且 CPU、内存和节点布局足以容纳以下任务。两个团队的诉求如下，名称和数值均为演示用途：

| 团队 | 命名空间 | GPU min / max | Pod 策略 |
| --- | --- | --- | --- |
| A：保障任务 | `ray-guaranteed` | 2 / 2 | Head 和 Worker 均受配额抢占保护，使用较高 Pod 优先级。 |
| B：弹性批处理 | `ray-batch` | 6 / 8 | Head 受保护；Worker 可被抢占，使用较低 Pod 优先级。 |

A 的保障资源闲置时，B 可以借用，最多运行 8 个单 GPU Worker。A 提交 2 GPU 任务后，希望通过调度器收回借出的 2 GPU，B 保留 6 GPU 的保障容量。`min` 是可回收的保障额度，并不表示对应 GPU 必须空置；`min=max` 也不会单独使 Pod 获得不可抢占属性。

[multi-team-quota.yaml](multi-team-quota.yaml) 给出了完整配额树。A 的 CPU、内存、GPU 都设置为 `min=max`；B 的 `min` 为 12 CPU、24 GiB、6 GPU，`max` 为 24 CPU、48 GiB、8 GPU。各维度子节点 `min` 之和均不超过根节点 `min`；子节点 `max` 可以重叠，不能因此认为 2 + 8 GPU 可以同时运行。

这是独立的进阶环境配置，不能与前面的演示树同时创建。已有树时，只把两个叶子合并到现有层级，并按实际容量调整祖先配额。无现有配额树的测试集群可执行：

```bash
kubectl create namespace ray-guaranteed
kubectl create namespace ray-batch
kubectl apply -f multi-team-quota.yaml
kubectl apply -f priority-classes.yaml
kubectl get queue -n kube-queue -o yaml
```

两个叶子均通过 `attributes.kube-queue/queue-policy: Block` 设置严格优先级：队头任务等待资源时，后面的低优先级任务不能继续借用该配额中它正在等待的资源。代价是可能产生队头阻塞，短小任务也需等待。该属性只控制目标队列，不是跨团队抢占开关，详见 [队列策略说明](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/configure-the-queue-policy-for-ack-kube-queue)。

### 分别设置队列优先级和 Pod 优先级

| 设置 | 位置 | 生效范围 |
| --- | --- | --- |
| `scheduling.x-k8s.io/priority: "9"` | RayJob 的 `metadata.annotations` | 同一个 Queue 内的任务排序；本例每个团队单独一个队列，不能用 A 的 9 与 B 的 3 推导跨队列抢占。 |
| `priorityClassName` | Head/Worker 的 Pod template `spec` | scheduler 使用的 Pod 优先级，参与调度与抢占判断。 |
| `quota.scheduling.alibabacloud.com/preemptible: "false"` | Head/Worker 的 Pod template `metadata.labels` | ACK 配额抢占保护；该配额组内所有受保护 Pod 的资源请求总量必须落在 `min` 内。 |

[priority-classes.yaml](priority-classes.yaml) 创建两个集群级 PriorityClass：`ray-protected=10000` 和 `ray-batch-worker=1000`，均不设为集群默认值。创建前确认名称未被其他业务使用。较高的 PriorityClass 本身不是不可抢占保证；`preemptionPolicy: Never` 表示该 Pod 不主动抢占别人，也不是保护该 Pod 不被抢占的开关。

下面是要合并进**完整业务 RayJob**的配置片段，不能单独 `kubectl apply`。保留原有镜像、入口、容器、资源请求和卷配置；标签必须落在 Pod template 上，仅贴到 RayJob 顶层不会自动保护 Pod。

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

为便于核算，两类任务的 Head 均请求 1 CPU、2 GiB、0 GPU，每个 Worker 请求 2 CPU、4 GiB、1 GPU；GPU 的 `requests` 与 `limits` 都设为 1。A 的 Head + 2 Worker 合计 **5 CPU、10 GiB、2 GPU**，全部在 A 的 `min` 内。B 的 Head + 8 Worker 合计 **17 CPU、34 GiB、8 GPU**，不超过 B 的 `max`；只保护 Head 时，受保护请求为 1 CPU、2 GiB，落在 B 的 `min` 内。提交器、Sidecar 和其他并发任务也需要计入相应的实际资源预算。

**不要把 B 的全部 Worker 都标为不可抢占。** 其 8 GPU 受保护请求会超过 B 的 6 GPU `min`，无法实现预期的借用与回收。这里的限制按配额组聚合，并且同时检查 CPU、内存和 GPU，不是“每个 Pod 小于 min 即可”。参见 [不可抢占 Pod 的配额限制](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/configure-elastic-quota-lendlimit)。

### 回收如何发生，如何验收

执行前，由管理员确认 ACK scheduler 已启用 Capacity Scheduling，并按 [抢占配置指南](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/enable-preemptive-scheduling/)将 `preemptionAlgorithm` 配置为 `ElasticQuota`。仅安装 ACK Koord Queue 或提高任务优先级不会开启这套回收规则。该设置影响集群调度，应在测试环境验证；本文不要求修改全局参数来运行前面的 CPU 示例。

在 **B 先运行、A 后提交**、没有其他负载、Worker 每个只申请 1 GPU 的条件下，目标状态如下：

| 阶段 | A 使用 GPU | B 使用 GPU | 解释 |
| --- | --- | --- | --- |
| A 空闲，B 已获准并运行 | 0 | 8 | B 使用自身 6 GPU 保障，并借用 A 闲置的 2 GPU。 |
| A 提交并通过队列准入 | 待调度 | 8 | KubeRay 创建 A 的 Pod；此时才有 scheduler 可以处理的待调度对象。 |
| 配额抢占与调度完成 | 2 | 6 | scheduler 回收 B 超过保障的 2 个单 GPU Worker；B Head 保留。 |
| A 结束且资源释放 | 0 | 最多 8 | B 的替补 Worker 在资源可用后重新调度，恢复弹性使用。 |

这是调度目标，不是无条件的即时结果。ElasticQuota 抢占要求 A 的待调度资源处于其保障范围内，B 有超过保障且可被回收的资源，并且驱逐后的节点能容纳 A；不应为满足 A 而把 B 压到保障以下。上述 B 回收后保留 Head + 6 Worker，即 13 CPU、26 GiB、6 GPU，仍满足 B 各维度的 `min`。Pod 资源粒度、亲和性和节点布局不合适时，回收可能无法完成。

观察命令如下，可分别在两个终端跟踪 Pod：

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

先确认 A 已出队，再检查 A 的调度事件、B Worker 的删除/重建事件，以及 B Head 的 UID 是否保持不变。若 A 仍为 `suspend: true`，先查 QueueUnit 中的配额、队头阻塞或 AdmissionCheck 原因；此时不能期待 scheduler 抢占来启动尚未创建的 Pod，也不要手动取消挂起掩盖准入问题。

B 的 `replicas: 8` 仍是期望副本数，抢占不会自动把它改成 6。KubeRay 可能补建被回收的 Worker，新增 Pod 在资源不足时保持 Pending。保护 Head 只降低控制面被配额抢占打断的风险，不保证 Worker 上的 actor、训练状态或整个 RayJob 不失败；批处理需支持重试、幂等输出或 checkpoint 恢复。固定 world size 的分布式训练不能直接套用该“8 降到 6 继续运行”的假设。若额外启用 Gang Scheduling，还需遵守同一 PodGroup 的优先级约束，不能直接照搬本例 Head/Worker 的不同优先级。

验证后先清理两个业务 RayJob 及其业务资源，再移除本例新增的配额叶子和命名空间。仅在配额树与 PriorityClass 均为本例独占、且没有其他使用者时，才删除相应整份资源清单。

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
