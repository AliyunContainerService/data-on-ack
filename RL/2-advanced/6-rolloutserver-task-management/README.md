# RolloutServer Task Management

This guide covers the full lifecycle of RolloutServer tasks (job / run / trial): submission, status and result queries, cancellation, retries, batch submission, and the corresponding SDK usage. For API authentication, see [Token Management](../7-rolloutserver-token-management/README.md).

## Concepts and Ports

- **run**: one rollout execution, identified by `run_id` (format `{task-dir-name}-{6-char uuid}`).
- **job**: a group of runs, specified by `job_id` at submission; quota and Viewer statistics are organized per job.
- **trial**: the execution artifacts of a run (trajectory, logs, verifier output, reward).

| Service | Port | Responsibility |
|---|---|---|
| ingress (server) | 8080 | Submission, status/result queries, cancellation, quota management |
| viewer | 8081 | Read-only job/trial/trajectory query API and UI |
| batch controller | 8082 | Dataset batch submission |

## Submit a Task

Both submission endpoints use multipart/form-data with two fields: `metadata` (a JSON string) and `task_archive` (a tar.gz of the task directory).

Required metadata fields:

| Field | Description |
|---|---|
| `job_id` | Job aggregation and quota dimension |
| `task_id` | Tracking identifier, joins with LLM proxy data |
| `task_path` | Directory name inside the archive (raw HTTP); with the SDK, a local directory path that the SDK archives for you |
| `agent` | Agent config, e.g. `{"name": "oracle"}` or `{"name": "mini-swe-agent", "model_name": "openai/...", "kwargs": {...}}` |

Common optional fields: `queue` (default `default`), `priority` (-100..100), `max_retries` (0..10, default 0), `timeout_multiplier` (default 1.0), `verifier.disable`, `environment_kwargs` / `environment_overrides` (allowed keys are backend-specific; discover via `GET /api/v1/capabilities/environments`), `llm_proxy_url`.

### Async submission (recommended)

```bash
curl -s -X POST $SERVER/api/v1/runs/async \
  -H "Authorization: Bearer $TOKEN" \
  -F 'metadata={"job_id":"demo","task_id":"t-001","task_path":"my-task","agent":{"name":"oracle"}}' \
  -F "task_archive=@my-task.tar.gz"
# → {"run_id": "my-task-a1b2c3", "status": "queued"}
```

Returns `run_id` immediately; poll status and result afterwards. Suitable for long trials and large batches.

### Sync submission

`POST /api/v1/runs` blocks until completion:

- Completed: 200 with the full result.
- Wait timeout: 202 with `{run_id, status: "running"}`; the trial keeps running and the result can be fetched later via the async query endpoints.
- Client disconnect: the trial becomes detached and keeps running on the worker; nothing is lost.

The sync endpoint holds an HTTP connection for the whole trial and is unsuitable for long trials or high concurrency.

## Query Status and Result

```bash
curl -s $SERVER/api/v1/runs/async/$RUN_ID/status -H "Authorization: Bearer $TOKEN"
curl -s $SERVER/api/v1/runs/async/$RUN_ID/result -H "Authorization: Bearer $TOKEN"
```

State machine: `queued` → `running` → `completed` / `failed` / `timeout` / `cancelled`.

- `status` returns the status.json content (status, job_id, queue, worker_id, timestamps, etc.).
- `result` returns **409** while unfinished; once terminal, the full result: `rewards`, `token_usage`, `rollout_details` (token_ids/logprobs, agent-dependent), `outcome` (passed / task_failed / errored / cancelled / unscored), `error`, `retry_count`, etc.
- `GET /api/v1/runs/async` lists recent runs with per-status counts (memory-cache based, best effort, not exhaustive).

The reward key varies with verifier configuration: `{"reward": ...}` when the verifier is enabled, `{"rewards": ...}` when disabled. Handle both.

## Cancel Tasks

```bash
# Cancel one run (works for sync and async submissions)
curl -s -X POST $SERVER/api/v1/runs/async/$RUN_ID/cancel -H "Authorization: Bearer $TOKEN"

# Cancel all queued/running runs of a job
curl -s -X POST $SERVER/api/v1/jobs/$JOB_ID/cancel -H "Authorization: Bearer $TOKEN"
```

Behavior:

- Queued runs are removed from the scheduler pool; running runs are aborted by their owning worker.
- The sandbox Pod is deleted and the quota slot is released.
- Idempotent: terminal runs return `already_terminal`; unknown runs return 404.
- Job cancellation returns `{cancelled[], already_terminal[], not_found[]}`.

There is no manual retry endpoint; re-run failed tasks by resubmitting.

## Retries and Timeouts

