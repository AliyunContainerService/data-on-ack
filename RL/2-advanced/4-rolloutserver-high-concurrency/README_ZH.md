# 支持数万Rollout并发

本文介绍如何配置 RolloutServer（kube-rl），使其支持 1 万至 5 万个 rollout 并发执行。内容涵盖分布式架构、部署拓扑、并发与配额配置、资源规划、沙箱容量准备，以及部署后的验证与监控方法。

本文以下数据来自实际测试，测试条件为：单 Pod（1 个 ingress 进程 + 4 个 worker 进程），`max_concurrent_trials=10000`，Oracle agent，hostNetwork，最小化资源请求。

## 分布式架构

RolloutServer 由以下角色组成，各角色可以部署在同一 Pod 内，也可以独立扩缩：

- **ingress**：接收 HTTP 提交，执行准入检查（队列深度、提交速率），将准入的 run 写入 pending pool。
- **中央调度器**：运行在 ingress 进程中。多个 ingress 进程之间通过 Redis lease 选举 leader，由 leader 负责 dispatch：每个 tick 按优先级和 queue/job 配额从 pending pool 挑选 run，投递到共享任务队列。
- **worker**：从共享任务队列领取任务，在沙箱后端（ACK Pod、E2B 或 SSH）中执行 trial。领取前先占用本地并发槽位，执行结束后归还。
- **viewer**（可选）：只读的 job/trial 查询接口与 UI。
- **Batch Controller**（可选）：数据集批量提交服务。

Redis 是所有角色的协调中心：任务队列、配额计数、调度器 pending pool 与 leader lease 都存储在 Redis 中。未配置 Redis 时，server 拒绝启动。trial 状态与结果写入共享存储卷（NAS），完成后的轨迹可归档到 OSS。

一条请求的处理流程：

```
提交 → ingress 准入（max_queued / rate_limit）
     → pending pool（按优先级排序）
     → leader dispatch（queue + job max_concurrent 门控）
     → 共享任务队列 → worker 领取（领取前占坑）
     → 沙箱执行 → 结果归档
```

这一架构决定了扩容方式：

- 所有 worker 从同一队列领取任务，领取前占坑机制保证任务均匀分配。增加 worker 即可线性提高总并发，不存在任务亲和或迁移问题。
- multi 模式下 ingress 也可以独立扩容；调度器通过 Redis lease 自动选举 leader，没有单点。
- single 模式将上述角色集中在一个 Pod 内运行；multi 模式将 ingress 部署为 Deployment、worker 部署为 StatefulSet，独立扩缩（见"选择部署模式"）。

## 容量概述

单 Pod 在不同内存限制下的实测并发能力：

| Pod 内存限制 | 安全并发上限 | 峰值内存 | 备注 |
|---|---|---|---|
| 4 Gi | 约 1000 | 约 3.5 Gi | 更高并发会触发 OOM |
| 8 Gi | 约 3000 | 约 5.5 Gi | 线程池达到上限后仍有余量 |
| 16 Gi | 约 10000 | 7.0 Gi（44%） | 实测 9918 个并发 Running，无 OOM、无重启 |

总并发数由以下公式决定：

```
总并发 = max_concurrent_trials × worker 进程总数
```

- single 模式：worker 进程总数 = `single.workers`。
- multi 模式：worker 进程总数 = `multi.worker.replicas`（每个 Pod 运行一个 worker 进程）。

`max_concurrent_trials` 的默认值为 4。部署大规模并发前，先显式设置该值。

按目标并发选择路径：

- 目标不超过 1 万：使用 single 模式，单 Pod 配置 16 Gi 内存、8 CPU。
- 目标超过 1 万：使用 multi 模式，横向扩展 worker StatefulSet，并配置 external Redis 和 RWX 存储卷。worker 在领取任务前先占用本地并发槽位，任务在共享队列中均匀分配，因此横向扩容不会导致任务分配不均。

## 前提条件

开始配置前，确认以下条件已满足：

- 一个 ACK 集群，包含大规格节点池（建议 16 核以上、64 Gi 以上内存、500 Gi SSD）。
- 一个可用的 Redis 实例。Redis 是 RolloutServer 的必需依赖：任务队列、配额计数、调度器状态均存储在 Redis 中。未配置 `redis_url` 时，server 拒绝启动。
- multi 模式需要一个 ReadWriteMany（RWX）存储卷，例如 `alicloud-nas-fs` 或 `alicloud-cpfs`。集群默认的块存储不满足要求。
- RolloutServer 镜像及对应的 imagePullSecret。

