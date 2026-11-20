# 在 ACS 上运行 Rollout Server 与沙箱

在容器计算服务 ACS 上运行 Rollout Server 和 trial 沙箱，分三种场景：Rollout Server 部署在
ACS 集群中；ACK 集群借用 ACS 算力运行沙箱；使用 ACS Agent Sandbox 的预热池运行沙箱。
三种场景都只通过 Helm values 配置，不修改 chart。

**为什么。** trial 沙箱数量随 rollout 并发起伏，且每个沙箱只存活一次 trial。按 Pod 计费、
秒级弹性的 ACS 算力比常驻节点池更贴合这种负载；Agent Sandbox 的预热池进一步省去 Pod 创建和
镜像拉取的等待。但 ACS Pod 有自己的约束（规格取整、无特权容器、存储与网络限制），默认 values
不能直接照搬。

Rollout Server 的基础部署与轨迹持久化参见[用 RolloutServer 管理轨迹](../3-rolloutserver-trajectory-management/README_ZH.md)。

## 工作原理

Rollout Server 由两类 Pod 组成，调度配置互相独立：

```text
Helm values
  ├─ 顶层 nodeSelector / tolerations / resources ──→ 服务端 Pod（ingress + worker + Redis sidecar）
  └─ config.*                                    ──→ 服务端创建的 trial 沙箱
        ├─ pod_overrides            → 深度合并进沙箱 Pod（或 SandboxSet 的 Pod 模板）
        ├─ use_sandbox_claim: true  → SandboxSet 预热池 + SandboxClaim 领用
        └─ backends（E2BEnvironment）→ 经 ack-sandbox-manager 的 E2B API 领用
```

| 场景 | 服务端 Pod | 沙箱 | 环境后端 | 关键 values |
| --- | --- | --- | --- | --- |
| 一：部署在 ACS 集群 | ACS Pod | ACS Pod | `ACKEnvironment`，Pod 模式 | `persistence.nas`、`resources`、`global.imagePrefix` |
| 二：ACK 集群借用 ACS 算力 | ECS 节点 | ACS Pod，经虚拟节点 | `ACKEnvironment`，Pod 模式 | `config.pod_overrides.metadata.labels` |
| 三：ACS Agent Sandbox | ACK 或 ACS 集群 | Agent Sandbox | `ACKEnvironment` SandboxClaim 模式，或 `E2BEnvironment` | `config.use_sandbox_claim` 或 `config.backends` |

## 前置条件

- 集群可从应用市场安装 `ack-rollout-server`。本文 values 基于 chart 0.8.0 渲染校验。
- 本机有 kubectl 与 Helm 3，已连接目标集群。
- trial 使用的任务镜像集群可拉取；建议在 `task.toml` 中写明预构建的 `docker_image`（ACS 不能运行
  默认的特权镜像构建 Job，见场景一步骤 4）。

```bash
export ROLLOUT_NAMESPACE=kube-rl
export ROLLOUT_RELEASE=ack-rollout-server
export ROLLOUT_SERVICE=ack-rollout-server
helm repo add aliyunhub https://aliacs-app-catalog.oss-cn-hangzhou.aliyuncs.com/charts-incubator/
helm repo update
```

## 步骤 0 —— 所有场景通用的 values

chart 的 Role 只授权 Release 所在命名空间，而 `config.namespace`（沙箱所在命名空间）默认是
`default`。两者不一致时，服务端没有权限创建沙箱，trial 直接失败。每份 values 都先写上：

```yaml
config:
  namespace: kube-rl          # 必须等于 $ROLLOUT_NAMESPACE
```

以下各场景的 values 片段都在此基础上追加，最终用同一条命令安装或升级：

```bash
helm upgrade --install "$ROLLOUT_RELEASE" aliyunhub/ack-rollout-server \
  -n "$ROLLOUT_NAMESPACE" --create-namespace -f values-acs.yaml
kubectl -n "$ROLLOUT_NAMESPACE" get pod,svc,pvc
# 期望：服务端 Pod Running，PVC Bound。
```

## 场景一 —— 在 ACS 集群中部署 Rollout Server

ACS 集群中所有 Pod 都是 ACS Pod，服务端和沙箱都不需要 nodeSelector 或容忍。要调整的是存储、
资源规格、镜像地址、服务暴露方式，以及沙箱的特权和构建方式。

### 步骤 1 —— 显式指定热数据存储