- `max_retries` (0..10, default 0): automatic retry **only for `failed` status**; `timeout`, cancelled, and detached trials are not retried. Exponential backoff 2s, 4s, 8s… capped at 60s. Retries reuse the same `run_id`; `retry_count` is returned with the result.
- `timeout_multiplier` (≥0.1, default 1.0): scales all task phase timeouts.
- Recovery after worker crashes (requeue / attach-and-resume) is an internal server mechanism requiring no user action.

## Inspect Jobs and Trials (Viewer)

Trial data queries are served by the viewer (8081), under the `/api` prefix:

```bash
curl -s $VIEWER/api/jobs                          # job list (pagination/filters)
curl -s $VIEWER/api/jobs/$JOB_ID                  # job detail, stats, failure analytics
curl -s $VIEWER/api/jobs/$JOB_ID/trials           # trial list
curl -s $VIEWER/api/jobs/$JOB_ID/trials/$RUN_ID/trajectory      # trajectory
curl -s $VIEWER/api/jobs/$JOB_ID/trials/$RUN_ID/verifier-output # verifier output
curl -s $VIEWER/api/jobs/$JOB_ID/trials/$RUN_ID/files           # artifact listing
```

Job quota management (ingress 8080; writes require an admin token):

- `GET /api/v1/job-quotas`: defaults and per-job usage.
- `PUT /api/v1/job-quotas`: runtime defaults (persisted in Redis).
- `PUT /api/v1/job-quotas/{job_id}`: per-job override (Redis, 30-day TTL). **Replaces the whole spec** — sending partial fields clears the rest. Allowed keys: `max_concurrent`, `max_queued`, `rate_limit`, `default_priority`, `max_priority`.

## Batch Submission (Batch Controller)

When the dataset already lives on the cluster, submit by dataset through the batch controller (8082):

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

curl -s $BATCH/api/v1/batches/$BATCH_ID          # progress and counts
curl -s $BATCH/api/v1/batches/$BATCH_ID/results  # per-item results and summary
```

Notes:

- `item_selector` supports `"all"`, slices `"0:10"`, explicit lists `"0,3,5"`, and combinations.
- Batch statuses: pending / running / completed / failed / **partial** (some items failed).
- `DELETE /api/v1/batches/{id}` only removes the state record of a terminal batch; it does not cancel runs.
- Submission concurrency is capped by `batchController.maxConcurrentSubmissions` (default 10).

## Priority

`priority` ∈ [-100, 100] at submission. Effective priority = explicit run value > job default > queue default (0), capped by the queue's `max_priority`. Ordering is insert-only, never preemptive. See "Configure the Central Scheduler" in the high-concurrency guide.

## SDK Usage

```python
from sdk.client import AgentRunClient

client = AgentRunClient("http://<server>:8080", auth_token="<token>")

# Async submit + poll + result (recommended)
result = await client.run_async_task(
    task_path="python/examples/test-task",
    job_id="demo", task_id="t-001", agent_name="oracle",
)

# Or step by step
run_id = await client.submit_async_task(task_path=..., job_id=..., task_id=..., agent_name=...)
await client.poll_async_task(run_id)
result = await client.get_async_result(run_id)

# Cancel
await client.cancel_run(run_id)
await client.cancel_job("demo")

# Dataset batch
batch_id = await client.submit_dataset_batch(
    dataset_name="swe-bench-verified", job_id="eval-1",
    task_id_prefix="swe", queue="evaluation",
    batch_controller_url="http://<batch>:8082",
)
await client.poll_batch(batch_id)

await client.aclose()
```

Main methods:

| Method | Purpose |
|---|---|
| `submit()` / `submit_async()` | Sync-endpoint submission (blocks) |
| `submit_async_task()` | Async-endpoint submission, returns run_id |
| `poll_async_task()` / `get_async_result()` | Poll status / fetch result |
| `run_async_task()` | Submit + poll + result combined |
| `cancel_run()` / `cancel_job()` | Cancellation |
| `submit_dataset_batch()` / `poll_batch()` / `get_batch_results()` | Dataset batches |
| `aclose()` | Close the shared connection pool |

Known pitfalls:

- **Do not mix sync and async methods on one instance**: the shared httpx pool binds to an event loop, while `submit()` opens a fresh loop per call; mixing raises "Event loop is closed". Use a separate instance for sync calls, and `await aclose()` on async ones.
- `submit_async()` does not raise on HTTP errors: network failures return `status="timeout"`, non-200 returns `status="failed"` (empty run_id). For strict error handling use `submit_async_task()` (raises `AgentRunError`).
- Single-run submission methods do not expose `queue` / `priority`; route by queue via raw HTTP metadata or `submit_dataset_batch(queue=...)`.