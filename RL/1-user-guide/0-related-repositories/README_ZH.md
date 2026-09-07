# 相关代码库介绍

本指南的 RL 示例构建在三个开源仓库之上。**它们都是阿里巴巴的「贡献 fork」** —— 按其自述：
*"Fork for contributing. All changes intended for upstream PRs."*（为贡献而 fork，所有改动都将以 PR 提交上游）。
三者的 fork **`main` 都与上游保持同步**；阿里巴巴的工作放在 **`feat/*` 与 `release-*` 分支**上，并通过 PR 上游化。
因此下文「增加了什么」是这些分支的**快照**（每节附核对用的 compare 命令）。

| 仓库 | 在 RL 栈中的角色 | 上游（fork 父仓库） |
|------|------------------|---------------------|
| [alibaba/slime](https://github.com/alibaba/slime) | RL 后训练框架（Megatron 训练 + SGLang rollout） | [THUDM/slime](https://github.com/THUDM/slime) |
| [alibaba/harbor](https://github.com/alibaba/harbor) | 智能体评测 / RL 环境运行器（生成 rollout） | [harbor-framework/harbor](https://github.com/harbor-framework/harbor) |
| [alibaba/verl-recipe](https://github.com/alibaba/verl-recipe) | 端到端 RL 训练配方（基于 verl，另一条路径） | [verl-project/verl-recipe](https://github.com/verl-project/verl-recipe) |

[2-run-an-rl-task](../2-run-an-rl-task/README_ZH.md) 中的示例走 **slime + harbor** 路径；
`verl-recipe` 是基于 verl 的替代路径，此处一并介绍以求完整。

---

## alibaba/slime

**是什么。** slime 是面向 RL Scaling 的 LLM 后训练框架：高性能训练（Megatron）与灵活的 rollout / 数据生成
（SGLang）直接打通，训练、rollout、reward/verifier、环境交互都走同一条 train / rollout / Data Buffer 路径。
它直接透传 Megatron 参数、并以 `--sglang-` 前缀暴露 SGLang 参数。

**阿里巴巴增加了什么（fork 分支，正在上游化）。** 主题是**在 ACK 沙箱上、经 harbor 跑智能体 remote-agent RL**：

- **端到端的 ACK 沙箱 remote-agent RL** —— 由训练循环驱动，让 slime RL 智能体在 harbor 管理的沙箱
  （经 E2B / SandboxSet）内运行。
- **Harbor 集成** —— 引入 harbor environment；把 `SandboxSet` 名透传进去；让远程 `HarborClient` 的提交格式
  对齐 Rollout Server 的 `AgentRunRequest`。
- **进程内 `OpenAIAdapter`** —— 替换早先的进程外 TokenProxy；从共卡的 SGLang 引擎提供生成、并采集每 token 的
  `(token_id, logprob)` 供训练，且**按 trial 隔离并发的 OpenAI 会话**。
- **standalone proxy 模式** + 引擎自注册（把 rollout 入口与单个引擎解耦）。
- **`train_remote_agent` 作为 `train.train` 的薄封装**（remote-agent 入口）。
- **鲁棒性修复** —— 尊重 rollout 超时并对 SGLang 5xx 重试；清洗 local trial 名；并整理出统一的 remote-agent
  runbook / 启动器。

> 核对：`git log THUDM/slime/main..alibaba/slime/release-slime-dev-<sha>`（fork 的 `main` 与上游一致，工作在
> `feat/*` 与 `release-slime-dev-*` 分支）。

## alibaba/harbor

**是什么。** Harbor（Terminal-Bench 作者团队开发）是用于评测/优化智能体、以及构建和使用 RL **环境**的框架。
它能在多种沙箱 provider 上并行运行任意智能体，并能**为 RL 优化生成 rollout** —— 这正是 slime 在此处的用法。

**阿里巴巴增加了什么（fork 分支，正在上游化）。** 让 harbor 在 ACK 上成为一等公民，并对接 rollout 管控面：

- **`ACKEnvironment`** —— 原生 ACK/Kubernetes 环境 provider（`feat/add-ack-environment`）：每个 trial 一个
  沙箱 Pod，与上游的 E2B / Modal / Daytona 等并列。
- **ACK 运行时 exec** —— ACK 上的沙箱内持久化 exec（`feat/ack-runtime-exec`）。
- **提交到 Rollout Server** —— 把 harbor run job 提交给 RolloutServer
  （见 [../../2-advanced/3-rolloutserver-trajectory-management](../../2-advanced/3-rolloutserver-trajectory-management/README_ZH.md)）。
- **镜像缓存 + 镜像版本检查** —— 预拉/校验任务镜像以缩短沙箱冷启动（`feat/image-cache`、
  `feat/add-image-version-check`；见 [../../2-advanced/1-image-cache](../../2-advanced/1-image-cache/README_ZH.md)）。
- **多租户凭据** —— E2B 凭据作为构造参数传入；按凭据区分 Kubernetes client manager；按 host 解析镜像构建的仓库鉴权。
- **Trial 恢复（RFC）** —— 重启后 attach 并续跑在途 trial，而非从头重跑。

> 核对：<https://github.com/harbor-framework/harbor/compare/main...alibaba:release-dev-e5e46809-rollout-server>
> （分支名带 base commit 后缀、会变动）。

## alibaba/verl-recipe

**是什么。** `verl-recipe` 收录基于 [verl](https://github.com/verl-project/verl) 的端到端 RL **配方**；它作为
verl 的 `recipe/` 子模块使用，每个 recipe 用 `REQUIRED_VERL.txt` + `install_verl.sh` 钉住所需 verl 版本以可复现。

**阿里巴巴增加了什么（fork 分支，正在上游化）。** 在 verl trainer 上、对标 slime 路径的**remote-agent（智能体）RL 配方**：

- **`remote_agent` RL 配方** + `external_sglang` 整合。
- **`remote_megatron_sglang` V1 trainer 移植** —— 适配上游 V1 `TaskRunner` API，在内部与上游 verl main 上均做过端到端验证。
- **E2B 沙箱 + harbor 数据集** 集成 —— Dockerfile 依赖、Hydra 驱动配置、本地 harbor 数据集 + 自动 `task_path`。
- **standalone proxy 模式** 与 **Kubernetes 部署示例**。

> 核对：<https://github.com/verl-project/verl-recipe/compare/main...alibaba:release-remote-agent-rl>
> 及 `.../compare/main...alibaba:feat/add-agentic-training-example`。

---

## 三者如何协同（slime 路径）

```
prompts.jsonl ──► slime (train_remote_agent.py, GRPO)
                     │  rollout
                     ▼
                  harbor Trial ──► ACKEnvironment ──► 沙箱 Pod（任务镜像）
                     │                                      ▲
                     └── 进程内 OpenAIAdapter ──────────────┘  (SGLang 生成，
                                                                 逐 token 采集供训练)
```

`verl-recipe` 是在 **verl** trainer 上的同一思路（其 `remote_agent` 配方），是上面 slime 路径的替代。