chart 默认为热数据卷申请 `ReadWriteMany`，且 `storageClassName` 为空。ACS 集群的默认
StorageClass 通常是云盘，不支持 RWX，PVC 会一直 Pending。按部署模式二选一：

```yaml
# single 模式（默认）：单副本 + Recreate 升级，同一时刻只有一个 Pod 挂载，云盘即可。
deploymentMode: single
persistence:
  enabled: true
  nas:
    storageClassName: <云盘 StorageClass>   # kubectl get storageclass
    accessModes: [ReadWriteOnce]
    size: 100Gi
```

```yaml
# multi 模式：ingress / worker / viewer 共享热数据卷，必须是 NAS 等 RWX 存储。
deploymentMode: multi
persistence:
  enabled: true
  nas:
    storageClassName: <NAS StorageClass>
    accessModes: [ReadWriteMany]
redis:
  mode: external
  external:
    existingSecret: <redis Secret>          # 键名 redis-url
```

完成产物归档到 OSS 时，OSS 在 ACS 上只支持静态卷：先建 PV/PVC，再写
`persistence.archive.existingClaim`。

### 步骤 2 —— 让资源规格落在 ACS 规格上

ACS 把 Pod 内所有容器的资源累加，向上取整到最近的受支持规格并按此计费，多出的资源分给第一个
容器。single 模式的服务端 Pod 有 server 和 Redis 两个容器，requests 与 limits 设成相同值，
并让合计正好是一个规格。下例合计 2 vCPU / 4 GiB：

```yaml
resources:
  requests: {cpu: "1.5", memory: 3Gi}
  limits:   {cpu: "1.5", memory: 3Gi}
redis:
  resources:
    requests: {cpu: 500m, memory: 1Gi}
    limits:   {cpu: 500m, memory: 1Gi}
```

multi 模式分别调整 `multi.ingress.resources`、`multi.worker.resources`、`viewer.resources`。
chart 没有服务端 Pod 的自定义 label 参数，服务端固定使用默认的 `general-purpose` 算力。

### 步骤 3 —— 镜像地址

应用市场控制台安装时会自动替换 `__ACK_REGION_ID__`；Helm CLI 安装需要自己写。服务端镜像前缀和
Redis 镜像地址是两个独立的参数，要一起改。ACS Pod 通过 VPC 拉镜像，集群没有 NAT 网关时改用
`-vpc` 域名：

```yaml
global:
  imagePrefix: registry-cn-hangzhou-vpc.ack.aliyuncs.com
redis:
  image:
    repository: registry-cn-hangzhou-vpc.ack.aliyuncs.com/acs/redis
```

有 NAT 网关时可以去掉 `-vpc`。

### 步骤 4 —— 沙箱 Pod

沙箱同样是 ACS Pod。需要指定算力类型时，通过 `pod_overrides` 加标签：

```yaml
config:
  namespace: kube-rl
  pod_overrides:
    metadata:
      labels:
        alibabacloud.com/compute-class: general-purpose   # 或 performance
        alibabacloud.com/compute-qos: default             # best-effort 更便宜，但可能被抢占
```

沙箱的 CPU、内存、临时存储请求来自 `task.toml` 的 `cpus`、`memory_mb`、`storage_mb`，同样按
规格取整计费；临时存储超过 30 GiB 的部分另外计费。

ACS 不允许特权容器，这影响两处：

- 不要在 `pod_overrides` 里写 `privileged: true`，沙箱 Pod 会启动失败。
- 任务镜像不存在时，服务端默认起一个特权 DinD Job 构建镜像，在 ACS 上起不来。改用预构建的
  `docker_image`，或改走远端 BuildKit：

  ```yaml
  config:
    use_buildkit: true
    buildkit_address: tcp://<buildkitd 地址>:1234
  ```

### 步骤 5 —— 暴露服务

默认 Service 为 `ClusterIP`，训练任务在集群内访问
`ack-rollout-server.kube-rl.svc.cluster.local:8080`。ACS 不支持 NodePort，集群外访问用
LoadBalancer，并同时开启 `auth.existingSecret` 认证：

```yaml
service:
  type: LoadBalancer
auth:
  existingSecret: <auth Secret>             # 键名 tokens
```

### 步骤 6 —— 验证

