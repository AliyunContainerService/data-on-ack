# 为 Agentic RL 准备数据集

一步一步准备好智能体 RL 运行所需的两个输入：

1. **任务沙箱镜像**（ACR）—— 每个任务一个镜像（智能体在其中工作的容器）。
2. **Prompt 清单**（`prompts.jsonl`）—— 每个任务一行，把 RL 样本映射到任务实例。

**前置条件**

- ACK 集群，`kubectl` 已配置。
- 本地机器装有 Docker（构建/推送任务镜像）。
- 已配置阿里云 CLI（`aliyun`），RAM 身份可管理 CR（容器镜像仓库）。
- 一个 **ACR 企业版**实例（没有可在控制台创建——实例创建未开放 CLI）。本指南使用已有实例。

## 步骤 1 —— 配置 ACR 命名空间【云上操作】

```bash
# 1.1 列出 ACR EE 实例，记下实例 ID 与域名
aliyun cr ListInstance
#   例如实例 "cri-xxx"，域名 "registry.cn-hangzhou.cr.aliyuncs.com"

# 1.2 为 RL 任务镜像创建命名空间
aliyun cr CreateNamespace --InstanceId cri-xxx \
  --NamespaceName terminal-bench --AutoCreateRepo false

# 1.3 创建仓库
aliyun cr CreateRepository --InstanceId cri-xxx \
  --RepoNamespaceName terminal-bench --RepoName tasks \
  --RepoType PRIVATE --Summary "RL task sandbox images"

# 1.4 获取登录令牌并 docker login（临时令牌，约 1 小时有效）
TOKEN_JSON=$(aliyun cr GetAuthorizationToken --InstanceId cri-xxx)
docker login registry.cn-hangzhou.cr.aliyuncs.com \
  --username "$(echo "$TOKEN_JSON" | jq -r .data.tempUserName)" \
  --password-stdin <<<"$(echo "$TOKEN_JSON" | jq -r .data.authorizationToken)"
```

## 步骤 2 —— 安装 harbor CLI 并下载任务数据集

```bash
# 2.1 harbor CLI（任何装有 Python 3.10+ 的机器）
pip install harbor            # 或：uv tool install harbor

# 2.2 下载数据集（此处用 SWE-bench Verified）——每个任务一个目录
harbor datasets download swe-bench-verified --output-dir ./data
ls data/swe-bench-verified | head
#   astropy__astropy-12907  astropy__astropy-14309  django__django-16379 ...
```

每个任务目录包含 `task.toml`（指令/测试/镜像引用）、`instruction.md`、`tests/`，以及可选的
`environment/Dockerfile`。

> **自带数据集也可接入**：步骤 3～6 对任何符合 harbor 任务格式的目录都成立——你可以用
> `harbor datasets init` 从模板新建任务目录，或直接把自备的目录（每个任务一个子目录，内含
> `task.toml` + `instruction.md` + `tests/`，可选 `environment/Dockerfile` 或在 `task.toml` 里指定
> `docker_image`）放到同一个数据集根下，跳过 `harbor datasets download`。后续步骤把
> `--tasks-dir` / gen-prompts.sh 指向你的数据集根即可。

## 步骤 3 —— 构建并推送沙箱镜像

harbor 会构建每个任务的镜像并推送到你的 ACR 仓库（本地 Docker 负责构建）：

```bash
harbor admin upload-images \
  --tasks-dir ./data/swe-bench-verified \
  --registry registry.cn-hangzhou.cr.aliyuncs.com/terminal-bench/tasks \
  --sanitize-image-names \   # ACR 不接受 '__' 等字符，自动转为 '-'
  --update-config \          # 把 docker_image / image_sha256 回写进每个 task.toml
  --skip-unchanged \         # 可重复执行；跳过已推送的任务
  -n 4                       # 并行构建数
```

关键参数（ACK 场景最要紧的几个）：

