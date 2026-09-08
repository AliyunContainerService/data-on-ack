# Related Repositories

The RL examples in this guide build on three open-source repositories. **Each is an Alibaba
"contribution fork"** — per its own description, *"Fork for contributing. All changes intended for
upstream PRs."* On all three, the fork's **`main` stays in sync with upstream**; Alibaba's work lives
on **`feat/*` and `release-*` branches** and is being upstreamed via PRs. So the "what's added" below
is a **snapshot** of those branches (verify with the compare command under each section).

| Repo | Role in the RL stack | Upstream (fork parent) |
|------|----------------------|------------------------|
| [alibaba/slime](https://github.com/alibaba/slime) | RL post-training framework (Megatron train + SGLang rollout) | [THUDM/slime](https://github.com/THUDM/slime) |
| [alibaba/harbor](https://github.com/alibaba/harbor) | Agent evaluation / RL-environment runner (rollout generation) | [harbor-framework/harbor](https://github.com/harbor-framework/harbor) |
| [alibaba/verl-recipe](https://github.com/alibaba/verl-recipe) | End-to-end RL training recipes (verl-based, alternative path) | [verl-project/verl-recipe](https://github.com/verl-project/verl-recipe) |

The examples in [2-run-an-rl-task](../2-run-an-rl-task/README.md) use the **slime + harbor** path.
`verl-recipe` is the verl-based alternative and is included here for completeness.

---

## alibaba/slime

**What it is.** slime is an LLM post-training framework for RL scaling: high-performance training
(Megatron) wired directly to flexible rollout/data-generation (SGLang), with training, rollout,
reward/verifier and environment interaction all flowing through one train / rollout / Data Buffer
path. It passes Megatron args through directly and exposes SGLang args with a `--sglang-` prefix.

**What Alibaba adds (fork branches, upstreaming in progress).** The theme is **agentic remote-agent
RL that runs on ACK sandboxes via harbor**:

- **Remote-agent RL on ACK sandboxes, end-to-end** — run slime RL agents inside harbor-managed
  sandboxes (over E2B / SandboxSet) driven from the training loop.
- **Harbor integration** — import the harbor environment; pass the `SandboxSet` name through to it;
  make the remote `HarborClient` submission match the Rollout Server `AgentRunRequest`.
- **In-process `OpenAIAdapter`** — replaces the earlier out-of-process TokenProxy; serves generation
  from the colocated SGLang engine and captures per-token `(token_id, logprob)` for training, with
  **concurrent OpenAI sessions isolated** per trial.
- **Standalone proxy mode** with engine self-registration (decouple the rollout entry from a single
  engine).
- **`train_remote_agent` as a thin wrapper** over `train.train` (the remote-agent entrypoint).
- **Robustness fixes** — honor rollout timeout and retry SGLang 5xx; sanitize local trial names;
  plus a consolidated remote-agent runbook / example launcher.

> Verify: `git log THUDM/slime/main..alibaba/slime/release-slime-dev-<sha>` (the fork's `main`
> matches upstream; the work is on the `feat/*` and `release-slime-dev-*` branches).

## alibaba/harbor

**What it is.** Harbor (from the creators of [Terminal-Bench](https://www.tbench.ai)) is a framework
for evaluating/optimizing agents and for building and using RL **environments**. It runs arbitrary
agents across many sandbox providers in parallel and can **generate rollouts for RL optimization** —
which is how slime uses it here.

**What Alibaba adds (fork branches, upstreaming in progress).** Making harbor first-class on ACK and
wiring it to a rollout control plane:

- **`ACKEnvironment`** — a native ACK/Kubernetes environment provider (`feat/add-ack-environment`):
  one sandbox pod per trial, alongside upstream providers like E2B / Modal / Daytona.
- **ACK runtime exec** — persistent in-sandbox exec on ACK (`feat/ack-runtime-exec`).
- **Rollout Server submission** — submit harbor run jobs to a RolloutServer
  (see [../../2-advanced/3-rolloutserver-trajectory-management](../../2-advanced/3-rolloutserver-trajectory-management/README.md)).
- **Image cache + image version check** — pre-pull / validate task images to cut sandbox cold start
  (`feat/image-cache`, `feat/add-image-version-check`; see
  [../../2-advanced/1-image-cache](../../2-advanced/1-image-cache/README.md)).
- **Multi-tenant credentials** — E2B credentials as constructor kwargs, and key the Kubernetes client
  manager by credentials; resolve image-build registry auth by host.
- **Trial recovery (RFC)** — attach and resume in-flight trials after a restart instead of re-running.

> Verify: <https://github.com/harbor-framework/harbor/compare/main...alibaba:release-dev-e5e46809-rollout-server>
> (branch names carry a base-commit suffix and rotate).

## alibaba/verl-recipe

**What it is.** `verl-recipe` hosts end-to-end RL **recipes** based on
[verl](https://github.com/verl-project/verl); it is used as verl's `recipe/` submodule, and each
recipe pins its required verl version (`REQUIRED_VERL.txt` + `install_verl.sh`) for reproducibility.

**What Alibaba adds (fork branches, upstreaming in progress).** A **remote-agent (agentic) RL recipe**
mirroring the slime path, on the verl trainer:

- **`remote_agent` RL recipe** with `external_sglang` consolidation.
- **`remote_megatron_sglang` V1 trainer port** — adapted to the upstream V1 `TaskRunner` API,
  e2e-validated on both internal and upstream verl main.
- **E2B sandbox + harbor dataset** integration — Dockerfile deps, Hydra-driven config, local harbor
  dataset with auto `task_path`.
- **Standalone proxy mode** and a **Kubernetes deployment example**.

> Verify: <https://github.com/verl-project/verl-recipe/compare/main...alibaba:release-remote-agent-rl>
> and `.../compare/main...alibaba:feat/add-agentic-training-example`.

---

## How they fit together (slime path)

```
prompts.jsonl ──► slime (train_remote_agent.py, GRPO)
                     │  rollout
                     ▼
                  harbor Trial ──► ACKEnvironment ──► sandbox pod (task image)
                     │                                     ▲
                     └── in-process OpenAIAdapter ─────────┘  (SGLang generation,
                                                                per-token capture for training)
```

`verl-recipe` is the same idea on the **verl** trainer (its `remote_agent` recipe), an alternative to
the slime path above.
