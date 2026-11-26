# Supporting Tens of Thousands of Concurrent Rollouts

This guide describes how to configure RolloutServer (kube-rl) to run 10,000 to 50,000 rollouts concurrently. It covers the distributed architecture, deployment topology, concurrency and quota configuration, resource planning, sandbox capacity preparation, and post-deployment verification and monitoring.

The figures below come from actual tests under the following conditions: a single Pod (1 ingress process + 4 worker processes), `max_concurrent_trials=10000`, Oracle agent, hostNetwork, and minimized resource requests.

## Distributed Architecture

RolloutServer consists of the following roles. They can run in one Pod or scale independently:

- **ingress**: accepts HTTP submissions, performs admission checks (queue depth, submission rate), and writes admitted runs into the pending pool.
- **Central scheduler**: runs inside ingress processes. Multiple ingress processes elect a leader through a Redis lease; the leader dispatches: on each tick it selects runs from the pending pool by priority and queue/job quota, and delivers them to the shared task queue.
- **worker**: claims tasks from the shared task queue and executes trials in a sandbox backend (ACK Pod, E2B, or SSH). It reserves a local concurrency slot before claiming and returns it after execution.
- **viewer** (optional): read-only job/trial query API and UI.
- **Batch Controller** (optional): dataset batch submission service.

Redis is the coordination center for all roles: the task queue, quota counters, scheduler pending pool, and leader lease all live in Redis. Without Redis configured, the server refuses to start. Trial state and results are written to a shared storage volume (NAS); finished trajectories can be archived to OSS.

Request processing flow:

```
Submit → ingress admission (max_queued / rate_limit)
       → pending pool (ordered by priority)
       → leader dispatch (queue + job max_concurrent gating)
       → shared task queue → worker claim (slot reserved before claiming)
       → sandbox execution → result archiving
```

This architecture determines how you scale:

- All workers claim from the same queue, and the reserve-before-claim mechanism keeps distribution even. Adding workers raises total concurrency linearly; there is no task affinity or migration problem.
- In multi mode, ingress can also scale independently; the scheduler elects a leader automatically through the Redis lease, with no single point.
- Single mode packs the roles into one Pod; multi mode deploys ingress as a Deployment and workers as a StatefulSet, scaled independently (see "Choose a Deployment Mode").

## Capacity Overview

Measured concurrency of a single Pod under different memory limits:

| Pod memory limit | Safe concurrency ceiling | Peak memory | Notes |
|---|---|---|---|
| 4 Gi | ~1000 | ~3.5 Gi | Higher concurrency triggers OOM |
| 8 Gi | ~3000 | ~5.5 Gi | Headroom remains after the thread pool saturates |
| 16 Gi | ~10000 | 7.0 Gi (44%) | 9918 concurrent Running observed, no OOM, no restarts |

Total concurrency is determined by:

```
Total concurrency = max_concurrent_trials × total worker processes
```

- Single mode: total worker processes = `single.workers`.
- Multi mode: total worker processes = `multi.worker.replicas` (one worker process per Pod).

The default of `max_concurrent_trials` is 4. Set it explicitly before deploying large-scale concurrency.

Choose a path by target concurrency:

- Up to 10,000: single mode, one Pod with 16 Gi memory and 8 CPU.
- Above 10,000: multi mode, scale the worker StatefulSet horizontally, with external Redis and an RWX volume. Workers reserve a local slot before claiming, so tasks distribute evenly across the shared queue and horizontal scaling does not skew assignment.

## Prerequisites

Before you start, confirm:

- An ACK cluster with a large-node pool (recommended: 16+ cores, 64+ Gi memory, 500 Gi SSD).
- A reachable Redis instance. Redis is a hard dependency: the task queue, quota counters, and scheduler state all live there. Without `redis_url`, the server refuses to start.
- For multi mode, a ReadWriteMany (RWX) volume such as `alicloud-nas-fs` or `alicloud-cpfs`. The cluster's default block storage does not qualify.
- The RolloutServer image and its imagePullSecret.