```bash
kubectl -n "$ROLLOUT_NAMESPACE" get pod -l app.kubernetes.io/name=ack-rollout-server \
  -o custom-columns='NAME:.metadata.name,INSTANCE:.metadata.annotations.alibabacloud\.com/instance-id,SPEC:.metadata.annotations.alibabacloud\.com/pod-use-spec'
# 期望：INSTANCE 以 acs- 开头；SPEC 为 2-4Gi。

kubectl -n "$ROLLOUT_NAMESPACE" port-forward svc/"$ROLLOUT_SERVICE" 8080:8080 &
curl -s http://localhost:8080/health | python3 -m json.tool
# 期望：status 为 ok，workers 非空。
```

然后按[用 RolloutServer 管理轨迹](../3-rolloutserver-trajectory-management/README_ZH.md)
步骤 4 提交一个 `oracle` 任务，reward 为 1.0 说明沙箱链路正常。

## 场景二 —— ACK 集群借用 ACS 算力运行沙箱

服务端留在 ECS 节点上，只把 trial 沙箱放到 ACS。需要 ACK 托管集群 Pro 版（Kubernetes ≥ 1.26）、
已开通并授权 ACS、ACK Virtual Node 组件 ≥ v2.13.0。

### 步骤 1 —— 用标签把沙箱调度到 ACS

```yaml
config:
  namespace: kube-rl
  pod_overrides:
    metadata:
      labels:
        alibabacloud.com/acs: "true"
        alibabacloud.com/compute-class: general-purpose
        alibabacloud.com/compute-qos: default
```

| 标签 | 是否必填 | 说明 |
| --- | --- | --- |
| `alibabacloud.com/acs` | 是 | `"true"`，调度器把沙箱 Pod 交给 ACS。 |
| `alibabacloud.com/compute-class` | 否（默认 `general-purpose`） | 算力类型，例如 `performance`。 |
| `alibabacloud.com/compute-qos` | 否（默认 `default`） | `best-effort` 更便宜但可能被抢占，适合可重试的 rollout。 |

`pod_overrides` 只合并进沙箱 Pod，不作用于镜像构建 Job，构建仍在 ECS 节点上进行。

不要改用 `config.node_selector: {type: virtual-kubelet}` 加 `config.tolerations`：

- 服务端把 `node_selector` 和 `tolerations` 同时用于特权 DinD 构建 Job，Job 会被调度到虚拟节点后
  起不来。
- 落到虚拟节点却没声明 ACS 算力类型的 Pod，默认用 ECI 而不是 ACS。

只有沙箱要挂 `WaitForFirstConsumer` 模式 StorageClass 的云盘时才用 nodeSelector 方式，因为
`alibabacloud.com/acs` 标签不支持这类 StorageClass；此时必须同时写 `compute-class` 标签。

### 步骤 2 —— 服务端调度（可选）

服务端 Pod 默认在 ECS 节点上。节点有污点时用顶层参数，这些参数不影响沙箱：

```yaml
tolerations:
  - key: CriticalAddonsOnly
    operator: Equal
    value: "true"
    effect: NoSchedule
```

### 步骤 3 —— 验证

提交一个任务，在 trial 运行期间查看沙箱 Pod：

```bash
kubectl -n "$ROLLOUT_NAMESPACE" get pod -l app=sandbox \
  -o custom-columns='NAME:.metadata.name,NODE:.spec.nodeName,INSTANCE:.metadata.annotations.alibabacloud\.com/instance-id'
# 期望：NODE 以 virtual-kubelet- 开头，INSTANCE 以 acs- 开头。
```

## 场景三 —— 使用 ACS Agent Sandbox 运行沙箱

Agent Sandbox 用 SandboxSet 维护预热池，trial 直接领用已就绪的沙箱。集群要求：

- ACS 集群：已开通 Agent Sandbox，ACS Virtual Node ≥ v2.17.0。
- ACK 集群：ACK Virtual Node ≥ v2.13.0。
- 两者都需 ack-agent-sandbox-controller ≥ v0.5.14-release.1；E2B 方式另需
  ack-sandbox-manager ≥ v0.6.0。

Rollout Server 有两种接入方式：

| 方式 | 环境后端 | 预热池由谁创建 | 适用 |
| --- | --- | --- | --- |
| SandboxClaim | `ACKEnvironment` + `use_sandbox_claim: true` | 服务端按任务环境自动创建 | 任务环境多、希望自动建池 |
| E2B API | `E2BEnvironment` | 预先手工创建 | 已有统一的沙箱池，或复用 E2B 生态 |

### 步骤 1a —— SandboxClaim 方式

服务端为每个任务环境查找同名 SandboxSet，没有就创建，再用 SandboxClaim 领用沙箱并在其中执行
agent。Agent Sandbox 要求 Pod 模板带以下三个标签，用 `pod_overrides` 写入模板：