## 选择部署模式

| Value | 默认值 | 说明 |
|---|---|---|
| `deploymentMode` | `single` | single：一个 Pod 内运行 N 个 ingress 进程和 M 个 worker 进程。multi：独立的 ingress Deployment 和 worker StatefulSet |
| `single.ingressProcs` | 2 | 每个 Pod 的 ingress 进程数，进程间通过 SO_REUSEPORT 共享端口 |
| `single.workers` | 4 | 每个 Pod 的 worker 进程数 |
| `multi.ingress.replicas` | 2 | ingress Deployment 副本数 |
| `multi.worker.replicas` | 4 | worker StatefulSet 副本数，Pod 序号即 worker id |
| `noopMode` | false | 跳过 tar 解压、Pod 创建和 trial 执行，仅测试分发路径 |

single 模式强制 `replicaCount=1`。设置大于 1 时，Helm 渲染失败，因为多个 Pod 的 worker id 会冲突。

### 配置 single 模式（1 万并发）

以下 values 已通过容量验证：

```yaml
deploymentMode: single
single:
  ingressProcs: 2
  workers: 4

config:
  namespace: kube-rl
  max_concurrent_trials: 10000
  rate_limit_enabled: false

resources:
  requests: { cpu: "1", memory: 512Mi }
  limits:   { cpu: "8", memory: 16Gi }

redis:
  mode: sidecar

persistence:
  enabled: true
  nas:
    size: 200Gi
    storageClassName: alicloud-nas-fs

viewer:
  enabled: true

image:
  repository: <registry>/ack-rollout-server
  tag: <8-char-tag>
imagePullSecrets:
  - name: <pull-secret>
```

sidecar Redis 的数据存储在 emptyDir 中，Pod 删除后队列丢失。生产环境使用 external Redis。

### 配置 multi 模式（数万并发）

以下示例配置 8 个 worker、每个 worker 5000 并发，总并发上限 4 万：

```yaml
deploymentMode: multi
multi:
  ingress:
    replicas: 4
    resources:
      requests: { cpu: "2", memory: 4Gi }
      limits:   { cpu: "8", memory: 8Gi }
  worker:
    replicas: 8
    resources:
      requests: { cpu: "2", memory: 8Gi }
      limits:   { cpu: "8", memory: 16Gi }

config:
  namespace: kube-rl
  max_concurrent_trials: 5000
  rate_limit_enabled: false
  quotas:
    default:
      max_concurrent: 40000
      max_queued: 100000

redis:
  mode: external
  external:
    existingSecret: kube-rl-redis   # key: redis-url，值为完整连接 URL

persistence:
  enabled: true
  nas:
    existingClaim: kube-rl-hot-nas
  archive:
    existingClaim: kube-rl-archive

viewer:
  enabled: true
batchController:
  enabled: true
  maxConcurrentSubmissions: 100
```

chart 默认的资源限制为 2 CPU、2 Gi，不足以支撑万级并发。按"规划内存资源"一节计算每个 Pod 的资源。

ConfigMap 变更通过 `checksum/config` 注解自动触发滚动更新。仅修改 Secret（token、redis-url）时，手动执行 `kubectl rollout restart`。

## 配置 worker 并发

1. 设置 `config.max_concurrent_trials` 为每个 worker 进程的目标并发数。该值同时决定执行信号量、领取槽位信号量和 k8s-exec 线程池的大小。
2. 设置 worker 进程数（`single.workers` 或 `multi.worker.replicas`），使两者乘积等于目标总并发。
3. 如果在 macOS 上进行本地测试，调大 OS 的 accept queue：`sudo sysctl -w kern.ipc.somaxconn=4096`。Linux 默认值 4096 已满足要求。

相关配置项：

