# Prepare a Dataset for Agentic RL

Step-by-step guide to prepare the two inputs every agentic RL run needs:

1. **Task sandbox images** in ACR — one image per task (the container the agent works in).
2. **A prompt manifest** (`prompts.jsonl`) — one line per task, mapping RL samples to task instances.

**Prerequisites**

- An ACK cluster, `kubectl` configured.
- Docker on your local machine (builds/pushes task images).
- Aliyun CLI (`aliyun`) configured with a RAM identity that can manage CR (Container Registry).
- An **ACR Enterprise Edition** instance (create one in the console if you don't have it — instance
  creation is not exposed via CLI). This guide uses an existing instance.

## Step 1 — Set up the ACR namespace [cloud]

```bash
# 1.1 List your ACR EE instances; note the instance id and its domain
aliyun cr ListInstance
#   e.g. instance "cri-xxx" with domain "registry.cn-hangzhou.cr.aliyuncs.com"

# 1.2 Create a namespace for RL task images
aliyun cr CreateNamespace --InstanceId cri-xxx \
  --NamespaceName terminal-bench --AutoCreateRepo false

# 1.3 Create the repository
aliyun cr CreateRepository --InstanceId cri-xxx \
  --RepoNamespaceName terminal-bench --RepoName tasks \
  --RepoType PRIVATE --Summary "RL task sandbox images"

# 1.4 Get a login token and docker login (temp token, valid ~1h)
TOKEN_JSON=$(aliyun cr GetAuthorizationToken --InstanceId cri-xxx)
docker login registry.cn-hangzhou.cr.aliyuncs.com \
  --username "$(echo "$TOKEN_JSON" | jq -r .data.tempUserName)" \
  --password-stdin <<<"$(echo "$TOKEN_JSON" | jq -r .data.authorizationToken)"
```

## Step 2 — Install the harbor CLI and download the task dataset

```bash
# 2.1 harbor CLI (any machine with Python 3.10+)
pip install harbor            # or: uv tool install harbor

# 2.2 Download the dataset (here: SWE-bench Verified) — creates one dir per task
harbor datasets download swe-bench-verified --output-dir ./data
ls data/swe-bench-verified | head
#   astropy__astropy-12907  astropy__astropy-14309  django__django-16379 ...
```

Each task directory holds `task.toml` (instruction/tests/image reference), `instruction.md`,
`tests/`, and optionally `environment/Dockerfile`.

## Step 3 — Build and push the sandbox images

harbor builds each task's image and pushes it under your ACR repo (local Docker does the build):

```bash
harbor admin upload-images \
  --tasks-dir ./data/swe-bench-verified \
  --registry registry.cn-hangzhou.cr.aliyuncs.com/terminal-bench/tasks \
  --sanitize-image-names \   # ACR rejects '__' etc.; auto-converted to '-'
  --update-config \          # writes docker_image / image_sha256 back into each task.toml
  --skip-unchanged \         # re-runnable; skips already-pushed tasks
  -n 4                       # parallel builders
```

Key options (the ones that matter on ACK):

| Option | What it does | Why on ACK |
|---|---|---|
| `--registry <url>` | Registry prefix used when a task's `task.toml` has no `docker_image` | Point it at your ACR repo; the synthesized tag defaults to `YYYYMMDD` |
| `--remote-buildkit tcp://<host>:1234` | Build via a remote BuildKit endpoint (buildx remote driver, falls back to `buildctl --addr`) | Build inside the cluster — no local Docker daemon needed, and base-image pulls don't traverse your laptop |
| `--sanitize-image-names` | Rewrites registry-illegal task names (`django__django-14349` → `django-django-14349`) | ACR rejects `__` in repository paths; without this the push fails at the registry |
| `--update-config` / `--override-config` | Writes `docker_image`, `built_content_hash`, `image_sha256` back into each `task.toml` (override = replace existing values) | The RL run reads the image from `task.toml` — without this the task keeps pointing at the old/missing image |
| `--skip-unchanged` | Skips build+push when the recorded content hash and remote digest already match | Re-runnable pipeline: a resumed/interrupted bulk upload only rebuilds what changed |
| `--diff-only` | Compares local hashes vs the remote registry only; no builds; exit 1 on drift | CI-style drift check that the pushed images match the task sources |
| `-n <N>` | Parallel builds/pushes | ACR + remote BuildKit handle several at once; 4 is a good start |
| `--tag <tag>` / `--tag-latest` | Overrides/replaces the tag; also tag `latest` | Pin a stable tag per dataset release rather than relying on the date default |
| `--filter <substr>` | Subset of tasks by name | Rebuild a single failing task without touching the rest |

