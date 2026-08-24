# 在阿里云上使用Ray GCS 容错

GCS（Global Control Service）是 Ray 的集群级元数据服务，运行在 head Pod 内，负责节点注册、actor 注册表、placement group、Job 元数据和资源管理。**默认这些数据只存在 head 进程的内存里，没有任何副本**。head 进程一退出，集群状态就跟着消失，一次进程级故障会沿下面这条链路放大成整个集群作废：

```
GCS 进程挂掉  → worker 上的 raylet 与 GCS 断连
             → 超过 RAY_gcs_rpc_server_reconnect_timeout_s（head 默认 60 秒）后 head 退出
             → head 重启，但内存里的集群状态已全部丢失
             → 新 head 不认识老 worker，将其终止
             → 集群实际上是重建，而不是恢复
```

开启 GCS 容错后，GCS 在运行期间把状态同步写入外部 Redis，重启后从 Redis 全量加载并继续服务；同时 worker 侧的重连超时被拉长到 600 秒，足以撑过一次 head 重启。head 故障从「集群重建」降级为「一次一分钟左右的控制面抖动」。

```
Ray head Pod                       外部 Redis                    Worker Pod
  GCS 进程  ──── ① 运行期同步写 ───→  RAY<ns>@NODE            全程不重启
  RAY_REDIS_ADDRESS                  RAY<ns>@ACTOR           RAY_gcs_rpc_server_
  REDIS_PASSWORD                     RAY<ns>@KV ...            reconnect_timeout_s=600
  RAY_external_storage_namespace            │                        │
       ↑                                    │                        │
  新 GCS ←──── ② 重启后全量加载 ────────────┘                        │
       └────────────────── ③ raylet 重连 ────────────────────────→ ┘
```

三个数值决定了故障模拟时该等多久：

| 配置项 | 值 | 由谁设置 |
|---|---|---|
| `RAY_gcs_rpc_server_reconnect_timeout_s`（head） | **60** | Ray 默认值，KubeRay 不注入 |
| `RAY_gcs_rpc_server_reconnect_timeout_s`（worker） | **600** | KubeRay 自动注入，必须大于 head 的值 |
| `RAY_external_storage_namespace` | RayCluster 的 **UID** | KubeRay 自动注入，作为 Redis key 前缀 |

本文所有命令与输出均在 ACK 托管版集群上实测：托管 KubeRay Operator v1.5.1，Ray 2.51.0。

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 **v1.3.0 及以上**版本的 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)（`gcsFaultToleranceOptions` 字段在 v1.3.0 引入）
- Ray 镜像版本 **2.0.0 及以上**。
- Redis。单分片 Redis Cluster 或 Redis Sentinel，可以有一个或多个副本

Redis 有两种形态：**集群内自建单副本 Redis**（形态 A，仅用于功能验证）和**云数据库 Tair**（形态 B，生产环境）。除第 1 步的准备工作不同外，后续每一步都会同时给出两者的命令与预期结果。

## 配置说明：`gcsFaultToleranceOptions`

加上这个字段本身就等于启用容错 —— `gcsFaultToleranceOptions: {}` 就是一份合法且已启用的配置：

```yaml
spec:
  gcsFaultToleranceOptions:
    # 形态 A 填 K8s Service 名，形态 B 填 Tair 实例的专有网络连接地址
    redisAddress: "redis:6379"      # 裸 host:port，不带 redis:// 前缀
    redisPassword:
      valueFrom:
        secretKeyRef:
          name: redis-password-secret
          key: password
```

| 字段 | 作用 |
|---|---|
| `redisAddress` | Redis 地址，必须是裸 `host:port`。写成 `redis://host:port` 会被整体当作主机名，head 启动时报 `resolve: Host not found` |
| `redisPassword` | 密码，从 Secret 读取。KubeRay 会自动给 head 启动命令追加 `--redis-password=$REDIS_PASSWORD`，不要在 `rayStartParams` 里重复手写 |
| `externalStorageNamespace` | Redis key 前缀，**保持不设置** |