## Choose a Deployment Mode

| Value | Default | Description |
|---|---|---|
| `deploymentMode` | `single` | single: N ingress processes + M worker processes in one Pod. multi: separate ingress Deployment and worker StatefulSet |
| `single.ingressProcs` | 2 | Ingress processes per Pod, sharing the port via SO_REUSEPORT |
| `single.workers` | 4 | Worker processes per Pod |
| `multi.ingress.replicas` | 2 | Ingress Deployment replicas |
| `multi.worker.replicas` | 4 | Worker StatefulSet replicas; the Pod ordinal is the worker id |
| `noopMode` | false | Skips tar extraction, Pod creation, and trial execution; tests the dispatch path only |

Single mode enforces `replicaCount=1`. Setting it higher fails Helm rendering because worker ids would collide across Pods.

### Configure single mode (10,000 concurrent)

These values are capacity-verified:

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

Sidecar Redis stores data in emptyDir; deleting the Pod loses the queue. Use external Redis in production.

### Configure multi mode (tens of thousands)

This example configures 8 workers at 5000 concurrency each, for a 40,000 ceiling:

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
    existingSecret: kube-rl-redis   # key: redis-url, value: full connection URL

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

The chart's default resource limits (2 CPU / 2 Gi) cannot sustain tens of thousands of concurrent rollouts. Size each Pod per "Plan Memory Resources".

ConfigMap changes roll Pods automatically via the `checksum/config` annotation. Secret-only changes (tokens, redis-url) require a manual `kubectl rollout restart`.

## Configure Worker Concurrency

1. Set `config.max_concurrent_trials` to the target concurrency per worker process. This value drives the execution semaphore, the claim-slot semaphore, and the k8s-exec thread pool size.
2. Set the worker process count (`single.workers` or `multi.worker.replicas`) so the product equals your target total concurrency.
3. For local tests on macOS, raise the OS accept queue: `sudo sysctl -w kern.ipc.somaxconn=4096`. The Linux default of 4096 is sufficient.

Related settings:

| Setting | Default | Description |
|---|---|---|
| `max_concurrent_trials` | 4 | Concurrency ceiling per worker |
| `exec_api_pool_size` | 256 | K8s exec WebSocket pool ceiling (ACK backend) |
| `listen_backlog` | 4096 | Ingress listen backlog, capped by OS somaxconn |
| `use_async_exec` | false | nohup+poll instead of long-lived kubectl exec; fewer WebSockets at scale |
| `shutdown_grace_period` | 30s | Graceful shutdown wait; timed-out tasks are requeued |
| `async_status_cache_max_entries` | 5000 | Per-process in-memory status cache ceiling |
| `pod_health_check_interval_sec` | 30s | Detects deleted Pods and cancels stuck trials |
| `pod_health_check_page_size` | 0 (full list) | Set pagination (e.g. 500) at 10k+ Pods to avoid parsing 50–100MB responses |
| `reconciler_interval_sec` | 300 (chart overrides to 60) | Orphan Pod GC interval |
| `max_orphan_pod_age_sec` | 7200 (chart overrides to 600) | Orphan Pod age threshold |
| `rate_limit_enabled` | false | Per-IP rate limiting switch |
| `rate_limit_submit` / `rate_limit_default` | 10/minute / 60/minute | Rate limits for submissions and other requests |

These can be overridden via environment variables: `KUBE_RL_MAX_CONCURRENT_TRIALS`, `KUBE_RL_NUM_WORKERS`, `KUBE_RL_NUM_INGRESS`, `KUBE_RL_LISTEN_BACKLOG`.

The following limits are hard-coded. Account for them when planning capacity:

- The worker k8s-exec thread pool is `min(max(32, max_concurrent_trials×4), 256)`, capped at 256. Tests show raising the cap hurts performance: the apiserver is the bottleneck, and 256 acts as backpressure.
- The Redis client uses `max_connections=128` with an LPUSH concurrency limit of `Semaphore(64)`. This is a verified backpressure combination; do not raise it.
- Ingress HTTP thread pool 32, file I/O pool 16, tar extraction pool 16.
- ACK K8s connection pool 256. When `max_concurrent_trials` exceeds 256, Pod creation requests queue at the connection pool — normal rate limiting.

## Plan Memory Resources

Each worker process adds roughly 1.4 Gi at the 10,000-concurrency watermark. Plan with:

```
Pod memory ≥ baseline 2.5 Gi + worker processes × 1.5 Gi, plus 50% headroom
```

Single mode with 4 workers maps to 16 Gi. In multi mode, give each worker Pod 16 Gi independently.

Why OOM happens at 4 Gi:

```
max_concurrent_trials=10000
  → k8s-exec pool = 256 threads/worker
  → 4 workers × 256 threads × ~1MB stack ≈ 1 Gi
  → baseline 2.5 Gi + threads 1 Gi + concurrency metadata > 4 Gi → OOM
```

At 16 Gi, peak memory for 10,000 concurrent is 7.0 Gi: baseline 2.5 Gi + threads 1 Gi + metadata 3.5 Gi.

Each worker holds ~293 threads (k8s-exec 256, io-aux 16, tar 6, gRPC 12, etc.). This is normal ThreadPoolExecutor behavior — threads are created on demand and not reclaimed — not a leak.

## Configure Queue Quotas

Quota has two layers; queue and job dimensions apply together:

| Layer | Field | Behavior |
|---|---|---|
| ingress admission | `max_queued` | Queue depth ceiling; 429 beyond it |
| ingress admission | `rate_limit` | Submission sliding window ("N/second\|minute\|hour"); 429 beyond it |
| scheduler/worker | `max_concurrent` | running + detached ceiling; excess waits in the pending pool |

Configure in values:

```yaml
config:
  quotas:
    default: { max_concurrent: 40000, max_queued: 100000 }
    rl:        { max_concurrent: 30000, max_queued: 80000 }
    evaluation: { max_concurrent: 10000, max_queued: 20000 }
  job_quota:
    max_concurrent: 5000
```

Behaviors to note:

- The `default` queue always exists. Queue names not listed in `quotas` are not rejected, but have no queue-level limits. Set limits explicitly for every queue you use at scale.
- Queue specs in the config file accept only `max_concurrent`, `max_queued`, and `rate_limit`. `default_priority` and `max_priority` are set via API.
- `PUT /api/v1/quotas/{queue}` changes live in memory only and revert to config values on restart.
- `PUT /api/v1/job-quotas/{job_id}` changes persist in Redis (30-day TTL) and replace the whole spec — sending partial fields clears the rest.
- Workers write a heartbeat every 30s (`quota_heartbeat_interval_sec`). When a heartbeat is older than 300s (`quota_stale_run_threshold_sec`), the ingress reconcile loop (every 60s, `quota_reconcile_interval_sec`) marks the run failed and reclaims its slot. After a worker crash holds slots, run `POST /api/v1/quotas/reconcile` manually.

## Configure the Central Scheduler

The central scheduler is always on; there is no switch. Every ingress process runs it, but only the Redis lease holder (the leader) dispatches.

| Setting | Default | Description |
|---|---|---|
| `scheduler_tick_interval_sec` | 1.0 | Leader recomputes capacity and dispatches each tick |
| `scheduler_lease_ttl_sec` | 15 | Leader lease TTL |
| `scheduler_dispatch_timeout_sec` | 60 | Re-dispatches if a promoted run is not running after 60s (run_id dedup keeps it idempotent) |
| `priority_policy` | `static` | static: pure priority, FIFO ties. aging: wait-time boost |
| `priority_aging_step_sec` | 60 | Effective priority +1 per 60s of waiting |
| `priority_aging_max_boost` | 100 | Boost ceiling; set above the largest priority gap in use, otherwise saturated runs fall back to pure priority |

