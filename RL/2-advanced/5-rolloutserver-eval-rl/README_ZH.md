# RolloutServer同时支持Evaluation和ReinforceLearning

本文介绍如何让一个 RolloutServer 同时服务 Evaluation（评测）和 Reinforcement Learning（强化学习，下称 RL）两类 rollout。核心原则：模型端点和凭据按请求配置，供应商 Secret 只在 Server 部署时注入，沙箱与容量通过 queue 隔离。

## 概述

两类 rollout 的配置差异：

| 配置 | RL rollout | Evaluation rollout |
|---|---|---|
| 模型端点 | verl 的 rollout proxy，通常每轮动态传入 | 供应商 API |
| 模型凭据 | proxy 的鉴权信息 | Server 中 Secret 注入的 key |
| Sandbox | 可共用，也可通过 queue/backend 分开 | 同左 |
| 轨迹 | 必须经过 RL proxy，采集 token/logprob | 普通评测轨迹即可 |

工作机制：

- 请求中的 `agent.kwargs.extra_env` 在 worker 进程内转换为每条 trial 的 agent 环境变量。该机制对所有 agent 通用，不限于某个 agent。
- 值中的 `${变量名}` 由 server 在构造 agent 时从 worker 进程环境解析。请求体中只出现变量引用，真实 key 不出现在请求中，也不落盘。
- `queue` 只负责沙箱后端路由与配额，不参与模型凭据选择。模型选择完全由请求字段完成。

## Backend 与 Queue 机制

Backend 与 Queue 是 RolloutServer 的两个核心抽象：Backend 封装沙箱执行能力的差异，Queue 封装上层业务的容量隔离。配置双模式前，先理解这两个概念：

- **Backend**：对不同后端和沙箱类型的配置抽象。每个 backend 在 `config.backends[]` 中声明一种环境类型（`import_path`，如 ACK Pod 或 E2B），以及默认环境参数（`defaults`，如 namespace、`sandbox_set_name`）和基础设施凭据（`env_profile`）。沙箱类型的差异收敛在 backend 层，上层请求无需感知。
- **Queue**：对上层业务的配额抽象。每个 queue 对应一条业务通道（如 `rl`、`evaluation`），通过 `max_concurrent` / `max_queued` / `rate_limit` 约束该业务的并发数、排队深度和提交速率。业务之间通过 queue 隔离容量，无需为每类业务单独部署 server。

两者通过 `queues` 关联：backend 声明自己服务的 queue，请求的 `queue` 字段（默认 `default`）决定 trial 在哪个 backend 上执行。路由规则：

- 一个 queue 只能绑定到一个 backend。
- 未绑定任何 backend 的 queue，落到标记 `default: true` 的 backend。
- 没有任何可用 backend 时，server 拒绝启动。
- queue 只决定沙箱路由与容量，不参与模型端点和凭据选择。

按隔离程度，双模式有两种配置方式。

### 方式 A：共用 backend，独立 queue

评测与 RL 共用同一沙箱后端，通过 queue 配额隔离容量。适用于两类流量运行在同一集群的场景：

```yaml
config:
  backends:
    - name: ack-pod
      import_path: harbor.environments.ack:ACKEnvironment
      default: true
      queues: ["rl", "evaluation"]
      defaults:
        namespace: kube-rl
  quotas:
    rl:         { max_concurrent: 30000, max_queued: 80000 }
    evaluation: { max_concurrent: 10000, max_queued: 20000 }
```

两个队列各自独立计数，防止评测流量挤占 RL 容量，反之亦然。

### 方式 B：独立 backend

评测与 RL 使用不同的沙箱后端（不同集群、不同沙箱池或不同凭据集），实现完全隔离：

```yaml
config:
  backends:
    - name: ack-rl
      import_path: harbor.environments.ack:ACKEnvironment
      default: true
      queues: ["rl"]
      defaults:
        namespace: kube-rl
    - name: e2b-eval
      import_path: harbor.environments.e2b:E2BEnvironment
      queues: ["evaluation"]
      env_profile: eval-infra
      defaults:
        sandbox_set_name: eval-pool
  env_profiles:
    - name: eval-infra
      env:
        E2B_API_KEY: ${E2B_EVAL_API_KEY}
        E2B_API_URL: http://sandbox-manager.sandbox-system.svc.cluster.local:8080
  quotas:
    rl:         { max_concurrent: 30000, max_queued: 80000 }
    evaluation: { max_concurrent: 10000, max_queued: 20000 }
```

