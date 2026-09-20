# Manage Trajectories with RolloutServer

Step-by-step guide to deploy a **rollout server** (the `kube-rl` Helm release) on ACK and use it to
run trials, persist trajectories, and serve RL training data.

**Why.** In the basic [run an RL task](../../1-user-guide/2-run-an-rl-task/README.md) flow, the
training framework drives sandbox trials directly and trajectories land as local files. At scale you
want a control plane that owns trial execution and trajectory lifecycle: fault tolerance (heartbeat
liveness, backoff retries, per-phase timeouts, attach-and-resume recovery), quota, cost attribution,
and a queryable store of every trajectory (`trajectory.json`, per-round logs, `rollout_details` =
token_ids/logprobs) with a Viewer (Job/Task/Trial, timeline, comparison grid).

**Prerequisites**

- An ACK cluster, `kubectl` + Helm 3.
- The server image in a registry the cluster can pull (see
  [1-prepare-dataset](../../1-user-guide/1-prepare-dataset/README.md) Steps 1–4 for ACR + pull secret).
- Aliyun CLI (`aliyun`) for the NAS below.

## Step 1 — Create persistence storage (NAS) [cloud]

```bash
aliyun nas CreateFileSystem \
  --RegionId cn-hangzhou --ZoneId cn-hangzhou-i \
  --FileSystemType standard --StorageType Performance \
  --ProtocolType NFS --VpcId <vpc-id> --VSwitchId <vswitch-id>
aliyun nas CreateMountTarget --FileSystemId <fs-id> --NetworkType Vpc \
  --VSwitchId <vswitch-id> --AccessGroupName DEFAULT_VPC_GROUP_NAME
aliyun nas DescribeMountTargets --FileSystemId <fs-id>    # wait for Active, note the domain
```

## Step 2 — Install the rollout server (Helm)

The chart ships with the RolloutServer project (`charts/kube-rl`). Minimal install:

```bash
helm upgrade --install kube-rl charts/kube-rl \
  -n kube-rl --create-namespace \
  --set image.repository=<acr-registry>/harbor-server \
  --set image.tag=<tag> \
  --set imagePullSecrets[0].name=acr-pull-secret \
  --set config.namespace=kube-rl
kubectl -n kube-rl get pod,svc    # expect: pod kube-rl-... 1/1 Running
```

What you get: Deployment `kube-rl` + Service `kube-rl` (HTTP entry), a Viewer, and per default the
`single` topology (each pod runs ingress + workers + a Redis sidecar; state persists on the
`persistence.nas` volume). For production use a values file (see the project's values examples:
demo / E2B / quota / large-concurrency) and mount the NAS via `persistence.nas`.

## Step 3 — Verify

```bash
kubectl -n kube-rl port-forward svc/kube-rl 8080:8080 &
curl -s http://localhost:8080/health | python3 -m json.tool   # expect status: healthy
```

## Step 4 — Submit your first task

Port-forward the Viewer too, then use the bundled script (packages the task dir, submits, polls to
completion, prints the reward):

```bash
kubectl -n kube-rl port-forward svc/kube-rl-viewer 8081:8081 &

SERVER=http://localhost:8080 VIEWER=http://localhost:8081 \
TASK_DIR=./data/swe-bench-verified/astropy__astropy-14309 \
AGENT=oracle bash submit-task.sh
```

You will see the trial move through phases — `env_starting → agent_running → verifying →
completed` — then a `result` JSON containing the reward (`oracle` runs the reference solution;
reward ≈ 1.0 confirms the plumbing). The trajectory (per-round `command.txt/stdout/stderr`,
`trajectory.json`, `rollout_details`) is persisted and browsable in the Viewer at
`http://localhost:8081/`.

Equivalent raw HTTP (what the script does):

```bash
TASK=astropy__astropy-14309
tar czf /tmp/$TASK.tar.gz -C ./data/swe-bench-verified "$TASK"
META='{"job_id":"demo","task_id":"'$TASK'-oracle","task_path":"'$TASK'","agent":{"name":"oracle"}}'
RUN_ID=$(curl -s -X POST "$SERVER/api/v1/runs/async" \
  -F "metadata=$META" -F "task_archive=@/tmp/$TASK.tar.gz" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["run_id"])')
curl -s "$SERVER/api/v1/runs/async/$RUN_ID/status" | python3 -m json.tool   # poll
curl -s "$SERVER/api/v1/runs/async/$RUN_ID/result" | python3 -m json.tool   # reward
```

**Variants**

- Swap the agent with `AGENT=<name>` (e.g. `terminus-2`, `claude-code`; `nop` for a no-op — the
  script then disables the verifier).
- Sandbox-pool backends: `POOL=<pool>` adds `environment_kwargs.sandbox_set_name` +
  `override_claim_image` (one generic pool serves any task image).
- Async API, quota, dataset batch submission, Python SDK for RL loops: see the project's
  `docs/tutorials` (start with "submit a task", then "trial data query" and the Python SDK).

## Step 5 — Use it from slime RL training

Point slime's remote-agent rollout at the server (no E2B key needed) — see
[run an RL task](../../1-user-guide/2-run-an-rl-task/README.md) Step 7:

```bash
MODE=kuberl KUBE_RL=http://kube-rl.kube-rl.svc.cluster.local:8080 \
DEPLOY=colocate ... bash examples/remote_agent/run_swebench.sh
```

Every rollout then executes under the server (retries, quota, persisted trajectories), and
TensorBoard + the Viewer together give you training curves and per-trial attribution.

## Troubleshooting

| Symptom | Fix |
|---|---|
| E2B submit fails 404 | Pool missing — set `POOL=` (sandbox_set_name); E2B needs a pre-warmed pool |
| `prometheus_client` missing → `/metrics` empty | Optional; install in the image or ignore |
| Pod CrashLoop after values change | ConfigMap changes roll the pod automatically; secret-only changes need `kubectl -n kube-rl rollout restart deploy/kube-rl` |

## Notes

- Best combined with a **recoverable environment** backend + persistent results volume: a restart
  re-attaches in-flight trials instead of leaking sandbox pods.
- An **OSS-direct** results backend lets agents write outputs straight to a shared volume, skipping
  the end-of-rollout artifact download through the API server.