Priority resolves as run > job default > queue default, capped by the queue's `max_priority`. Ordering is insert-only, never preemptive: a high-priority run still waits for running trials to free slots.

Observe scheduler state via `GET /api/v1/scheduler` (is_leader, pending depth, active, inflight). Only the leader reports the queues field.

Scheduler behavior with tens of thousands queued: each tick, the leader does a full `ZRANGE` per queue and one `HGETALL` per run (serial await). 30,000 queued means roughly 30,000 Redis round trips per second — the main dispatch throughput bottleneck. Mitigations:

- Cap pending depth with `max_queued`; submit in batches from the client.
- Split into multiple queues by business line to spread depth.
- Raising `scheduler_tick_interval_sec` (e.g. 2–5s) lowers Redis load but not throughput. Dispatch has no per-tick batch cap.

## Configure Redis

Single mode can use sidecar Redis (demos and load tests). Multi mode and production use external Redis:

```yaml
redis:
  mode: external
  external:
    existingSecret: kube-rl-redis   # key must be redis-url, value is the full URL
```

`redis.mode: none` fails Helm rendering.

The client uses `redis.asyncio` with timeout retries and atomic reconnect. `redis_socket_timeout` and `redis_socket_connect_timeout` default to 5s; `redis_health_check_interval` to 30s.

At tens-of-thousands scale, deploy external Redis independently (replication or cluster edition) and monitor connections and memory. The pending pool stores one full submission message per run (meta HASH); watch Redis memory when tens of thousands are queued.

## Configure Storage

| Setting | Description |
|---|---|
| `persistence.nas` (RWX) | Hot volume: run store, active task archives, trial output. Use NAS/CPFS; size by peak concurrency × retention |
| `persistence.archive.existingClaim` | Static OSS PVC mapped to `config.archive_dir`. Finished trials are written as a single `{run_id}.tar.gz` plus `{run_id}.trajectory.json`. Enable this at tens-of-thousands trajectory volume, otherwise the run store grows without bound. OSS CSI has no dynamic provisioning; pre-create the PVC |
| `archive_after_seconds` / `archive_check_interval` | Archive 3600s after completion; scan every 300s |
| `persistence.enabled: false` | emptyDir, demos only — results lost on restart |

Metadata I/O amplification at scale:

- Each running run writes a status.json heartbeat every 30s: 10,000 running ≈ 333 writes/s.
- Ingress reconcile reads status.json per run every 60s: tens of thousands queued means tens of thousands of reads per minute.
- Each process scans the whole run store at startup to load the status cache. The cache caps at 5000 entries and active entries cannot be evicted; with tens of thousands active, status queries fall back to NAS reads.

Choose low-latency NAS/CPFS for the hot volume and enable archiving to move cold data to OSS.

## Prepare ACK Sandbox Capacity

The ACK backend has no separate concurrency parameter. The effective limit chain: `max_concurrent_trials` → k8s-exec pool (256) → K8s connection pool (256) → apiserver throughput → node maxPods and CNI.

Prepare the cluster:

1. Label large nodes: `kubectl label node <node-name> nodetype=big`.
2. Raise node maxPods from the default 213 to 1000: edit `/var/lib/kubelet/ack-managed-config.yaml` and restart kubelet.
3. Enable hostNetwork in submissions to eliminate CNI setup latency:

```json
"environment_kwargs": {
  "node_selector": {"nodetype": "big"},
  "skip_image_check": true,
  "pod_overrides": {"spec": {"hostNetwork": true, "dnsPolicy": "ClusterFirstWithHostNet"}}
}
```

4. For load tests, minimize resource requests (defaults cpus=1, memory_mb=2048 exhaust node allocatable resources at 10k concurrency):