`env_profile` 为 backend 提供基础设施凭据（如 E2B key），值通过 `${VAR}` 引用 pod 环境变量，Secret 本身不进入配置文件。模型 key 不能放在这里（见"迁移现有部署"）。

两种方式下，请求都通过 `queue` 字段选择用途：`"queue": "rl"` 或 `"queue": "evaluation"`。模型端点和凭据仍由请求中的 `extra_env` 按请求配置，与 queue 无关。

## 注入供应商 Secret

使用独立变量名注入供应商 key，避免将其设为所有请求的默认值：

1. 创建包含供应商 key 的 Secret（示例名 `hbo-eval-model`，key 为 `api-key`）。
2. 在 Helm values 中通过 `extraEnv` 引用该 Secret：

```yaml
extraEnv:
  - name: EVAL_PROVIDER_API_KEY
    valueFrom:
      secretKeyRef:
        name: hbo-eval-model
        key: api-key
```

说明：

- chart 的 `extraEnv` 按原生 EnvVar 渲染，支持 `valueFrom.secretKeyRef`，注入到单体 server、multi 模式的 ingress/worker、viewer 所有角色。batchController 使用独立键 `batchController.extraEnv`。chart 没有 `extraEnvFrom` 这个 value。
- 使用独立变量名的原因：agent 的凭据解析按"请求 `extra_env` 优先、server 进程 `os.environ` 兜底"逐键回退。如果将 `OPENAI_API_KEY` / `OPENAI_BASE_URL` 直接放入 Server 全局环境，未显式配置的请求（包括 RL 请求）会继承这套供应商默认值，评测配置会泄漏进 RL 轨迹。使用 `EVAL_PROVIDER_API_KEY` 这类独立名后，只有显式引用它的评测请求会用到该 key。
- 该机制不会把整个 server 环境灌进沙箱：沙箱 exec 只收到显式合并后的 env。继承是"agent 按候选变量名逐个回退读取"的效果。

## 提交 Evaluation rollout

在请求中通过 `agent.kwargs.extra_env` 指定供应商端点，并用 `${变量名}` 引用部署时注入的 key：

```json
{
  "queue": "evaluation",
  "agent": {
    "name": "mini-swe-agent",
    "model_name": "openai/qwen4-max",
    "kwargs": {
      "extra_env": {
        "OPENAI_BASE_URL": "https://dashscope.aliyuncs.com/compatible-mode/v1",
        "OPENAI_API_KEY": "${EVAL_PROVIDER_API_KEY}"
      }
    }
  }
}
```

请求里只传变量引用，不传真实 key。

`extra_env` 与 `${VAR}` 的精确语义：

- 支持 dict 和 K8s list（`[{"name": K, "value": V}]`）两种格式。server 将其组装为 agent 环境，并在构造 agent 时解析变量引用。
- `${VAR}` 从 worker 进程的 `os.environ` 解析。变量未定义时抛出 `ValueError`，该 trial 失败，不会静默使用空值。
- 支持 `${VAR:-default}` 默认值语法。
- 只有整个值是一个模板时才解析（fullmatch）：`"${EVAL_PROVIDER_API_KEY}"` 会解析；`"Bearer ${TOKEN}"` 这类嵌在长字符串中间的引用按字面透传。
- 解析后的 env 经 per-task 隔离的 exec overlay 注入 agent 的 setup/run 命令，不污染 verifier/build 命令。
- `model_name` 使用 `provider/model` 格式。`openai/<model-id>` 表示 OpenAI 兼容端点，配合 `OPENAI_BASE_URL` / `OPENAI_API_KEY` 使用。

## 提交 RL rollout

保留 recipe 当前的 proxy 接入方式：动态传入 `llm_proxy_url`，并在 `extra_env` 中显式设置端点与鉴权。proxy 无鉴权时的示例：

