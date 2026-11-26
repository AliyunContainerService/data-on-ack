# RolloutServer 任务管理

本文介绍 RolloutServer 的任务（job / run / trial）全生命周期管理：提交、查询、取消、重试、批量提交，以及对应的 SDK 用法。API 鉴权见 [Token 管理](../7-rolloutserver-token-management/README_ZH.md)。

## 概念与端口

- **run**：一次 rollout 执行，唯一标识 `run_id`（格式 `{task目录名}-{6位uuid}`）。
- **job**：一组 run 的聚合单元，由提交时的 `job_id` 指定；配额与 Viewer 统计都按 job 维度组织。
- **trial**：run 的执行产物（轨迹、日志、verifier 输出、reward）。

| 服务 | 端口 | 职责 |
|---|---|---|
| ingress（server） | 8080 | 提交、状态/结果查询、取消、配额管理 |
| viewer | 8081 | 只读的 job/trial/轨迹查询 API 与 UI |
| batch controller | 8082 | 数据集批量提交 |

## 提交任务

两个提交端点都使用 multipart/form-data，包含两个字段：`metadata`（JSON 字符串）和 `task_archive`（task 目录的 tar.gz 包）。

metadata 必填字段：

| 字段 | 说明 |
|---|---|
| `job_id` | job 聚合与配额维度 |
| `task_id` | 任务追踪标识，与 LLM proxy 数据关联 |
| `task_path` | 压缩包内的 task 目录名（raw HTTP）；SDK 传本地目录路径，由 SDK 打包 |
| `agent` | agent 配置，如 `{"name": "oracle"}` 或 `{"name": "mini-swe-agent", "model_name": "openai/...", "kwargs": {...}}` |

常用可选字段：`queue`（默认 `default`）、`priority`（-100..100）、`max_retries`（0..10，默认 0）、`timeout_multiplier`（默认 1.0）、`verifier.disable`、`environment_kwargs` / `environment_overrides`（允许的键由 backend 决定，通过 `GET /api/v1/capabilities/environments` 查询）、`llm_proxy_url`。

### 异步提交（推荐）

```bash
curl -s -X POST $SERVER/api/v1/runs/async \
  -H "Authorization: Bearer $TOKEN" \
  -F 'metadata={"job_id":"demo","task_id":"t-001","task_path":"my-task","agent":{"name":"oracle"}}' \
  -F "task_archive=@my-task.tar.gz"
# → {"run_id": "my-task-a1b2c3", "status": "queued"}
```

立即返回 `run_id`，之后轮询状态与结果。适合长任务与大批量。

### 同步提交

`POST /api/v1/runs` 阻塞等待执行完成：

- 完成：200 返回完整结果。
- 等待超时：202 返回 `{run_id, status: "running"}`，任务继续执行，之后用异步查询端点取结果。
- 客户端断连：任务转为 detached 继续在 worker 执行，不丢失。

同步端点在整个 trial 期间持有 HTTP 连接，不适合长任务与高并发场景。

## 查询状态与结果

```bash
curl -s $SERVER/api/v1/runs/async/$RUN_ID/status -H "Authorization: Bearer $TOKEN"
curl -s $SERVER/api/v1/runs/async/$RUN_ID/result -H "Authorization: Bearer $TOKEN"
```

状态机：`queued` → `running` → `completed` / `failed` / `timeout` / `cancelled`。

- `status` 返回 status.json 内容（status、job_id、queue、worker_id、时间戳等）。
- `result` 在未完成时返回 **409**；完成后返回完整结果：`rewards`、`token_usage`、`rollout_details`（token_ids/logprobs，取决于 agent）、`outcome`（passed / task_failed / errored / cancelled / unscored）、`error`、`retry_count` 等。
- `GET /api/v1/runs/async` 列出近期 runs 与各状态计数（基于内存缓存，尽力而为，不保证全量）。

reward 的键名随 verifier 配置变化：启用 verifier 时为 `{"reward": ...}`，禁用时为 `{"rewards": ...}`，读取时两者都兼容。

## 取消任务

```bash
# 取消单个 run（同步/异步提交的都可以）
curl -s -X POST $SERVER/api/v1/runs/async/$RUN_ID/cancel -H "Authorization: Bearer $TOKEN"

# 取消整个 job 下的 queued/running run
curl -s -X POST $SERVER/api/v1/jobs/$JOB_ID/cancel -H "Authorization: Bearer $TOKEN"
```

行为：

- queued 的 run 直接从调度池摘除；running 的 run 由所属 worker 中止执行。
- 沙箱 Pod 会被删除，配额槽位释放。
- 幂等：已终态的 run 返回 `already_terminal`；未知 run 返回 404。
- job 取消返回 `{cancelled[], already_terminal[], not_found[]}`。

没有手动 retry 端点；失败任务重跑需要重新提交。

## 重试与超时

- `max_retries`（0..10，默认 0）：**仅 `failed` 状态自动重试**；`timeout`、取消、detached 不重试。指数退避 2s、4s、8s…封顶 60s。重试复用同一 `run_id`，`retry_count` 随结果返回。
- `timeout_multiplier`（≥0.1，默认 1.0）：整体缩放 task 的各阶段超时。
- worker 崩溃后的任务恢复（requeue / attach 续跑）是服务端内部机制，不需要用户干预。

