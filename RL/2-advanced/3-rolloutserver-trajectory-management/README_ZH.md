# 用 RolloutServer 管理轨迹

一步一步在 ACK 上部署 **rollout server**（`kube-rl` Helm 发布），用它运行 trial、持久化轨迹、并提供 RL 训练数据。

**为什么。** 在基础的 [执行 RL 任务](../../1-user-guide/2-run-an-rl-task/README_ZH.md) 流程里，训练框架直接驱动沙箱
trial，轨迹只是本地文件。规模化后你需要一个管控面来接管 trial 执行与轨迹生命周期：容错（心跳判活、退避重试、
分阶段超时、attach 续跑恢复）、配额、成本归因，以及一个可查询的全量轨迹存储（`trajectory.json`、逐轮日志、
`rollout_details` = token_ids/logprobs）外加 Viewer（Job/Task/Trial、时间线、对比网格）。

**前置条件**

- ACK 集群，`kubectl` + Helm 3。
- server 镜像在集群可拉的仓库里（ACR + pull secret 见
  [1-prepare-dataset](../../1-user-guide/1-prepare-dataset/README_ZH.md) 步骤 1–4）。
- 已配置阿里云 CLI（`aliyun`，用于下文 NAS）。

## 步骤 1 —— 创建持久化存储（NAS）【云上操作】

```bash
aliyun nas CreateFileSystem \
  --RegionId cn-hangzhou --ZoneId cn-hangzhou-i \
  --FileSystemType standard --StorageType Performance \
  --ProtocolType NFS --VpcId <vpc-id> --VSwitchId <vswitch-id>
aliyun nas CreateMountTarget --FileSystemId <fs-id> --NetworkType Vpc \
  --VSwitchId <vswitch-id> --AccessGroupName DEFAULT_VPC_GROUP_NAME
aliyun nas DescribeMountTargets --FileSystemId <fs-id>    # 等 Active，记下域名
```

## 步骤 2 —— 安装 rollout server（Helm）

chart 随 RolloutServer 项目发布（`charts/kube-rl`）。最小安装：

```bash
helm upgrade --install kube-rl charts/kube-rl \
  -n kube-rl --create-namespace \
  --set image.repository=<acr-registry>/harbor-server \
  --set image.tag=<tag> \
  --set imagePullSecrets[0].name=acr-pull-secret \
  --set config.namespace=kube-rl
kubectl -n kube-rl get pod,svc    # 期望：pod kube-rl-... 1/1 Running
```

你会得到：Deployment `kube-rl` + Service `kube-rl`（HTTP 入口）、一个 Viewer，默认 `single` 拓扑
（每个 Pod 跑 ingress + worker + Redis sidecar；状态持久化在 `persistence.nas` 卷上）。生产环境请用
values 文件（项目自带 demo / E2B / 配额 / 大并发四份示例 values），并通过 `persistence.nas` 挂载 NAS。

## 步骤 3 —— 验证

```bash
kubectl -n kube-rl port-forward svc/kube-rl 8080:8080 &
curl -s http://localhost:8080/health | python3 -m json.tool   # 期望 status: healthy
```

## 步骤 4 —— 提交第一个任务

再端口转发 Viewer，然后用自带脚本（打包任务目录、提交、轮询到终态、打印 reward）：

```bash
kubectl -n kube-rl port-forward svc/kube-rl-viewer 8081:8081 &

SERVER=http://localhost:8080 VIEWER=http://localhost:8081 \
TASK_DIR=./data/swe-bench-verified/astropy__astropy-14309 \
AGENT=oracle bash submit-task.sh
```

你会看到 trial 走过各阶段——`env_starting → agent_running → verifying → completed`——然后是含 reward 的
`result` JSON（`oracle` 跑参考答案；reward ≈ 1.0 说明链路通了）。轨迹（逐轮 `command.txt/stdout/stderr`、
`trajectory.json`、`rollout_details`）已持久化，可在 Viewer（`http://localhost:8081/`）浏览。

等价的裸 HTTP（脚本内部即此）：

```bash
TASK=astropy__astropy-14309
tar czf /tmp/$TASK.tar.gz -C ./data/swe-bench-verified "$TASK"
META='{"job_id":"demo","task_id":"'$TASK'-oracle","task_path":"'$TASK'","agent":{"name":"oracle"}}'
RUN_ID=$(curl -s -X POST "$SERVER/api/v1/runs/async" \
  -F "metadata=$META" -F "task_archive=@/tmp/$TASK.tar.gz" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["run_id"])')
curl -s "$SERVER/api/v1/runs/async/$RUN_ID/status" | python3 -m json.tool   # 轮询
curl -s "$SERVER/api/v1/runs/async/$RUN_ID/result" | python3 -m json.tool   # reward
```

**变体**

- `AGENT=<name>` 换智能体（如 `terminus-2`、`claude-code`；`nop` 空操作——脚本会自动关掉 verifier）。
- 沙箱池后端：`POOL=<池名>` 会附加 `environment_kwargs.sandbox_set_name` + `override_claim_image`
  （一个通用池即可服务任意任务镜像）。
- 异步 API、配额、数据集批量提交、接入 RL 循环的 Python SDK：见项目 `docs/tutorials`
  （先看「提交一个任务」，再看「试验数据查询」与 Python SDK）。

## 步骤 5 —— 从 slime RL 训练中使用

让 slime 的 remote-agent rollout 指向该 server（无需 E2B key）——见
[执行 RL 任务](../../1-user-guide/2-run-an-rl-task/README_ZH.md) 步骤 7：

```bash
MODE=kuberl KUBE_RL=http://kube-rl.kube-rl.svc.cluster.local:8080 \
DEPLOY=colocate ... bash examples/remote_agent/run_swebench.sh
```

此后每个 rollout 都在 server 管控下执行（重试、配额、轨迹持久化），TensorBoard + Viewer 一起给你训练曲线
与逐 trial 归因。

## 排障

| 症状 | 处理 |
|---|---|
| E2B 提交 404 | 缺池——设 `POOL=`（sandbox_set_name）；E2B 必须用预热池 |
| 缺 `prometheus_client` → `/metrics` 空 | 可选件；镜像内安装或忽略 |
| 改 values 后 Pod CrashLoop | ConfigMap 变更会自动滚动；只改 Secret 时需 `kubectl -n kube-rl rollout restart deploy/kube-rl` |

## 说明

- 最好与**可恢复的 Environment** 后端 + 持久 results 卷搭配：重启后 re-attach 在途 trial，而不是泄漏沙箱 Pod。
- **OSS 直写** results 后端可让智能体把输出直接写共享卷，跳过 rollout 结束时经 API server 的产物回传。