不设置 `externalStorageNamespace` 时，KubeRay 会用 RayCluster 的 UID 作为默认值，而 UID 全局唯一且不可复用。因此多个 RayCluster 可以放心共用一个 Redis 而互不干扰，同名集群删除重建也不会读到上一代的脏状态。手动设置恰好会破坏这两个性质：两个集群设成同一个值会互相覆盖元数据；在 `RayService` 上设成固定值，升级期间新旧 RayCluster 并存时会抢同一个存储命名空间。

## 1. 准备 Redis

两种形态选一种执行。这一步的产物是后续步骤共用的两样东西：一个存着密码的 Secret `redis-password-secret`，以及一个能执行 `redis-cli` 的 Pod。

### 形态 A：集群内自建 Redis（功能验证）

```bash
kubectl apply -f redis.yaml
kubectl wait --for=condition=available deploy/redis --timeout=120s
export REDIS_POD=$(kubectl get pod -l app=redis -o jsonpath='{.items[0].metadata.name}')
```

预期结果 —— `deployment.apps/redis condition met`，且 Pod 已就绪：

```
NAME                     READY   STATUS    RESTARTS   AGE
redis-6b9d5f8c4d-hq2xn   1/1     Running   0          15s
```

[`redis.yaml`](redis.yaml) 部署一个单副本 Redis，开启 AOF 持久化并设置 `maxmemory-policy noeviction`，密码为 `5241590000000000`。**它只适合功能验证**：单副本一旦丢数据，恢复后的 GCS 也就丢了集群状态。

### 形态 B：云数据库 Tair（生产环境）