| 配置项 | 默认值 | 说明 |
|---|---|---|
| `max_concurrent_trials` | 4 | 每 worker 并发上限 |
| `exec_api_pool_size` | 256 | K8s exec WebSocket 连接池上限（ACK 后端） |
| `listen_backlog` | 4096 | ingress listen backlog，受 OS somaxconn 限制 |
| `use_async_exec` | false | 以 nohup+poll 代替 kubectl exec 长连接，减少 WebSocket 占用 |
| `shutdown_grace_period` | 30s | 优雅停机等待时长，超时的任务重新入队 |
| `async_status_cache_max_entries` | 5000 | 每进程内存 status 缓存上限 |
| `pod_health_check_interval_sec` | 30s | 检测被删除的 Pod，取消卡住的 trial |
| `pod_health_check_page_size` | 0（全量 list） | 万级 Pod 时设置分页（如 500），避免每次解析 50–100MB 响应 |
| `reconciler_interval_sec` | 300（chart 覆盖为 60） | 孤儿 Pod 回收间隔 |
| `max_orphan_pod_age_sec` | 7200（chart 覆盖为 600） | 孤儿 Pod 年龄阈值 |
| `rate_limit_enabled` | false | per-IP 限流开关 |
| `rate_limit_submit` / `rate_limit_default` | 10/minute / 60/minute | 提交与其他请求的限流 |

以上配置可通过环境变量覆盖：`KUBE_RL_MAX_CONCURRENT_TRIALS`、`KUBE_RL_NUM_WORKERS`、`KUBE_RL_NUM_INGRESS`、`KUBE_RL_LISTEN_BACKLOG`。

以下限制为硬编码，不可配置。规划容量时将其计入：

- worker k8s-exec 线程池大小为 `min(max(32, max_concurrent_trials×4), 256)`，上限 256。实测调大该上限会降低性能：apiserver 是瓶颈，256 起到背压作用。
- Redis 客户端 `max_connections=128`，LPUSH 并发限制为 `Semaphore(64)`。这是经过验证的背压组合，不要调大。
- ingress HTTP 线程池 32，文件 I/O 线程池 16，tar 解压线程池 16。
- ACK K8s 连接池 256。`max_concurrent_trials` 超过 256 时，Pod 创建请求在连接池层排队，属于正常限流。

## 规划内存资源

每个 worker 进程在 1 万并发水位下增加约 1.4 Gi 内存。按以下规则规划：

```
Pod 内存 ≥ 基线 2.5 Gi + worker 进程数 × 1.5 Gi，另留 50% 余量
```

single 模式 4 个 worker 对应 16 Gi。multi 模式下每个 worker Pod 独立配置 16 Gi。

4 Gi 内存下 OOM 的原因：

```
max_concurrent_trials=10000
  → k8s-exec 线程池 = 256 线程/worker
  → 4 worker × 256 线程 × 约 1MB 栈 ≈ 1 Gi
  → 基线 2.5 Gi + 线程 1 Gi + 并发元数据 > 4 Gi → OOM
```

16 Gi 下 1 万并发的峰值内存为 7.0 Gi：基线 2.5 Gi + 线程 1 Gi + 元数据 3.5 Gi。

每个 worker 约 293 个线程（k8s-exec 256、io-aux 16、tar 6、gRPC 12 等）。这是 ThreadPoolExecutor 按需创建线程后不回收的正常行为，不是泄漏。

## 配置队列配额

配额分两层，queue 维度和 job 维度同时生效：

| 层 | 字段 | 行为 |
|---|---|---|
| ingress 准入 | `max_queued` | 队列深度上限，超出返回 429 |
| ingress 准入 | `rate_limit` | 提交速率滑窗（"N/second\|minute\|hour"），超出返回 429 |
| 调度器/worker | `max_concurrent` | running + detached 数量上限，超出在 pending pool 排队 |

在 values 中配置：

```yaml
config:
  quotas:
    default: { max_concurrent: 40000, max_queued: 100000 }
    rl:        { max_concurrent: 30000, max_queued: 80000 }
    evaluation: { max_concurrent: 10000, max_queued: 20000 }
  job_quota:
    max_concurrent: 5000
```

配置时注意以下行为：

- `default` 队列始终存在。未在 `quotas` 中列出的队列名不会被拒绝，但没有队列级限制。大规模提交前，为使用的队列显式设置限制。
- 配置文件中的 queue spec 只接受 `max_concurrent`、`max_queued`、`rate_limit` 三个键。`default_priority` 和 `max_priority` 通过 API 设置。
- `PUT /api/v1/quotas/{queue}` 的修改仅保存在内存中，重启后回落到配置值。
- `PUT /api/v1/job-quotas/{job_id}` 的修改存储在 Redis（30 天 TTL），且整体替换 spec。只发送部分字段会清除其余字段。
- worker 每 30s（`quota_heartbeat_interval_sec`）写一次心跳。心跳超过 300s（`quota_stale_run_threshold_sec`）时，ingress 的对账循环（每 60s，`quota_reconcile_interval_sec`）将该 run 标记为 failed 并回收槽位。worker 崩溃导致槽位占用时，执行 `POST /api/v1/quotas/reconcile` 手动对账。