```json
"environment_overrides": {"override_cpus": 0, "override_memory_mb": 10, "override_storage_mb": 100}
```

Resource minimization applies to load tests and light tasks only. Give real agent tasks the resources they need and recompute node capacity accordingly.

Warm pool (SandboxClaim) parameters: `use_sandbox_claim`, `sandboxset_replicas` (default 5), `claim_timeout` (300s), `sandbox_ready_timeout_sec` (120s).

## Prepare E2B Sandbox Capacity

E2B capacity is determined by the SandboxSet warm pool, not server configuration. Raising `max_concurrent_trials` does not create sandboxes: trials beyond pool capacity block on claim and fail after `claim_timeout` (300s).

Scale the pool:

```bash
kubectl scale sandboxset <name> --replicas=<target concurrency>
```

Submission notes:

- Always include `sandbox_set_name`; otherwise E2B tries to build a template and returns 404.
- `override_claim_image: true` lets the task's own `docker_image` override the pool image. This option is unreliable on some clusters; the safe approach is one SandboxSet per image.
- `E2B_API_KEY`, `E2B_API_URL`, `E2B_SANDBOX_URL` are injected via `config.env_profiles`. env_profiles accepts infrastructure credentials only, never model keys.

## Submit Rollouts in Bulk

Choose the submission path by scenario:

- Tens of thousands of tasks: use the async API (`POST /api/v1/runs/async` + polling) or SDK `run_async_task()`. The sync endpoint holds an HTTP connection for the whole trial and is unsuitable at 10k scale.
- Dataset already on the cluster: use the Batch Controller (`POST /api/v1/batches`, port 8082) and raise `batchController.maxConcurrentSubmissions` (default 10).
- RL training loops: use SDK `run_async_task()`.

Adjust two throttling points before bulk submission:

- `rate_limit_submit`: default 10/minute per IP. Raise it or set `rate_limit_enabled: false`.
- `batchController.maxConcurrentSubmissions`: default 10.

Cap in-flight submissions on the client (e.g. `asyncio.Semaphore(200)`), submit in rate-controlled batches, and retry on 429 caused by `max_queued`.

## Verify the Deployment

Verify in this order:

1. Check `GET /health`: the server is reachable and at least one worker is registered.
2. Check `GET /api/v1/scheduler`: `enabled: true` with a leader present.
3. Submit one rollout with `agent=oracle`; confirm it reaches `completed` with a non-zero reward.
4. Step-load test (100 → 1000 → 5000 → target concurrency); at each step watch memory, thread count, loop_lag, and the 429 ratio.
5. Delete one worker Pod; confirm processing-list recovery and slot reclamation (300s heartbeat + reconcile).

Load-test clients should submit from multiple processes (each with its own event loop and connection pool) to break the single-process submission ceiling. To test the dispatch path only, set `noopMode: true`.

For real-Pod capacity tests, use Oracle agent + `sleep` to hold trials running, with hostNetwork and minimized resources; run the submission script as an in-cluster Job to avoid local network limits.

## Monitor Runtime State

```bash
curl -s $SERVER/health                    # worker list and status
curl -s $SERVER/debug/memory              # ingress RSS and threads
curl -s $SERVER/metrics                   # Prometheus metrics (rollout_server_* prefix)
curl -s $SERVER/api/v1/scheduler          # leader, pending depth, inflight
curl -s $SERVER/api/v1/quotas             # queued/active per queue
kubectl top pod -n kube-rl                # Pod memory watermark
kubectl get pods -n kube-rl | awk '{print $3}' | sort | uniq -c   # trial Pod phase distribution
```

Alert thresholds to watch:

| Metric | Threshold | Meaning |
|---|---|---|
| `loop_lag` p50 | > 100ms | Event loop blocked by synchronous work |
| Pod memory usage | > 70% | Approaching OOM risk |
| restartCount | > 0 | OOM or crash occurred |
| pending depth | sustained near `max_queued` | Submission rate exceeds drain capacity |