| 参数 | 作用 | 在 ACK 上为什么重要 |
|---|---|---|
| `--registry <url>` | 当 task.toml 里没有 `docker_image` 时，用该仓库前缀合成镜像引用 | 指向你的 ACR 仓库；合成 tag 默认为 `YYYYMMDD` |
| `--remote-buildkit tcp://<host>:1234` | 经远程 BuildKit 端点构建（buildx remote driver，回退 `buildctl --addr`） | 在集群内构建——本地无需 Docker daemon，基础镜像拉取也不经过你的笔记本 |
| `--sanitize-image-names` | 改写仓库非法的任务名（`django__django-14349` → `django-django-14349`） | ACR 仓库路径不接受 `__`；不开这个推送会直接被 registry 拒绝 |
| `--update-config` / `--override-config` | 把 `docker_image`、`built_content_hash`、`image_sha256` 回写进 task.toml（override = 覆盖已有值） | RL 运行时从 task.toml 读镜像——不回写的话任务仍指向旧/缺失镜像 |
| `--skip-unchanged` | 记录的内容哈希与远端 digest 一致时跳过构建+推送 | 流水线可重跑：中断续传只重建有变化的任务 |
| `--diff-only` | 只对比本地哈希 vs 远端仓库，不构建；有漂移退出码 1 | CI 式漂移检查：确认推送的镜像与任务源一致 |
| `-n <N>` | 并行构建/推送数 | ACR + 远程 BuildKit 能吃下并发；4 是个好起点 |
| `--tag <tag>` / `--tag-latest` | 覆盖/替换 tag；同时打 `latest` | 为数据集版本钉一个稳定 tag，而不是依赖默认日期 |
| `--filter <子串>` | 按名字筛任务子集 | 只重建某个失败任务，不动其余 |

验证一个镜像：

```bash
docker manifest inspect registry.cn-hangzhou.cr.aliyuncs.com/terminal-bench/tasks/astropy-astropy-14309:latest
```

## 步骤 4 —— 在集群里创建镜像拉取 secret

pull secret 必须是**长期有效**的凭证（步骤 1 的令牌 1 小时就过期）。用实例的固定访问凭证
（ACR 控制台设置），或用 CLI 建一个专用 RAM 用户：

```bash
# （可选）专用只读 RAM 用户
aliyun ram CreateUser --UserName acr-rl-puller
aliyun ram CreateAccessKey --UserName acr-rl-puller       # 记下 AK/SK
aliyun ram AttachPolicyToUser --UserName acr-rl-puller \
  --PolicyType System --PolicyName AliyunContainerRegistryReadOnlyAccess

kubectl create secret docker-registry acr-pull-secret \
  --docker-server=registry.cn-hangzhou.cr.aliyuncs.com \
  --docker-username=<用户名> \
  --docker-password=<密码> \
  -n default
```

## 步骤 5 —— 验证集群内能拉取镜像

```bash
kubectl run tb-pull-test --restart=Never -- sleep 5 \
  --image=registry.cn-hangzhou.cr.aliyuncs.com/terminal-bench/tasks/astropy-astropy-14309:latest \
  --overrides='{"spec":{"imagePullSecrets":[{"name":"acr-pull-secret"}]}}'
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/tb-pull-test --timeout=300s
kubectl delete pod tb-pull-test
```

> 此处报 `insufficient_scope: authorization failed` 说明 secret 对该仓库没有权限
> （凭证错/实例错）——先修步骤 4 再继续。

## 步骤 6 —— 生成 prompt 清单

用随 [2-run-an-rl-task](../2-run-an-rl-task/gen-prompts.sh) 附带的生成脚本从数据集目录自动生成
`prompts.jsonl`（只认含 `task.toml` 的目录；`metadata.instance_id` 自动取任务目录名）：

```bash
bash ../2-run-an-rl-task/gen-prompts.sh ./data/swe-bench-verified -o prompts.jsonl -n 3
```

选项：`-n <数量>`（`0` = 全部任务）、`-o <文件>`、`--prompt "<指令>"`（默认为 SWE 修复指令）。
手工样例见 [2-run-an-rl-task/prompts.jsonl](../2-run-an-rl-task/prompts.jsonl)。

`metadata.instance_id` **必须**与任务目录名一致——rollout 时它填充任务路径模板
（`/var/model-dataset/swe-bench-verified/{instance_id}`）。

## 完成 —— 你现在拥有的

- ACR 中的任务镜像（各 `task.toml` 已引用），以及
- 一份 `prompts.jsonl` 清单，

可直接进入 [2-run-an-rl-task](../2-run-an-rl-task/README_ZH.md)。两种框架如何消费：
slime 经 `--prompt-data` 传入清单；verl 的 agentic 配方把 `local_harbor` 数据集 /
`task_path_template` 指向同一批任务目录——同一批任务、同一批镜像。

## 提示

- 难易任务要混合：GRPO 依赖**同一 prompt 组内的 reward 方差**；基础模型全部得 0 分的数据集没有学习信号。
- 任务数据集与清单一起放在共享卷上（run-task 指南将其挂载在 `/var/model-dataset`）。