Verify one image:

```bash
docker manifest inspect registry.cn-hangzhou.cr.aliyuncs.com/terminal-bench/tasks/astropy-astropy-14309:latest
```

## Step 4 — Create the image pull secret in the cluster

The pull secret must be a **long-lived** credential (the Step-1 token expires in ~1h). Use a fixed
access credential for the instance (set in the ACR console), or create a dedicated RAM user via CLI:

```bash
# (optional) dedicated read-only RAM user
aliyun ram CreateUser --UserName acr-rl-puller
aliyun ram CreateAccessKey --UserName acr-rl-puller       # save AK/SK
aliyun ram AttachPolicyToUser --UserName acr-rl-puller \
  --PolicyType System --PolicyName AliyunContainerRegistryReadOnlyAccess

kubectl create secret docker-registry acr-pull-secret \
  --docker-server=registry.cn-hangzhou.cr.aliyuncs.com \
  --docker-username=<username> \
  --docker-password=<password> \
  -n default
```

## Step 5 — Verify the images pull from the cluster

```bash
kubectl run tb-pull-test --restart=Never -- sleep 5 \
  --image=registry.cn-hangzhou.cr.aliyuncs.com/terminal-bench/tasks/astropy-astropy-14309:latest \
  --overrides='{"spec":{"imagePullSecrets":[{"name":"acr-pull-secret"}]}}'
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded pod/tb-pull-test --timeout=300s
kubectl delete pod tb-pull-test
```

> `insufficient_scope: authorization failed` here means the secret lacks scope for the repo
> (wrong credential / wrong instance) — fix Step 4 before continuing.

## Step 6 — Generate the prompt manifest

Generate `prompts.jsonl` automatically from the dataset directory with the script shipped in
[2-run-an-rl-task](../2-run-an-rl-task/gen-prompts.sh) (only directories containing `task.toml`
are picked up; `metadata.instance_id` is set to each task directory name automatically):

```bash
bash ../2-run-an-rl-task/gen-prompts.sh ./data/swe-bench-verified -o prompts.jsonl -n 3
```

Options: `-n <max>` (`0` = all tasks), `-o <file>`, `--prompt "<instruction>"` (default is the SWE
fix instruction). A hand-written sample is in
[2-run-an-rl-task/prompts.jsonl](../2-run-an-rl-task/prompts.jsonl).

`metadata.instance_id` **must** equal the task directory name — it fills the task-path template
(`/var/model-dataset/swe-bench-verified/{instance_id}`) at rollout time.

## Done — what you have now

- Task images in ACR (`task.toml` now references them), and
- a `prompts.jsonl` manifest,

ready for [2-run-an-rl-task](../2-run-an-rl-task/README.md). How each framework consumes them:
slime passes the manifest via `--prompt-data`; verl's agentic recipe points its `local_harbor`
dataset / `task_path_template` at the same task directories — same tasks, same images.

## Tips

- Mix easy and hard tasks: GRPO needs **reward variance within a prompt group**; a dataset the base
  model scores 0 on everywhere yields no learning signal.
- Keep the task dataset on the shared volume together with the manifest (the run-task guide mounts
  it at `/var/model-dataset`).