## 配置中央调度器

中央调度器始终启用，没有开关。每个 ingress 进程都运行调度器，只有持有 Redis lease 的进程（leader）执行 dispatch。

| 配置项 | 默认值 | 说明 |
|---|---|---|
| `scheduler_tick_interval_sec` | 1.0 | leader 每个 tick 重算容量并 dispatch |
| `scheduler_lease_ttl_sec` | 15 | leader lease TTL |
| `scheduler_dispatch_timeout_sec` | 60 | promote 后 60s 未进入 running 时重新投递（run_id 去重保证幂等） |
| `priority_policy` | `static` | static：纯优先级，同级 FIFO。aging：按等待时间提升有效优先级 |
| `priority_aging_step_sec` | 60 | 每等待 60s，有效优先级 +1 |
| `priority_aging_max_boost` | 100 | 提升上限。设置为大于业务中最大优先级差，否则饱和后回到纯优先级排序 |

优先级解析顺序为 run > job default > queue default，受 queue `max_priority` 封顶。排序是插入式的，不抢占：高优先级 run 也需要等待运行中的 trial 释放槽位。

通过 `GET /api/v1/scheduler` 观察调度器状态（is_leader、pending 深度、active、inflight）。只有 leader 进程报告 queues 字段。

数万 queued 时的调度器行为：leader 每个 tick 对每个队列执行全量 `ZRANGE`，并对每个 run 执行一次 `HGETALL`（串行 await）。3 万 queued 约等于每秒 3 万次 Redis 往返，这是调度吞吐的主要瓶颈。缓解方法：

- 用 `max_queued` 控制 pending 深度，客户端分批提交。
- 按业务拆分多个 queue，分散单队列深度。
- 调大 `scheduler_tick_interval_sec`（如 2–5s）可以降低 Redis 负载，但不提高吞吐。dispatch 没有每 tick 批量上限。

## 配置 Redis

single 模式可使用 sidecar Redis（演示和压测场景）。multi 模式和生产环境使用 external Redis：

```yaml
redis:
  mode: external
  external:
    existingSecret: kube-rl-redis   # key 必须是 redis-url，值为完整 URL
```

`redis.mode: none` 会导致 Helm 渲染失败。

客户端使用 `redis.asyncio`，带超时重试和断连重连。`redis_socket_timeout` 和 `redis_socket_connect_timeout` 默认 5s，`redis_health_check_interval` 默认 30s。

数万规模时，独立部署 external Redis（主从或集群版），监控连接数与内存。pending pool 中每个 run 保存一条完整提交消息（meta HASH），数万 queued 时关注 Redis 内存水位。

## 配置存储

| 配置 | 说明 |
|---|---|
| `persistence.nas`（RWX） | 热卷：run store、活跃任务归档、trial 输出。使用 NAS/CPFS，按峰值并发 × 保留期规划容量 |
| `persistence.archive.existingClaim` | OSS 静态 PVC，映射到 `config.archive_dir`。完成的 trial 写入单个 `{run_id}.tar.gz` 和 `{run_id}.trajectory.json`。数万轨迹场景启用该配置，否则 run store 持续增长。OSS CSI 不支持动态供给，PVC 需预先创建 |
| `archive_after_seconds` / `archive_check_interval` | 完成 3600s 后归档；每 300s 扫描一次 |
| `persistence.enabled: false` | 使用 emptyDir，仅用于演示，重启后结果丢失 |

数万并发下的元数据 I/O 开销：

- 每个 running run 每 30s 写一次 status.json 心跳：1 万 running 约 333 次写/秒。
- ingress 对账每 60s 逐 run 读取 status.json：数万 queued 即数万次读/分钟。
- 每个进程启动时全量扫描 run store 加载 status 缓存。缓存上限 5000 条，活跃条目不可驱逐；数万活跃时，状态查询回退到 NAS 读取。

为热卷选择低延迟 NAS/CPFS，并启用归档将冷数据移到 OSS。

## 准备 ACK 沙箱容量

ACK 后端没有独立的并发上限参数。实际限制链为：`max_concurrent_trials` → k8s-exec 线程池（256）→ K8s 连接池（256）→ apiserver 吞吐 → 节点 maxPods 与 CNI。

按以下步骤准备集群：