```yaml
config:
  namespace: kube-rl
  use_sandbox_claim: true
  claim_timeout: 300
  pod_overrides:
    metadata:
      labels:
        alibabacloud.com/acs: "true"
        alibabacloud.com/compute-class: agent-sandbox
        alibabacloud.com/compute-qos: default
```

| 参数 | 是否必填 | 说明 |
| --- | --- | --- |
| `config.use_sandbox_claim` | 是 | `true`，改用 SandboxSet + SandboxClaim。 |
| `config.claim_timeout` | 否（默认 `300`） | 等待领用沙箱的秒数。 |
| `config.pod_overrides.metadata.labels` | 是 | 写入 SandboxSet 的 Pod 模板；`compute-class` 必须是 `agent-sandbox`。 |
| `config.sandbox_labels` | 否 | 同时写入 SandboxSet、SandboxClaim 和 Pod 模板，适合业务标识。 |
| `config.sandbox_env_vars` | 否 | 领用沙箱时注入的环境变量。 |

自动创建的 SandboxSet 在 `config.namespace` 中，默认 5 个预热沙箱，预热中的沙箱同样计费。
服务端不修改已存在的同名 SandboxSet，要控制池大小就预先创建同名 SandboxSet。
SandboxClaim 方式不走 DinD 构建，缺镜像时用预构建 `docker_image` 或 BuildKit。

### 步骤 1b —— E2B API 方式

ack-sandbox-manager 提供 E2B 兼容 API，服务端按 SandboxSet 名称领用沙箱，不直接创建 Pod，
因此 `pod_overrides` 在这种方式下不生效，沙箱规格完全由 SandboxSet 决定。

先按 Agent Sandbox 文档创建 SandboxSet。模板必须带三个 ACS 标签，`spec.runtimes` 必须含
`agent-runtime`：

```yaml
apiVersion: agents.kruise.io/v1alpha1
kind: SandboxSet
metadata:
  name: harbor-e2b-pool
  namespace: default
spec:
  runtimes:
    - name: agent-runtime
  replicas: 4
  template:
    metadata:
      labels:
        alibabacloud.com/acs: "true"
        alibabacloud.com/compute-class: agent-sandbox
        alibabacloud.com/compute-qos: default
    spec:
      containers:
        - name: sandbox
          image: <基础镜像>
          resources:
            requests: {cpu: "1", memory: 2Gi}
            limits:   {cpu: "1", memory: 2Gi}
```

把 ack-sandbox-manager 的 API Key 放进 Release 命名空间的 Secret，再声明 E2B 后端：

```bash
kubectl -n "$ROLLOUT_NAMESPACE" create secret generic e2b-credentials \
  --from-literal=api-key=<E2B_API_KEY>
```

```yaml
config:
  namespace: kube-rl
  backends:
    - name: e2b-pool
      import_path: harbor.environments.e2b:E2BEnvironment
      default: true
      queues: ["default"]
      defaults:
        sandbox_set_name: harbor-e2b-pool
        override_claim_image: true
      allow_overrides: [sandbox_set_name, override_claim_image]
extraEnv:
  - name: E2B_API_KEY
    valueFrom:
      secretKeyRef: {name: e2b-credentials, key: api-key}
  - name: E2B_API_URL
    value: http://sandbox-manager.sandbox-system.svc.cluster.local:8080
  - name: E2B_SANDBOX_URL
    value: http://sandbox-gateway.sandbox-system.svc.cluster.local:7788
```

| 参数 | 是否必填 | 说明 |
| --- | --- | --- |
| `defaults.sandbox_set_name` | 是 | 领用的 SandboxSet 名称。不设时客户端尝试构建模板，返回 404。 |
| `defaults.override_claim_image` | 否（默认 `true`） | 把沙箱镜像原地换成 `task.toml` 的 `docker_image`；`false` 时用 SandboxSet 自身镜像。 |
| `allow_overrides` | 否 | 提交请求可通过 `environment_kwargs` 覆盖的参数，默认不放通。 |
| `E2B_API_KEY` | 是 | 只经 Secret 注入。不要写进 `config.*`，`config.*` 会渲染到命名空间内可读的 ConfigMap。 |
| `E2B_API_URL`、`E2B_SANDBOX_URL` | 集群内访问时必填 | sandbox-manager 与 sandbox-gateway 的集群内地址；集群外访问改设 `E2B_DOMAIN`。 |