```json
{
  "queue": "rl",
  "llm_proxy_url": "http://<verl-proxy>:<port>/v1",
  "agent": {
    "name": "mini-swe-agent",
    "model_name": "openai/<policy-model>",
    "kwargs": {
      "extra_env": {
        "OPENAI_BASE_URL": "http://<verl-proxy>:<port>/v1",
        "OPENAI_API_KEY": "EMPTY"
      }
    }
  }
}
```

RL 的其他 session、采样参数继续由 recipe 填写。

**显式设置 `OPENAI_BASE_URL`，不要依赖 `llm_proxy_url` 自动改写端点。** `llm_proxy_url` 不会自动改写 agent 的模型端点；仅设置该字段时，内置 agent（mini-swe-agent、claude-code 等）会以 `Unknown option 'model_base_url'` 报错。可靠做法二选一：

1. `extra_env.OPENAI_BASE_URL` 指向 proxy（上例写法，推荐）。
2. `agent.kwargs.base_url` 显式传入。

`llm_proxy_url` 字段仍按 recipe 约定传入（SDK/slime hook 会透传），用于标识本条 rollout 的 proxy 归属。

token/logprob 采集：`rollout_details`（token_ids/logprobs）由 server 的 LLM 调用层产出。目前只有部分 agent（如 `terminus-2`，需开启 `collect_rollout_details`）会填充该字段；mini-swe-agent、claude-code 等 CLI 类 agent 不产出 `rollout_details`。RL 需要 token 级数据时，选用支持采集的 agent，或由 verl proxy 侧自行记录。"轨迹必须经过 RL proxy"这一约束不变。

## 迁移现有部署

如果当前部署把供应商 `OPENAI_API_KEY` 和 `OPENAI_BASE_URL` 放在 Server 全局环境中（适合纯评测场景），按以下步骤迁移到双模式：

1. 从 Server 全局环境移除 `OPENAI_API_KEY` / `OPENAI_BASE_URL`，改为 `EVAL_PROVIDER_API_KEY`（Secret `secretKeyRef` 注入，见"注入供应商 Secret"）。
2. 评测请求显式携带 `extra_env`：端点 + `${EVAL_PROVIDER_API_KEY}` 引用。
3. RL 请求显式携带 `extra_env` 指向 verl proxy，不依赖 `llm_proxy_url` 改写端点。
4. 检查 `config.env_profiles` 中没有模型凭据。
5. 按需为 `rl` / `evaluation` 配置独立配额。

关于 `env_profiles` 的限制：该配置用于 Sandbox 后端的基础设施凭据（如 `E2B_API_KEY`、`KUBECONFIG`），不能放模型 key。server 在启动期校验并拒绝模型相关变量：前缀 `ANTHROPIC_` / `OPENAI_` / `GEMINI_` / `LLM_`，以及精确匹配 `GOOGLE_API_KEY`（大小写不敏感；`GOOGLE_APPLICATION_CREDENTIALS` 放行供 GKE 使用）。报错信息会指引改用 `agent.kwargs.extra_env`。

`env_profiles` 的 `${VAR}` 与请求级 `extra_env` 是两套解析语义：前者在进程启动时一次性解析，变量缺失即启动失败，不支持 `:-` 默认值；后者在每条 trial 构造 agent 时解析，支持默认值。

## 验证双模式运行

上线双模式前，按以下最小矩阵执行联合验证：

1. 提交一条 evaluation rollout（`queue=evaluation`，供应商端点）。确认状态 completed、reward 正常、轨迹中不含 proxy 地址。
2. 提交一条 RL rollout（`queue=rl`，proxy 端点）。确认状态 completed，且 verl proxy 侧可见对应流量。
3. 两类请求并发混跑（各不少于 50 条）。确认互不串端点：评测轨迹不含 proxy 地址，RL 轨迹不含供应商地址；两个队列的配额各自独立计数。
4. 提交一条不带 `extra_env` 的请求。确认其不再继承供应商默认值：改造后应因缺凭据而失败或走 agent 自身默认，而不是打到供应商。
5. 引用一个未注入的变量（如 `${NOT_INJECTED}`）。确认 trial 以 `ValueError` 失败（fail-fast）。