## Troubleshoot

Triage in the order quota → workers → sandbox pod → task:

| Symptom | Cause | Action |
|---|---|---|
| Many runs stuck `queued` | `max_concurrent` full or insufficient workers | Check `/api/v1/quotas` and the workers array in `/health` |
| `queued` with idle quota | Crashed worker holding slots | Run `POST /api/v1/quotas/reconcile` |
| 429 | `max_queued` full or rate limit hit | The response body distinguishes them; adjust limits or slow the client |
| Many Pods Pending/Unschedulable | Node resources, maxPods, or CNI | Resource overrides, maxPods=1000, hostNetwork |
| E2B claim timeout | Insufficient SandboxSet replicas | `kubectl scale sandboxset` |
| OOM restarts | Undersized memory | Adjust limits per "Plan Memory Resources" |
| Results lost after restart | `persistence.enabled: false` | Enable RWX persistence |
| Dispatch latency grows with queued count | Leader O(N) tick | Cap depth, split queues, batch submissions |

## Known Limitations

These limitations cannot be removed by configuration in the current version. Understand their behavior and mitigations when planning capacity.

### Dispatch overhead grows with queue depth

Each tick (default 1s), the leader scans the entire pending pool and issues one Redis query per run. With 30,000 runs queued, that is roughly 30,000 serial Redis round trips per second, and dispatch latency rises noticeably with depth. Raising `scheduler_tick_interval_sec` lowers scan frequency but not per-scan cost.

Mitigations: cap pending depth with `max_queued`; split into multiple queues by business line; submit in batches instead of flooding tens of thousands of requests at once.

### NAS metadata I/O amplification

Each running run writes a heartbeat file every 30s (10,000 running ≈ 333 writes/s); the ingress reconcile loop reads each run's state file every 60s — tens of thousands queued means tens of thousands of reads per minute; every process also scans the whole run store at startup. These small-file operations concentrate on NAS metadata performance: when NAS latency is high, heartbeats and status queries slow down, which can further cause runs to be misjudged as timed out.

Mitigations: choose low-latency NAS/CPFS for the hot volume; enable the archive volume to move finished trajectories to OSS; do not keep cold data on the hot volume long-term.

### Sandbox creation rate is limited by the apiserver

The worker's K8s connection pool and exec thread pool cap at 256 — deliberate backpressure: the apiserver is the shared bottleneck, and tests show raising the cap hurts performance. When `max_concurrent_trials` exceeds 256, Pod creation requests queue at the connection pool and sandbox startup slows.

Mitigations: enable hostNetwork to remove CNI setup latency; set `skip_image_check` to reduce pre-submission checks; use warm pools (SandboxClaim / SandboxSet) to pre-create sandboxes.

### Status cache overflow

The per-process in-memory status cache caps at 5000 entries, and queued/running entries cannot be evicted. Beyond 5000 active runs the cache overflows and emits a warning; status queries fall back to NAS file reads and query latency rises. This is expected behavior and does not affect result correctness.

Mitigation: no action required; if query latency matters, limit the number of active runs per process.

### Redis single point and data durability

In single mode, sidecar Redis data lives in emptyDir: deleting the Pod loses the queue and quota state. A single-instance Redis deployment is itself a single point.

Mitigations: use external Redis (replication or cluster edition) in production; multi mode requires external Redis.

### Default throttles on bulk submission

Two default throttles are easy to miss: `rate_limit_submit` limits one IP to 10 submissions per minute; `batchController.maxConcurrentSubmissions` defaults to 10 concurrent. When flooding tens of thousands of tasks, these cause 429 rejections or slow progress.

Mitigations: raise `rate_limit_submit` or set `rate_limit_enabled: false`; raise `maxConcurrentSubmissions`.