1. 为大规格节点打标签：`kubectl label node <node-name> nodetype=big`。
2. 将节点 maxPods 从默认 213 调整为 1000：修改 `/var/lib/kubelet/ack-managed-config.yaml` 后重启 kubelet。
3. 在提交请求中启用 hostNetwork，消除 CNI 配置延迟：

```json
"environment_kwargs": {
  "node_selector": {"nodetype": "big"},
  "skip_image_check": true,
  "pod_overrides": {"spec": {"hostNetwork": true, "dnsPolicy": "ClusterFirstWithHostNet"}}
}
```

4. 压测场景下最小化资源请求（默认 cpus=1、memory_mb=2048，万级并发会耗尽节点可调度资源）：

```json
"environment_overrides": {"override_cpus": 0, "override_memory_mb": 10, "override_storage_mb": 100}
```

资源最小化仅适用于压测和轻负载任务。真实 agent 任务按实际需要配置资源，并据此重新计算节点容量。

暖池（SandboxClaim）相关参数：`use_sandbox_claim`、`sandboxset_replicas`（默认 5）、`claim_timeout`（300s）、`sandbox_ready_timeout_sec`（120s）。

## 准备 E2B 沙箱容量

E2B 后端的容量由 SandboxSet 暖池决定，与 server 配置无关。调大 `max_concurrent_trials` 不会增加沙箱数量：超出池容量的 trial 阻塞在 claim，`claim_timeout`（300s）后失败。

扩容暖池：

```bash
kubectl scale sandboxset <name> --replicas=<目标并发>
```

提交请求时注意：

- 必须携带 `sandbox_set_name`，否则 E2B 尝试构建模板并返回 404。
- `override_claim_image: true` 使任务自带的 `docker_image` 覆盖池镜像。该选项在部分集群上不稳定；稳妥做法是为每个镜像建立独立的 SandboxSet。
- `E2B_API_KEY`、`E2B_API_URL`、`E2B_SANDBOX_URL` 通过 `config.env_profiles` 注入。env_profiles 只接受基础设施凭据，不接受模型 key。

## 批量提交 rollout

按场景选择提交路径：

- 数万任务：使用 async API（`POST /api/v1/runs/async` + 轮询）或 SDK `run_async_task()`。sync 端点在整个 trial 期间持有 HTTP 连接，不适用于万级并发。
- 数据集已在集群上：使用 Batch Controller（`POST /api/v1/batches`，端口 8082），并调大 `batchController.maxConcurrentSubmissions`（默认 10）。
- RL 训练循环：使用 SDK `run_async_task()`。

批量提交前调整两个节流点：

- `rate_limit_submit`：默认 per-IP 10/minute。调大该值或设置 `rate_limit_enabled: false`。
- `batchController.maxConcurrentSubmissions`：默认 10。

客户端控制在途提交数量（例如 `asyncio.Semaphore(200)`），按速率分批提交，并对 `max_queued` 触发的 429 重试。

## 验证部署

按以下顺序验证：

1. 检查 `GET /health`，确认 server 可达且至少有 1 个 worker。
2. 检查 `GET /api/v1/scheduler`，确认 `enabled: true` 且存在 leader。
3. 提交一条 `agent=oracle` 的 rollout，确认状态到达 `completed` 且 reward 非零。
4. 阶梯压测（100 → 1000 → 5000 → 目标并发），每档观察内存、线程数、loop_lag 和 429 比例。
5. 删除一个 worker Pod，确认 processing list 恢复和槽位回收（心跳 300s + 对账）。

压测客户端建议以多进程方式提交（每个进程持有独立的 event loop 和连接池），突破单进程提交上限。只测试分发路径时，设置 `noopMode: true`。

真实 Pod 容量压测可使用 Oracle agent + `sleep` 保持运行 + hostNetwork + 资源最小化的任务，提交脚本以 Job 形式在集群内运行，避免本地网络限制。

## 监控运行状态

```bash
curl -s $SERVER/health                    # worker 列表与状态
curl -s $SERVER/debug/memory              # ingress RSS 与线程数
curl -s $SERVER/metrics                   # Prometheus 指标（rollout_server_* 前缀）
curl -s $SERVER/api/v1/scheduler          # leader、pending 深度、inflight
curl -s $SERVER/api/v1/quotas             # 各队列 queued/active
kubectl top pod -n kube-rl                # Pod 内存水位
kubectl get pods -n kube-rl | awk '{print $3}' | sort | uniq -c   # trial Pod phase 分布
```

关注以下告警水位：