多套沙箱池用不同凭据时，改用 `config.env_profiles`，以 `${变量名}` 引用 `extraEnv` 注入的变量。

### 步骤 2 —— 验证

```bash
kubectl get sbs -A
# 期望：对应 SandboxSet 的 AVAILABLE 大于 0。

kubectl -n "$ROLLOUT_NAMESPACE" port-forward svc/"$ROLLOUT_SERVICE" 18080:8080 &
harbor run --path ./my-task --agent oracle \
  --rollout-server http://127.0.0.1:18080
# 期望：trial 终态 completed，reward 为 1.0。
```

## 排障

| 症状 | 检查与处理 |
| --- | --- |
| 热数据 PVC 一直 Pending | 默认 SC 是云盘，不支持 RWX。single 模式改 `accessModes: [ReadWriteOnce]` 并指定云盘 SC；multi 模式指定 NAS SC。 |
| ACS 中服务端 `ImagePullBackOff` | 无 NAT 时 `global.imagePrefix` 和 `redis.image.repository` 都要改成 `-vpc` 域名，只改一个仍会失败。 |
| trial 失败，日志有 `Forbidden` | `config.namespace` 与 Release 命名空间不一致，Role 不覆盖沙箱所在命名空间。 |
| 镜像构建 Pod 起不来 | ACS 拒绝特权容器。改用预构建 `docker_image`，或配置 `use_buildkit` + `buildkit_address`。 |
| 沙箱在虚拟节点上，但实例 ID 不以 `acs-` 开头 | 落到了 ECI：没有声明 ACS 算力类型，补 `alibabacloud.com/acs` 或 `compute-class` 标签。 |
| 沙箱 Pending，`untolerated taint` | 用了 `node_selector: {type: virtual-kubelet}` 却没配 `virtual-kubelet.io/provider` 容忍；推荐改用标签方式。 |
| 修改 `pod_overrides` 后，SandboxClaim 沙箱仍是旧配置 | 服务端不更新已存在的 SandboxSet。确认没有在跑的 trial 后删除对应 SandboxSet，下次提交时按新配置重建。 |
| SandboxClaim 超时 | `kubectl get sbs` 查看 AVAILABLE；池为空或补货慢时调大 `claim_timeout` 或预建更大的池。 |
| E2B 提交返回 404 | 没设 `sandbox_set_name`，E2B 方式必须指定预热池。 |
| `connection refused on port 49983` | SandboxSet 缺少 `spec.runtimes: [{name: agent-runtime}]`，沙箱内没有 envd。 |
| 只改了 Secret，配置没生效 | Pod 只随 ConfigMap 变化自动滚动；`kubectl -n "$ROLLOUT_NAMESPACE" rollout restart deploy/"$ROLLOUT_RELEASE"`。 |

## 说明

- ACS 按取整后的规格计费，`pod-use-spec` 注解显示实际规格。规划沙箱规格时以 `task.toml` 资源为准。
- `best-effort` 沙箱被抢占时 trial 失败，只用于可重试的 rollout。
- 预热池中的空闲沙箱持续计费。SandboxClaim 方式每个任务环境一个池，任务环境多时总量会放大；
  环境统一时优先用 E2B 方式共享一个池，配合 `override_claim_image` 换镜像。
- 不要覆盖沙箱的 `app` 标签：服务端按 `app=sandbox,session` 回收遗留沙箱。
- 服务端 Pod 不能通过 values 设置 `compute-class`，在 ACS 集群中固定使用 `general-purpose`。

## 相关文档

- [ACS Pod 实例概述](https://help.aliyun.com/zh/cs/user-guide/acs-pod-instance-overview)
- [ACK 托管集群 Pro 版接入 ACS 算力](https://help.aliyun.com/zh/cs/user-guide/access-acs-computing-power-in-an-ack-cluster)
- [ACK Pro 集群支持 ECS 和 ACS 算力混合调度](https://help.aliyun.com/zh/cs/user-guide/ack-cluster-supports-mixed-scheduling-of-ecs-and-acs-computing)
- [在 ACS 集群中创建 Agent Sandbox](https://help.aliyun.com/zh/cs/user-guide/create-an-agent-sandbox)
- [使用 E2B SDK 接入 Agent Sandbox](https://help.aliyun.com/zh/cs/user-guide/connect-to-agent-sandbox-using-the-e2b-sdk)
- [免密拉取 ACR 镜像](https://help.aliyun.com/zh/cs/user-guide/exempting-the-miracular-acr-image)
