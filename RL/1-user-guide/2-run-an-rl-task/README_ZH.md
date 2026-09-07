# 执行 RL 任务

本文在 ACK 上**一步一步**跑一个智能体 RL 训练任务，使用
[slime](https://github.com/alibaba/slime)（Megatron 训练 + SGLang rollout）。基于 verl 的替代路径见文末
[方式 B](#方式-b--verl-verl--vllm)。

你将搭建的形态：

```
prompts.jsonl ──► slime GRPO（train_remote_agent.py，运行在 Ray head pod 内）
                     │  rollout
                     ▼
                  harbor Trial ──► 沙箱 Pod（ACR 中的任务镜像）
                     │                    ▲
                     └── 进程内 OpenAI 适配器 ── SGLang 引擎（共用同一批 GPU）
                          采集逐 token 数据供训练
```

**前置条件**

- 带 GPU 节点池的 ACK 集群，`kubectl` 已配置。
- 已完成 [1-prepare-dataset](../1-prepare-dataset/README_ZH.md)（ACR 命名空间 + pull secret + 任务镜像 + prompts）。
- 本地机器装有 Docker（用于构建 workspace 镜像）。
- 已配置阿里云 CLI（`aliyun`，用于下文的 NAS 创建）。

## 步骤 1 —— 创建共享存储（NAS）【云上操作】

head pod 需要持久存储放模型 checkpoint 与任务数据集。

```bash
# 1.1 在集群所在 VPC 创建一个 NFS NAS 文件系统（替换 region/zone/vpc/vswitch）
aliyun nas CreateFileSystem \
  --RegionId cn-hangzhou --ZoneId cn-hangzhou-i \
  --FileSystemType standard --StorageType Performance \
  --ProtocolType NFS --VpcId <vpc-id> --VSwitchId <vswitch-id>

# 1.2 在同一 VSwitch 下创建挂载点
aliyun nas CreateMountTarget \
  --FileSystemId <fs-id> --NetworkType Vpc --VSwitchId <vswitch-id> \
  --AccessGroupName DEFAULT_VPC_GROUP_NAME

# 1.3 等挂载点 Active 后读取其域名
aliyun nas DescribeMountTargets --FileSystemId <fs-id>

# 1.4 在集群里创建 PV/PVC（本目录自带该文件）
sed "s|<NAS_MOUNT_DOMAIN>|<挂载点域名>|" nas-pv-pvc.yaml | kubectl apply -f -
kubectl get pvc rl-data        # 期望：Bound
```

## 步骤 2 —— 安装 KubeRay operator

```bash
helm repo add kuberay https://ray-project.github.io/kuberay/
helm repo update
helm install kuberay-operator kuberay/kuberay-operator \
  -n kuberay-system --create-namespace
kubectl -n kuberay-system get deploy    # 期望：kuberay-operator ... Available
```

## 步骤 3 —— 构建并推送 slime workspace 镜像

workspace 镜像打包了 Megatron-LM + slime + harbor（即训练环境）。

```bash
git clone https://github.com/alibaba/slime.git && cd slime
git checkout release-slime-dev-<sha>        # 承载 remote-agent RL 的分支（见仓库分支列表）

docker build -f docker/Dockerfile.workspace -t <acr-registry>/slime-workspace:<tag> .
docker push <acr-registry>/slime-workspace:<tag>
```

（`<acr-registry>` 与 [1-prepare-dataset](../1-prepare-dataset/README_ZH.md) 步骤 1 登录的是同一个仓库。）

## 步骤 4 —— 创建 RayCluster

```bash
sed -e "s|<IMAGE>|<acr-registry>/slime-workspace:<tag>|" \
    -e "s|<PULL_SECRET>|<acr-pull-secret>|" raycluster.yaml | kubectl apply -f -

kubectl wait --for=condition=Ready pod -n default \
  -l ray.io/node-type=head --timeout=600s
POD=$(kubectl get pod -n default -l ray.io/node-type=head \
  -o jsonpath='{.items[0].metadata.name}')
echo "head pod: $POD"
```

## 步骤 5 —— 准备模型与数据集（在 head pod 内）

```bash
# 5.1 下载 HF checkpoint 到本地盘（0.5B 冒烟足够便宜；放本地盘，别放 NAS）
kubectl exec -n default $POD -- bash -c '
  pip install -q "huggingface_hub[cli]" 2>/dev/null
  hf download Qwen/Qwen2.5-0.5B-Instruct --local-dir /root/Qwen2.5-0.5B-Instruct'

# 5.2 转成 Megatron torch_dist checkpoint（--ref-load 用），存到 NAS
kubectl exec -n default $POD -- bash -c '
  cd /root/slime && source scripts/models/qwen2.5-0.5B.sh &&
  PYTHONPATH=/root/Megatron-LM python tools/convert_hf_to_torch_dist.py "${MODEL_ARGS[@]}" \
    --hf-checkpoint /root/Qwen2.5-0.5B-Instruct \
    --save /var/model/Qwen2.5-0.5B_torch_dist'

# 5.3 下载任务数据集（SWE-bench Verified）到 NAS
kubectl exec -n default $POD -- bash -c \
  'harbor datasets download swe-bench-verified --output-dir /var/model-dataset'
kubectl exec -n default $POD -- ls /var/model-dataset/swe-bench-verified | head
```

## 步骤 6 —— 编写 prompt 清单

把 [prompts.jsonl](prompts.jsonl)（3 个示例任务）拷进 head pod：

```bash
kubectl cp prompts.jsonl default/$POD:/root/slime/examples/remote_agent/prompts.jsonl
```

每行映射一个任务：`metadata.instance_id` 必须与任务目录名一致（它填充任务路径模板
`/var/model-dataset/swe-bench-verified/{instance_id}`）。

## 步骤 7 —— 启动 RL 训练

所有运行都走统一启动器 `examples/remote_agent/run_swebench.sh`，三个正交开关：`MODE`（沙箱提交方式）、
`DEPLOY`（GPU 布局）、模型/preset 变量。分离式启动，避免 exec 断连带崩任务：

```bash
kubectl exec -n default $POD -- bash -c \
  'cd /root/slime && setsid bash -c "MODE=kuberl DEPLOY=colocate \
     HF_CKPT=/root/Qwen2.5-0.5B-Instruct \
     REF_LOAD=/var/model/Qwen2.5-0.5B_torch_dist \
     MODEL_NAME=openai/Qwen2.5-0.5B-Instruct \
     GPUS=2 TP=2 GLOBAL_BATCH_SIZE=2 NUM_ROLLOUT=1 \
     bash examples/remote_agent/run_swebench.sh" > /root/run.log 2>&1 & echo launched'
```

- `MODE=kuberl` 把每个 trial 提交给 **rollout server**（`KUBE_RL`，默认
  `http://kube-rl.kube-rl.svc.cluster.local:8080`）——先按
  [用 RolloutServer 管理轨迹](../../2-advanced/3-rolloutserver-trajectory-management/README_ZH.md)
  安装，或设 `KUBE_RL=http://<host>:<port>` 指向已有实例。
- `MODE=local` 在**进程内**跑 trial（便于调试；需要集群内 sandbox 栈的环境变量：`E2B_API_KEY`，
  可选 `E2B_API_URL`/`E2B_SANDBOX_URL`）。
- `DEPLOY=colocate` 训推共享 GPU（`DEPLOY=disagg` 需加 `ROLLOUT_GPUS=<n>` 指定独立推理 GPU）。
- 换大模型用 `MODEL_PRESET=<preset>` + `TP`/`PP`（27B 还需安装钉版本的 `mbridge`——见 slime runbook）。

## 步骤 8 —— 监控

```bash
kubectl exec -n default $POD -- bash -c '
  L=/root/run.log
  echo "运行中: $(pgrep -cf train_remote_agent.py)"
  echo "OOM: $(grep -c "CUDA out of memory" $L)"
  echo "已完成 rollout 步数: $(grep -c "perf/rollout_time" $L)"
  grep -o "rollout/rewards.: [0-9.]*" $L | tail -3'
```

可靠的「又完成一步」信号是 `perf/rollout_time` 的行数（每个 rollout = 一次优化器步）。
**不要**用 `train/step`——它每个 rollout 都记 `0`。

## 步骤 9 —— 查看结果

```bash
# TensorBoard（rollout 性能在 perf/*，质量在 rollout/*）
kubectl exec -n default $POD -- bash -c \
  'nohup tensorboard --logdir /root/slime/tensorboard_log --port 6006 >/dev/null 2>&1 &'
kubectl port-forward -n default $POD 6006:6006
# 打开 http://localhost:6006

# 逐轨迹 trace（trajectory.json、逐步日志）
kubectl exec -n default $POD -- ls /root/slime/trials | tail
```

## 排障

| 症状 | 处理 |
|---|---|
| 沙箱拉镜像报 `insufficient_scope: authorization failed` | pull secret 用错——见 [1-prepare-dataset](../1-prepare-dataset/README_ZH.md) 步骤 4 |
| `Agent name ... is not valid` | 用确切注册名（如带连字符的 `terminus-2`），经 `HARBOR_AGENT_NAME` 传入 |
| 所有 reward 相同（如全 0）、`pg_loss=0` | GRPO 需要组内 reward **方差**——保持 `N_SAMPLES>=2`、难易任务混合 |
| 长上下文 CUDA OOM | 调小 `--max-tokens-per-gpu` 或调大 `TP`（logits 峰值 `tokens × vocab × 2 / TP` 通常是主因） |

---

## 方式 B —— verl（verl + vLLM）

同一思路在 verl trainer 上的实现，经 [alibaba/verl-recipe](https://github.com/alibaba/verl-recipe) 的
**agentic** 配方：一个 OpenAI 兼容的 proxy server 把远程智能体的请求桥接到 verl 的 vLLM rollout，
并把轨迹回收给 GRPO/PPO。在任何已检出这两个仓库的 GPU 主机（或 Ray head）上运行：

```bash
# B.1 安装 verl + recipe（recipe 是 verl 的 recipe/ 子模块）
git clone https://github.com/verl-project/verl.git && cd verl
git submodule update --init --recursive recipe   # ……或直接 clone alibaba/verl-recipe
uv venv --python 3.12 && source .venv/bin/activate
uv pip install -e .
uv pip install -e ./harbor                      # 或 alibaba/harbor fork

# B.2 拉取数据集（与方式 A 相同的 harbor 任务）
harbor datasets download swe-bench-verified --output-dir ./data

# B.3 启动（GRPO；配置在 recipe/agentic/config/agentic_trainer.yaml，
#     agent loop 在 recipe/agentic/remote-agent.yaml）
MODEL_PATH=/var/model/Qwen2.5-7B-Instruct \
REMOTE_AGENT_USE_LOCAL_TRIAL=true \
REMOTE_AGENT_TASK_PATH_TEMPLATE=$PWD/data/swe-bench-verified/'{instance_id}' \
  bash recipe/agentic/train_agentic_hybrid_vllm_sweagent.sh
```

关键旋钮（优先级 yaml < shell env < CLI）：`LLM_PROXY_IP`（proxy 需能被智能体访问）、
`REMOTE_AGENT_NAME`、`REMOTE_AGENT_USE_LOCAL_TRIAL`、`PROXY_SERVER_URL`（设置后走独立 proxy 模式）。
完整参考：recipe 的 `agentic/README.md`。

> `agentic` 是生产全家桶（standalone proxy、disagg、ACK/harbor 集成）；并列的 `remote_agent`
> 配方是框架无关、面向上游的解耦版本。
