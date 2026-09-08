# Run an RL Task

This guide runs an agentic RL training job on ACK, **step by step**, using
[slime](https://github.com/alibaba/slime) (Megatron train + SGLang rollout). A verl-based
alternative is in [Option B](#option-b--verl-verl--vllm) at the end.

What you will build:

```
prompts.jsonl ──► slime GRPO (train_remote_agent.py, in the Ray head pod)
                     │  rollout
                     ▼
                  harbor Trial ──► sandbox pod (task image from ACR)
                     │                    ▲
                     └── in-process OpenAI adapter ── SGLang engine (same GPUs)
                          captures per-token data for training
```

**Prerequisites**

- An ACK cluster with a GPU nodepool, `kubectl` configured.
- [1-prepare-dataset](../1-prepare-dataset/README.md) completed (ACR namespace + pull secret + task images + prompts).
- Docker on your local machine (to build the workspace image).
- Aliyun CLI (`aliyun`) configured (NAS creation and the KubeRay component install below).

## Step 1 — Create shared storage (NAS) [cloud]

The head pod needs persistent storage for the model checkpoints and the task dataset.

```bash
# 1.1 Create an NFS NAS file system in your cluster's VPC (substitute region/zone/vpc/vswitch)
aliyun nas CreateFileSystem \
  --RegionId cn-hangzhou --ZoneId cn-hangzhou-i \
  --FileSystemType standard --StorageType Performance \
  --ProtocolType NFS --VpcId <vpc-id> --VSwitchId <vswitch-id>

# 1.2 Create a mount target in the same VSwitch
aliyun nas CreateMountTarget \
  --FileSystemId <fs-id> --NetworkType Vpc --VSwitchId <vswitch-id> \
  --AccessGroupName DEFAULT_VPC_GROUP_NAME

# 1.3 Wait until the mount target is Active, then read its domain
aliyun nas DescribeMountTargets --FileSystemId <fs-id>

# 1.4 Create the PV/PVC in the cluster (files ship with this guide)
sed "s|<NAS_MOUNT_DOMAIN>|<mount-target-domain>|" nas-pv-pvc.yaml | kubectl apply -f -
kubectl get pvc rl-data        # expect: Bound
```

> **Why NAS?** The Ray head pod's container disk is ephemeral — a pod restart or reschedule would
> lose the multi-GB model checkpoint and task dataset (Step 5) and force re-preparing them. A NAS
> volume keeps that data across restarts, and its **ReadWriteMany** access mode lets worker pods
> (`DEPLOY=disagg` / multi-node training) read the same checkpoint and dataset. Any existing RWX
> PVC (NAS/CPFS) can substitute. For a quick single-node smoke run you may skip NAS and use the
> pod's local disk — at the cost of re-downloading/converting after each restart.

## Step 2 — Install the KubeRay operator (ACK component) [cloud]

KubeRay is available as a managed ACK component, installed with the aliyun CLI:

```bash
# 2.1 Find the version available for your cluster
CLUSTER_ID=<cluster-id>
VERSION=$(aliyun cs DescribeClusterAddonsVersion --ClusterId $CLUSTER_ID \
  | jq -r '."kuberay-operator".next_version')     # e.g. v1.7.0-release.3

# 2.2 Install (takes a few minutes)
aliyun cs InstallClusterAddons --ClusterId $CLUSTER_ID \
  --body "{\"name\": \"kuberay-operator\", \"version\": \"$VERSION\"}"

# 2.3 Verify — the operator deployment is Running
kubectl get deploy -A | grep -i kuberay
```

## Step 3 — Build and push the slime workspace image

The workspace image bundles Megatron-LM + slime + harbor (it is the training environment).

```bash
git clone https://github.com/alibaba/slime.git && cd slime
git checkout release-slime-dev-<sha>        # the branch carrying remote-agent RL (see the repo's branches)

docker build -f docker/Dockerfile.workspace -t <acr-registry>/slime-workspace:<tag> .
docker push <acr-registry>/slime-workspace:<tag>
```

(`<acr-registry>` is the same registry you logged into during
[1-prepare-dataset](../1-prepare-dataset/README.md) Step 1.)

## Step 4 — Create the RayCluster

```bash
sed -e "s|<IMAGE>|<acr-registry>/slime-workspace:<tag>|" \
    -e "s|<PULL_SECRET>|<acr-pull-secret>|" raycluster.yaml | kubectl apply -f -

kubectl wait --for=condition=Ready pod -n default \
  -l ray.io/node-type=head --timeout=600s
POD=$(kubectl get pod -n default -l ray.io/node-type=head \
  -o jsonpath='{.items[0].metadata.name}')
echo "head pod: $POD"
```

## Step 5 — Prepare the model and dataset (inside the head pod)

```bash
# 5.1 Download an HF checkpoint to local disk (0.5B: cheap smoke; local disk, not NAS)
kubectl exec -n default $POD -- bash -c '
  pip install -q "huggingface_hub[cli]" 2>/dev/null
  hf download Qwen/Qwen2.5-0.5B-Instruct --local-dir /root/Qwen2.5-0.5B-Instruct'

# 5.2 Convert it to a Megatron torch_dist checkpoint (for --ref-load) onto the NAS
kubectl exec -n default $POD -- bash -c '
  cd /root/slime && source scripts/models/qwen2.5-0.5B.sh &&
  PYTHONPATH=/root/Megatron-LM python tools/convert_hf_to_torch_dist.py "${MODEL_ARGS[@]}" \
    --hf-checkpoint /root/Qwen2.5-0.5B-Instruct \
    --save /var/model/Qwen2.5-0.5B_torch_dist'

# 5.3 Download the task dataset (SWE-bench Verified) onto the NAS
kubectl exec -n default $POD -- bash -c \
  'harbor datasets download swe-bench-verified --output-dir /var/model-dataset'
kubectl exec -n default $POD -- ls /var/model-dataset/swe-bench-verified | head
```

## Step 6 — Generate the prompt manifest

Use the bundled generator to build `prompts.jsonl` from the dataset directory (one line per task;
`metadata.instance_id` is set to each task directory name automatically — it must match, since it
fills the task-path template `/var/model-dataset/swe-bench-verified/{instance_id}`):

```bash
# run inside the head pod (recommended — writes straight to slime's expected path)
POD=$POD bash gen-prompts.sh /var/model-dataset/swe-bench-verified -i

# or generate locally and copy in
bash gen-prompts.sh ./data/swe-bench-verified -o prompts.jsonl -n 3
kubectl cp prompts.jsonl default/$POD:/root/slime/examples/remote_agent/prompts.jsonl
```

Options: `-n <max>` (task count, `0` = all, default 3), `-o <file>`, `--prompt "<instruction>"`
(custom instruction text; default is the SWE fix instruction). A hand-written sample is in
[prompts.jsonl](prompts.jsonl) for reference.

## Step 7 — Launch RL training

All runs go through the unified launcher `examples/remote_agent/run_swebench.sh` with three
orthogonal switches: `MODE` (sandbox submission), `DEPLOY` (GPU layout), and model/preset vars.
Launch detached so a dropped exec doesn't kill the job:

```bash
kubectl exec -n default $POD -- bash -c \
  'cd /root/slime && setsid bash -c "MODE=kuberl DEPLOY=colocate \
     HF_CKPT=/root/Qwen2.5-0.5B-Instruct \
     REF_LOAD=/var/model/Qwen2.5-0.5B_torch_dist \
     MODEL_NAME=openai/Qwen2.5-0.5B-Instruct \
     GPUS=2 TP=2 GLOBAL_BATCH_SIZE=2 NUM_ROLLOUT=1 \
     bash examples/remote_agent/run_swebench.sh" > /root/run.log 2>&1 & echo launched'
```

- `MODE=kuberl` submits each trial to a **rollout server** (`KUBE_RL`, default
  `http://kube-rl.kube-rl.svc.cluster.local:8080`) — install it via
  [Manage Trajectories with RolloutServer](../../2-advanced/3-rolloutserver-trajectory-management/README.md)
  first, or set `KUBE_RL=http://<host>:<port>` to point at an existing one.
- `MODE=local` runs trials **in-process** (handy for debugging; requires the in-cluster sandbox
  stack env: `E2B_API_KEY`, optionally `E2B_API_URL`/`E2B_SANDBOX_URL`).
- `DEPLOY=colocate` shares GPUs between train and inference (add `ROLLOUT_GPUS=<n>` for
  `DEPLOY=disagg` dedicated rollout GPUs).
- Scale to a larger model with `MODEL_PRESET=<preset>` + `TP`/`PP` (27B also needs the pinned
  `mbridge` installed — see the slime runbook).

## Step 8 — Monitor

```bash
kubectl exec -n default $POD -- bash -c '
  L=/root/run.log
  echo "running: $(pgrep -cf train_remote_agent.py)"
  echo "OOM: $(grep -c "CUDA out of memory" $L)"
  echo "rollout steps: $(grep -c "perf/rollout_time" $L)"
  grep -o "rollout/rewards.: [0-9.]*" $L | tail -3'
```

A reliable "one more step finished" signal is the count of `perf/rollout_time` lines (each
rollout = one optimizer step). Do **not** use `train/step` — it logs `0` per rollout.

## Step 9 — View results

```bash
# TensorBoard (rollout perf under perf/*, quality under rollout/*)
kubectl exec -n default $POD -- bash -c \
  'nohup tensorboard --logdir /root/slime/tensorboard_log --port 6006 >/dev/null 2>&1 &'
kubectl port-forward -n default $POD 6006:6006
# open http://localhost:6006

# per-trajectory traces (trajectory.json, per-step logs)
kubectl exec -n default $POD -- ls /root/slime/trials | tail
```

## Troubleshooting

| Symptom | Fix |
|---|---|
| Sandbox pull fails `insufficient_scope: authorization failed` | Wrong pull secret — see [1-prepare-dataset](../1-prepare-dataset/README.md) Step 4 |
| `Agent name ... is not valid` | Use the exact registered name (e.g. `terminus-2`, hyphenated) via `HARBOR_AGENT_NAME` |
| All rewards equal (e.g. all 0), `pg_loss=0` | GRPO needs reward **variance** within a group — keep `N_SAMPLES>=2`, mix easy/hard tasks |
| CUDA OOM at long context | Lower `--max-tokens-per-gpu` or raise `TP` (logits spike `tokens × vocab × 2 / TP` is the usual driver) |

---

## Option B — verl (verl + vLLM)

The same idea on the verl trainer via [alibaba/verl-recipe](https://github.com/alibaba/verl-recipe)'s
**agentic** recipe: an OpenAI-compatible proxy server bridges a remote agent's requests to verl's
vLLM rollout and returns trajectories for GRPO/PPO. Run it on any GPU host with the repos checked
out (or the Ray head):

```bash
# B.1 Install verl + recipe (recipe is verl's recipe/ submodule)
git clone https://github.com/verl-project/verl.git && cd verl
git submodule update --init --recursive recipe   # ...or clone alibaba/verl-recipe directly
uv venv --python 3.12 && source .venv/bin/activate
uv pip install -e .
uv pip install -e ./harbor                      # or the alibaba/harbor fork

# B.2 Fetch the dataset (same harbor tasks as Option A)
harbor datasets download swe-bench-verified --output-dir ./data

# B.3 Launch (GRPO; config via recipe/agentic/config/agentic_trainer.yaml,
#     agent loop via recipe/agentic/remote-agent.yaml)
MODEL_PATH=/var/model/Qwen2.5-7B-Instruct \
REMOTE_AGENT_USE_LOCAL_TRIAL=true \
REMOTE_AGENT_TASK_PATH_TEMPLATE=$PWD/data/swe-bench-verified/'{instance_id}' \
  bash recipe/agentic/train_agentic_hybrid_vllm_sweagent.sh
```

Key knobs (yaml < shell env < CLI): `LLM_PROXY_IP` (proxy reachable from the agent),
`REMOTE_AGENT_NAME`, `REMOTE_AGENT_USE_LOCAL_TRIAL`, `PROXY_SERVER_URL` (set → standalone proxy
mode). Full reference: the recipe's `agentic/README.md`.

> `agentic` is the production bundle (standalone proxy, disagg, ACK/harbor integration); the
> sibling `remote_agent` recipe is the framework-agnostic upstream-facing version.

### Running verl on Kubernetes

For on-cluster runs the recipe ships K8s manifests (`agentic/k8s/`) and a dedicated K8s config
(`agentic/config/agentic_trainer_k8s.yaml`). The flow (from the recipe's ACK runbook):

```bash
# B.4 Deploy the standalone LLM proxy as its own Deployment+Service — the sandbox
#     pods must reach it by the stable Service name; deploy BEFORE the RayCluster
kubectl apply -f recipe/agentic/k8s/llm-proxy-server.yaml

# B.5 Create the RayCluster (head + training/rollout worker groups; verl image)
kubectl apply -f recipe/agentic/k8s/ray-cluster-<env>.yaml
POD=$(kubectl get pod -l app=verl-disagg-mooncake,role=head \
  -o jsonpath='{.items[0].metadata.name}')

# B.6 Launch training in the head pod, with the k8s config
#     (remote_agent.environment_kwargs points at ACKEnvironment + buildkit;
#      remote_agent.proxy_server_url: http://llm-proxy-server)
kubectl exec $POD -c ray-head -- bash -c '
  MODEL_PATH=/var/model/Qwen2.5-7B-Instruct \
  nohup python3 -m recipe.agentic.agentic_main \
    --config-name agentic_trainer_k8s \
    actor_rollout_ref.model.path=$MODEL_PATH \
    algorithm.adv_estimator=grpo \
    trainer.n_gpus_per_node=8 trainer.nnodes=1 \
    > /tmp/train.log 2>&1 &'
kubectl exec $POD -c ray-head -- tail -f /tmp/train.log
```

Notes from the ACK runbook that matter here:
- **Standalone proxy is the K8s default**: `proxy_server_url: http://llm-proxy-server` in the k8s
  config; sandbox pods reach it via the Service name, so `LLM_PROXY_IP` no longer has to be a
  pod IP.
- **Environment**: `remote_agent.environment_kwargs` is pre-wired to
  `harbor.environments.ack:ACKEnvironment` (namespace / `image_pull_secret` / in-cluster BuildKit
  via `buildkit_address: buildkit-service:1234`).
- **Node tuning**: GPU taints tolerations, large `memory`/`shm` (NCCL + model loading),
  `RAY_memory_usage_threshold=0.99`, RDMA resource requests where available — copy them from the
  recipe's RayCluster yaml.
- Dataset/model PVCs mount into the RayCluster (e.g. `/var/model`, `/var/model-dataset`), same as
  the slime path in Steps 4–5.