| 指标 | 阈值 | 含义 |
|---|---|---|
| `loop_lag` p50 | > 100ms | event loop 被同步工作阻塞 |
| Pod 内存使用率 | > 70% | 接近 OOM 风险区 |
| restartCount | > 0 | 发生过 OOM 或崩溃 |
| pending 深度 | 持续逼近 `max_queued` | 提交速率超过消化能力 |

## 故障排查

按 quota → workers → sandbox pod → task 的顺序排查：

| 症状 | 原因 | 处理 |
|---|---|---|
| 大量 run 停留在 `queued` | `max_concurrent` 已满或 worker 容量不足 | 检查 `/api/v1/quotas` 和 `/health` 的 workers 数组 |
| `queued` 但配额空闲 | worker 崩溃后占用槽位 | 执行 `POST /api/v1/quotas/reconcile` |
| 429 | `max_queued` 已满或触发限流 | 响应体区分两者；调整限制或降低提交速率 |
| Pod 大量 Pending/Unschedulable | 节点资源、maxPods 或 CNI 限制 | 资源 override、maxPods=1000、hostNetwork |
| E2B claim 超时 | SandboxSet replicas 不足 | `kubectl scale sandboxset` |
| OOM 重启 | 内存配置不足 | 按"规划内存资源"一节调整 limits |
| 重启后结果丢失 | `persistence.enabled: false` | 启用 RWX 持久化 |
| 调度延迟随 queued 数上涨 | leader 每 tick 的 O(N) 开销 | 控制深度、拆分队列、分批提交 |

## 已知限制

以下限制在当前版本中无法通过配置消除，规划容量时需要了解其表现和缓解方法。

### 调度开销随排队深度增长

leader 每个 tick（默认 1 秒）全量扫描 pending pool，并对每个 run 发起一次 Redis 查询。3 万个 run 排队时，约等于每秒 3 万次串行 Redis 往返，dispatch 延迟随排队深度明显上升。调大 `scheduler_tick_interval_sec` 只能降低扫描频率，不能减少单次扫描的开销。

缓解方法：用 `max_queued` 控制 pending 深度；按业务拆分多个 queue；客户端分批提交，避免一次性灌入数万个请求。

### NAS 元数据 I/O 放大

每个 running run 每 30 秒写一次心跳文件（1 万 running 约 333 次写/秒）；ingress 对账循环每 60 秒逐 run 读取状态文件，数万 queued 即数万次读/分钟；每个进程启动时还会全量扫描 run store。这些小文件操作集中压在 NAS 元数据性能上：NAS 延迟高时，心跳和状态查询变慢，可能进一步导致 run 被误判超时。

缓解方法：选择低延迟 NAS/CPFS 作为热卷；启用归档卷，将完成后的轨迹移到 OSS；避免冷数据长期留在热卷。

### 沙箱创建速率受 apiserver 限制

worker 的 K8s 连接池和 exec 线程池上限为 256，这是有意的背压设计：apiserver 是共享瓶颈，实测调大上限反而降低性能。`max_concurrent_trials` 超过 256 时，Pod 创建请求在连接池层排队，沙箱启动速度变慢。

缓解方法：启用 hostNetwork 消除 CNI 配置延迟；设置 `skip_image_check` 减少提交前检查；使用暖池（SandboxClaim / SandboxSet）预创建沙箱。

### 状态缓存超容

每进程内存中的 status 缓存上限为 5000 条，且 queued/running 状态的条目不可驱逐。活跃 run 超过 5000 时缓存超容并产生告警，状态查询回退到读取 NAS 文件，查询延迟上升。这是预期行为，不影响结果正确性。

缓解方法：无需处理；对查询延迟敏感时，控制单进程对应的活跃 run 数量。

### Redis 单点与数据持久性

single 模式的 sidecar Redis 数据存放在 emptyDir 中，Pod 删除即丢失队列和配额状态。Redis 自身以单实例部署时也是单点。

缓解方法：生产环境使用 external Redis（主从或集群版）；multi 模式必须使用 external Redis。

### 批量提交的默认节流

两个默认节流点容易被忽略：`rate_limit_submit` 限制单 IP 每分钟提交 10 次；`batchController.maxConcurrentSubmissions` 默认并发 10。灌入数万任务时，这两处会导致提交被 429 拒绝或进度缓慢。

缓解方法：调大 `rate_limit_submit` 或设置 `rate_limit_enabled: false`；调大 `maxConcurrentSubmissions`。