[云数据库 Tair（兼容 Redis）](https://help.aliyun.com/zh/redis/)提供主备自动切换与托管备份。

**① 实例选型**

Ray 官方文档列出的支持形态是**单分片** Redis Cluster 或 Redis Sentinel（可带一个或多个副本），在 Tair 上对应**标准架构（主从）**，本文即用该架构验证。

GCS 元数据量很小 —— 下面 1 worker + 1 actor 只产生 7 个 key，数据都压在这几个 HASH 的 field 里，key 的数量不随集群规模增长。规格按自己的 actor 与 placement group 规模评估，并对内存使用率、连接数、响应延迟配置告警。

**② 网络与白名单**

实例必须与 ACK 集群**同 Region、同 VPC**，并在白名单里放行集群的网段，本文操作时放行了集群VPC网段。

**③ 建 Secret，起一个常驻的 redis-cli Pod**

集群里没有 Redis Pod，因此需要一个客户端载体：它既做部署前的连通性预检，也承担第 3、5 步查 Redis 的活。密码从刚建的 Secret 里取，不会出现在命令行和 shell 历史中：
该步骤非必须项，仅为了查看数据放便，也可以参考[Tair连接实例](https://help.aliyun.com/zh/redis/getting-started/step-3-connect-to-an-apsaradb-for-redis-instance)登陆实例进行查看；

```bash
export TAIR_HOST=r-xxxxxxxx.redis.<region>.rds.aliyuncs.com
kubectl create secret generic redis-password-secret --from-literal=password='<Tair 实例密码>'

kubectl run redis-client --image=redis:7.4-alpine --restart=Never \
  --env="REDISCLI_AUTH=$(kubectl get secret redis-password-secret -o go-template='{{.data.password | base64decode}}')" \
  --command -- sleep infinity
kubectl wait --for=condition=Ready pod/redis-client --timeout=60s
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} PING
```

预期结果：

```
PONG
```

> 建 Secret 必须用 `--from-literal`：从文件创建会把文件末尾的换行符一并存进去，head 启动时报 `WRONGPASS`。
>
> 预检不通就先解决白名单、安全组或密码，别急着部署 —— GCS 连不上 Redis 的表现是 head 持续 `CrashLoopBackOff`，事后排查的成本远高于预检。

最后把 [`raycluster-gcs-ft.yaml`](raycluster-gcs-ft.yaml) 里的 `redisAddress` 换成实例地址，这是形态 B 对 RayCluster 的唯一改动：

```yaml
    redisAddress: "r-xxxxxxxx.redis.<region>.rds.aliyuncs.com:6379"
```

## 2. 部署启用容错的 RayCluster

[`raycluster-gcs-ft.yaml`](raycluster-gcs-ft.yaml) 包含上面那段 `gcsFaultToleranceOptions` 配置，以及一个存放两个验证脚本的 ConfigMap。head 设置了 `num-cpus: "0"`，使 actor 必然被调度到 worker 上。

```bash
kubectl apply -f raycluster-gcs-ft.yaml
kubectl wait --for=condition=Ready pod -l ray.io/cluster=raycluster-gcs-ft --timeout=300s
kubectl get raycluster raycluster-gcs-ft
kubectl get pod -l ray.io/cluster=raycluster-gcs-ft
```

预期结果（两种形态一致：`STATUS` 为 `ready`，两个 Pod 均为 `1/1 Running` 且 `RESTARTS` 为 0）：

```
NAME                DESIRED WORKERS   AVAILABLE WORKERS   CPUS   MEMORY   GPUS   STATUS   AGE
raycluster-gcs-ft   1                 1                   2      4Gi      0      ready    66s

NAME                                         READY   STATUS    RESTARTS   AGE
raycluster-gcs-ft-head-v6wxq                 1/1     Running   0          66s
raycluster-gcs-ft-small-group-worker-z5qgc   1/1     Running   0          66s
```

**集群 ready 不代表容错已经生效。** 下面三项检查缺一不可；未启用容错时它们全部为空。

```bash
# 命名空间里可能有多个 Ray 集群，选择器必须带上 ray.io/cluster，否则可能取到别的集群的 Pod
export HEAD_POD=$(kubectl get pod -l ray.io/cluster=raycluster-gcs-ft,ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
export WORKER_POD=$(kubectl get pod -l ray.io/cluster=raycluster-gcs-ft,ray.io/group=small-group -o jsonpath='{.items[0].metadata.name}')
```

**① KubeRay 添加了 Redis 清理 finalizer**

```bash
kubectl get raycluster raycluster-gcs-ft -o jsonpath='{.metadata.finalizers}{"\n"}'
# ["ray.io/gcs-ft-redis-cleanup-finalizer"]
```

**② head 被注入了三个环境变量，启动参数带上了 `--redis-password`**

```bash
kubectl get pod ${HEAD_POD} \
  -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' \
  | grep -iE "redis|external_storage"
```

预期结果共四行：前三行是注入的环境变量（`REDIS_PASSWORD` 的值来自 `secretKeyRef`，故显示为空），第四行是 KubeRay 生成的启动命令，因为它的值里含 `redis` 也被 `grep` 匹配到了（下面省略了其中部分与容错无关的参数）。形态 B 只有 `RAY_REDIS_ADDRESS` 这一行不同，显示为实例的专有网络地址：

```
RAY_external_storage_namespace=cbf3aa87-0a34-45de-8b64-0f4f8bda7b4c    # = RayCluster UID
RAY_REDIS_ADDRESS=redis:6379                                           # 形态 B：r-xxxxxxxx.redis.<region>.rds.aliyuncs.com:6379
REDIS_PASSWORD=
KUBERAY_GEN_RAY_START_CMD=ray start --head --block ... --num-cpus=0 --redis-password=$REDIS_PASSWORD
```

`--redis-password` 由 Operator 自动追加，无需手写。确认它确实落到了容器的启动参数里：

```bash
kubectl get pod ${HEAD_POD} -o jsonpath='{.spec.containers[0].args}' | tr ' ' '\n' | grep redis
# --redis-password=$REDIS_PASSWORD
```

**③ worker 被注入了 600 秒重连超时，head 没有**

```bash
kubectl get pod ${WORKER_POD} \
  -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep reconnect
# RAY_gcs_rpc_server_reconnect_timeout_s=600

kubectl get pod ${HEAD_POD} -o jsonpath='{.spec.containers[0].env}' | grep -c reconnect
# 0    head 沿用 Ray 默认的 60 秒，符合预期
```

## 3. 创建 detached actor 并查看 Redis 数据

`detached_actor.py` 创建一个名为 `counter_actor` 的计数器 actor。普通 actor 的生命周期绑定在创建它的 Driver 上，Driver 一退出 actor 就被回收；而 `lifetime="detached"` 的 actor 独立于 Driver 存在，「名字 → actor」这条映射由 GCS 保管（开启容错后即写入 Redis 的 `@ACTOR` key），因此后续任何一个新 Driver 都能用 `ray.get_actor("counter_actor")` 把它取回来。

`increment_counter.py` 每次运行都是一个新 Driver，它**能否按名字取到 actor**，检验的是 head 重启后 GCS 元数据有没有从 Redis 恢复；取到之后**计数是否接着往上加**，检验的是 actor 进程本身有没有被牵连重启（它跑在 worker 上，只要 worker 不重启，内存里的计数就还在）。未开启容错时，新 head 的 GCS 里没有这条注册信息，`ray.get_actor` 会直接报错找不到 `counter_actor`。

```bash
kubectl exec ${HEAD_POD} -- python3 /home/ray/samples/detached_actor.py
kubectl exec ${HEAD_POD} -- python3 /home/ray/samples/increment_counter.py
```

预期结果：

```
1
```

再看 GCS 写进 Redis 的内容。`RAY_UID` 是 key 前缀，**必须在删除 RayCluster 之前导出**，集群删掉后就查不到了：

```bash
export RAY_UID=$(kubectl get raycluster raycluster-gcs-ft -o jsonpath='{.metadata.uid}')

# 形态 A：库里只有这个集群，直接列全库最直观
kubectl exec ${REDIS_POD} -- env REDISCLI_AUTH="5241590000000000" redis-cli KEYS "*"

# 形态 B：生产实例上不能用 KEYS（它会阻塞 Redis 直到遍历完成），改用游标扫描并按前缀过滤
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} --scan --pattern "RAY${RAY_UID}*"
```

预期结果（两种形态一致）—— 所有 key 都以 `RAY<UID>` 为前缀：

```
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@KV                 # 内部 KV，存放 Ray 各组件的杂项元数据
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@NODE               # 节点注册表：每个 raylet 的地址、资源和存活状态
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@JOB                # Job 元数据：每个 driver 提交的 job 及其状态
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@JobCounter         # Job ID 自增计数器
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@ACTOR              # actor 注册表，counter_actor 的「名字 → ID」就在这里
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@ACTOR_TASK_SPEC    # actor 创建任务的规格，重建 actor 时要用
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@WORKERS            # worker 进程信息，含异常退出记录
```

想看某张表的内容，`HGETALL` 在两种形态下写法相同，只是执行的 Pod 不同：

```bash
kubectl exec ${REDIS_POD} -- env REDISCLI_AUTH="5241590000000000" redis-cli HGETALL "RAY${RAY_UID}@NODE"   # 形态 A
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} HGETALL "RAY${RAY_UID}@NODE"                        # 形态 B
```

## 4. 模拟 head 故障

这一步两种形态完全一样：Redis 只负责存元数据，恢复时序由 head 侧的 60 秒超时决定。杀掉 head Pod 内的 GCS 进程，同时观察两个 Pod：

```bash
kubectl exec ${HEAD_POD} -- pkill gcs_server

for i in $(seq 1 10); do
  echo "=== T+$((i*10))s ==="
  kubectl get pod -l ray.io/cluster=raycluster-gcs-ft --no-headers \
    -o 'custom-columns=NAME:.metadata.name,READY:.status.containerStatuses[0].ready,RESTARTS:.status.containerStatuses[0].restartCount'
  sleep 10
done
```

> `custom-columns=...` 必须加引号：zsh 会把里面的 `[0]` 当成通配符，不加引号会报 `no matches found`。

预期结果：

| 时刻 | head | worker |
|---|---|---|
| T+0s ~ T+40s | ready=true，restarts=0 | ready=true，restarts=0 |
| **T+50s** | ready=**false** | ready=true，restarts=0 |
| **T+60s** | 容器重启，restarts=**1** | ready=true，restarts=**0** |
| **T+70s** | ready=**true** | ready=true，restarts=0 |

**worker 全程 `ready=true`、`RESTARTS=0`，这就是 GCS 容错的效果。** 60 秒这个数字正是 head 侧 `RAY_gcs_rpc_server_reconnect_timeout_s` 的默认值。未启用容错时 worker 拿不到 600 秒的覆盖值，会与 head 同时到点退出。

再次访问 detached actor：

```bash
kubectl exec ${HEAD_POD} -- python3 /home/ray/samples/increment_counter.py
```

预期结果 —— 计数继续累加而不是从头开始，因为 actor 始终活在 worker Pod 上：

```
2
```

## 5. 删除集群并验证 Redis key 自动清理

删除前后各查一次 key 数量，就能确认清理是否真的发生。两种形态的判据不同：形态 A 的库里只有这个集群，用 `DBSIZE`（当前库的 key 总数）即可；形态 B 的实例上往往还住着别的 Ray 集群甚至别的业务，`DBSIZE` 不会归零，必须按 UID 前缀计数。

```bash
# 删除前 —— 形态 A
kubectl exec ${REDIS_POD} -- env REDISCLI_AUTH="5241590000000000" redis-cli DBSIZE
# 删除前 —— 形态 B
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} --scan --pattern "RAY${RAY_UID}*" | wc -l

kubectl delete raycluster raycluster-gcs-ft
kubectl get job --field-selector metadata.name=raycluster-gcs-ft-redis-cleanup

# 删除后 —— 重复上面对应形态的计数命令
```

预期结果 —— KubeRay 会创建一个清理 Job 删掉本集群的 key，清完才摘掉 finalizer，Job 随后由 `ttlSecondsAfterFinished` 自动回收：

```
NAME                              STATUS     COMPLETIONS   DURATION   AGE
raycluster-gcs-ft-redis-cleanup   Complete   1/1           5s         6s
```

| 形态 | 计数方式 | 删除前 | 删除后 |
|---|---|---|---|
| A：自建 Redis | `DBSIZE` | `(integer) 7` | `(integer) 0` |
| B：Tair | `RAY<uid>*` 前缀计数 | `7` | `0` |

自 KubeRay v1.1.0 起该清理是 best-effort（v1.0.0 时 Job 失败会导致集群删不掉）：Job 失败也会摘掉 finalizer，但 Redis 里会残留 key，需要按 UID 前缀手工清理：

```bash
# 形态 A
kubectl exec ${REDIS_POD} -- sh -c \
  "redis-cli -a 5241590000000000 --scan --pattern 'RAY${RAY_UID}*' | xargs -r redis-cli -a 5241590000000000 DEL"
# 形态 B
kubectl exec redis-client -- sh -c \
  "redis-cli -h ${TAIR_HOST} --scan --pattern 'RAY${RAY_UID}*' | xargs -r redis-cli -h ${TAIR_HOST} DEL"
```

## 清理

```bash
kubectl delete -f raycluster-gcs-ft.yaml --ignore-not-found

# 形态 A
kubectl delete -f redis.yaml

# 形态 B：Tair 实例保留，只删客户端 Pod 和 Secret
kubectl delete pod redis-client --ignore-not-found
kubectl delete secret redis-password-secret --ignore-not-found
```

## 参考资料

- [GCS fault tolerance in KubeRay](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-gcs-ft.html)
- [Tuning Redis for a persistent fault tolerant GCS](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-gcs-persistent-ft.html)
- [Ray Serve end-to-end fault tolerance](https://docs.ray.io/en/latest/serve/production-guide/fault-tolerance.html)
- [Ray Core：GCS 容错](https://docs.ray.io/en/latest/ray-core/fault_tolerance/gcs.html)
- [ACK 上 Ray 集群最佳实践](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ray-cluster-best-practices/)
- [Redis 的 key 驱逐机制](https://redis.io/docs/latest/develop/reference/eviction/)