## 查看 job 与 trial（Viewer）

trial 数据查询由 viewer（8081）提供，路由前缀 `/api`：

```bash
curl -s $VIEWER/api/jobs                          # job 列表（分页/过滤）
curl -s $VIEWER/api/jobs/$JOB_ID                  # job 详情与统计、失败分析
curl -s $VIEWER/api/jobs/$JOB_ID/trials           # trial 列表
curl -s $VIEWER/api/jobs/$JOB_ID/trials/$RUN_ID/trajectory      # 轨迹
curl -s $VIEWER/api/jobs/$JOB_ID/trials/$RUN_ID/verifier-output # verifier 输出
curl -s $VIEWER/api/jobs/$JOB_ID/trials/$RUN_ID/files           # 产物文件列表
```

job 配额管理（ingress 8080，写操作需 admin token）：

- `GET /api/v1/job-quotas`：默认配额与各 job 用量。
- `PUT /api/v1/job-quotas`：运行时默认配额（Redis 持久化）。
- `PUT /api/v1/job-quotas/{job_id}`：单 job 覆盖（Redis 30 天 TTL）。**整体替换** spec，只发部分字段会清除其余字段；允许键：`max_concurrent`、`max_queued`、`rate_limit`、`default_priority`、`max_priority`。

## 批量提交（Batch Controller）

数据集已在集群上时，用 batch controller（8082）按数据集批量提交：

```bash
curl -s -X POST $BATCH/api/v1/batches -H "Content-Type: application/json" -d '{
  "dataset_name": "swe-bench-verified",
  "item_selector": "0:10",
  "job_id": "eval-epoch-1",
  "task_id_prefix": "swe",
  "agent": {"name": "oracle"},
  "queue": "evaluation"
}'
# → {"batch_id": "batch-xxxxxxxxxxxx", "status": "submitted", "items": [...]}

curl -s $BATCH/api/v1/batches/$BATCH_ID          # 进度与计数
curl -s $BATCH/api/v1/batches/$BATCH_ID/results  # 逐 item 结果与汇总
```

要点：

- `item_selector` 支持 `"all"`、切片 `"0:10"`、显式 `"0,3,5"` 及混合。
- batch 状态：pending / running / completed / failed / **partial**（部分失败）。
- `DELETE /api/v1/batches/{id}` 仅删除终态 batch 的状态记录，不取消 run。
- 并发提交上限由 `batchController.maxConcurrentSubmissions` 控制（默认 10）。

## 优先级

提交时 `priority` ∈ [-100, 100]。生效优先级 = run 显式值 > job 默认 > queue 默认（0），受 queue `max_priority` 封顶。排序为插入式、不抢占。详见高并发指南的"配置中央调度器"。

## SDK 用法

```python
from sdk.client import AgentRunClient

client = AgentRunClient("http://<server>:8080", auth_token="<token>")

# 异步提交 + 轮询 + 取结果（推荐）
result = await client.run_async_task(
    task_path="python/examples/test-task",
    job_id="demo", task_id="t-001", agent_name="oracle",
)

# 或分步控制
run_id = await client.submit_async_task(task_path=..., job_id=..., task_id=..., agent_name=...)
await client.poll_async_task(run_id)
result = await client.get_async_result(run_id)

# 取消
await client.cancel_run(run_id)
await client.cancel_job("demo")

# 数据集批量
batch_id = await client.submit_dataset_batch(
    dataset_name="swe-bench-verified", job_id="eval-1",
    task_id_prefix="swe", queue="evaluation",
    batch_controller_url="http://<batch>:8082",
)
await client.poll_batch(batch_id)

await client.aclose()
```

主要方法：

| 方法 | 作用 |
|---|---|
| `submit()` / `submit_async()` | 同步端点提交（阻塞等待） |
| `submit_async_task()` | 异步端点提交，返回 run_id |
| `poll_async_task()` / `get_async_result()` | 轮询状态 / 取结果 |
| `run_async_task()` | 提交 + 轮询 + 取结果组合 |
| `cancel_run()` / `cancel_job()` | 取消 |
| `submit_dataset_batch()` / `poll_batch()` / `get_batch_results()` | 数据集批量 |
| `aclose()` | 关闭共享连接池 |

已知注意事项：

- **同一实例不要混用同步与异步方法**：共享 httpx 连接池绑定 event loop，`submit()` 每次新开 loop，混用会报 "Event loop is closed"。同步调用用独立实例，异步实例用完 `await aclose()`。
- `submit_async()` 不抛 HTTP 错误：网络异常返回 `status="timeout"`、非 200 返回 `status="failed"`（run_id 为空）；需要严格错误处理时用 `submit_async_task()`（抛 `AgentRunError`）。
- 单 run 提交方法不暴露 `queue` / `priority` 参数；按 queue 路由请用 raw HTTP metadata 或 `submit_dataset_batch(queue=...)`。