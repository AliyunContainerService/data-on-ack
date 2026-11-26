# Serving Evaluation and RL on One RolloutServer

This guide describes how one RolloutServer serves both Evaluation and Reinforcement Learning (RL) rollouts. The core principle: configure model endpoints and credentials per request, inject provider Secrets only at server deployment time, and isolate sandboxes and capacity through queues.

## Overview

Configuration differences between the two rollout types:

| Configuration | RL rollout | Evaluation rollout |
|---|---|---|
| Model endpoint | verl's rollout proxy, usually passed dynamically each round | Provider API |
| Model credentials | The proxy's authentication info | Key injected from a Secret on the server |
| Sandbox | Shared, or separated via queue/backend | Same |
| Trajectory | Must pass through the RL proxy to collect token/logprob | Ordinary evaluation trajectory |

How it works:

- `agent.kwargs.extra_env` in a request becomes the agent environment variables of each trial, inside the worker process. This mechanism applies to all agents, not a specific one.
- `${VAR}` references in values are resolved by the server from the worker process environment when the agent is constructed. Requests carry variable references only; real keys never appear in requests and are never written to disk.
- `queue` handles sandbox routing and quota only. It does not participate in model credential selection; model selection is entirely driven by request fields.

## Backend and Queue Mechanism

Backend and Queue are RolloutServer's two core abstractions: Backend encapsulates differences in sandbox execution capability; Queue encapsulates capacity isolation for upper-layer business lines. Understand both before configuring dual mode:

- **Backend**: a configuration abstraction over different backends and sandbox types. Each backend declares one environment type (`import_path`, e.g. ACK Pod or E2B) in `config.backends[]`, together with default environment parameters (`defaults`, e.g. namespace, `sandbox_set_name`) and infrastructure credentials (`env_profile`). Sandbox-type differences are confined to the backend layer; upper-layer requests need not know about them.
- **Queue**: a quota abstraction over upper-layer business lines. Each queue maps to one business lane (e.g. `rl`, `evaluation`) and constrains its concurrency, queue depth, and submission rate via `max_concurrent` / `max_queued` / `rate_limit`. Business lines isolate capacity through queues without deploying a separate server per line.

The two are linked through `queues`: a backend declares the queues it serves, and the request's `queue` field (default `default`) decides which backend executes the trial. Routing rules:

- A queue can bind to exactly one backend.
- Queues not bound to any backend fall through to the backend marked `default: true`.
- If no backend is reachable, the server refuses to start.
- Queues determine sandbox routing and capacity only, never model endpoints or credentials.

By isolation level, dual mode has two configuration patterns.

### Pattern A: shared backend, separate queues

Evaluation and RL share one sandbox backend; queue quotas isolate capacity. Suitable when both traffic types run in the same cluster:

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

The two queues count independently, so evaluation traffic cannot squeeze RL capacity, and vice versa.

### Pattern B: separate backends

Evaluation and RL use different sandbox backends (different clusters, sandbox pools, or credential sets) for full isolation:

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

`env_profile` supplies infrastructure credentials (e.g. the E2B key) to the backend; values reference Pod environment variables via `${VAR}`, so Secrets never enter the config file. Model keys must not go here (see "Migrate an Existing Deployment").

In both patterns, requests select their purpose through the `queue` field: `"queue": "rl"` or `"queue": "evaluation"`. Model endpoints and credentials are still configured per request via `extra_env`, independent of the queue.

## Inject the Provider Secret

Inject the provider key under a dedicated variable name so it does not become the default for all requests:

1. Create a Secret containing the provider key (example name `hbo-eval-model`, key `api-key`).
2. Reference it from Helm values via `extraEnv`:

```yaml
extraEnv:
  - name: EVAL_PROVIDER_API_KEY
    valueFrom:
      secretKeyRef:
        name: hbo-eval-model
        key: api-key
```

Notes:

- The chart renders `extraEnv` as native EnvVars, so `valueFrom.secretKeyRef` works. It is injected into all roles: single-mode server, multi-mode ingress/worker, and viewer. batchController uses its own key, `batchController.extraEnv`. There is no `extraEnvFrom` value.
- Why a dedicated name: agent credential resolution falls back per key, "request `extra_env` first, server process `os.environ` second". If `OPENAI_API_KEY` / `OPENAI_BASE_URL` sit in the server's global environment, requests without explicit configuration (including RL requests) inherit the provider defaults — evaluation configuration leaks into RL trajectories. With a dedicated name like `EVAL_PROVIDER_API_KEY`, only evaluation requests that explicitly reference it use the key.
- This does not dump the whole server environment into the sandbox: sandbox exec receives only the explicitly merged env. Inheritance is the effect of "the agent reads its candidate variable names with per-key fallback".

## Submit an Evaluation Rollout

Specify the provider endpoint via `agent.kwargs.extra_env` and reference the deployment-time key with `${VAR}`:

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

Requests carry variable references only, never real keys.

Exact semantics of `extra_env` and `${VAR}`:

- Both dict and K8s list (`[{"name": K, "value": V}]`) formats are supported. The server assembles them into the agent environment and resolves variable references when constructing the agent.
- `${VAR}` resolves from the worker process's `os.environ`. An undefined variable raises `ValueError` and the trial fails — it never silently uses an empty value.
- The `${VAR:-default}` default-value syntax is supported.
- Resolution applies only when the entire value is a template (fullmatch): `"${EVAL_PROVIDER_API_KEY}"` resolves; a reference embedded in a longer string, like `"Bearer ${TOKEN}"`, passes through literally.
- The resolved env is injected into the agent's setup/run commands through a per-task isolated exec overlay; verifier/build commands are not affected.
- `model_name` uses the `provider/model` format. `openai/<model-id>` denotes an OpenAI-compatible endpoint, used with `OPENAI_BASE_URL` / `OPENAI_API_KEY`.

## Submit an RL Rollout

Keep the recipe's current proxy integration: pass `llm_proxy_url` dynamically and set the endpoint and authentication explicitly in `extra_env`. Example for an unauthenticated proxy:

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

Other RL session and sampling parameters continue to be filled in by the recipe.

**Set `OPENAI_BASE_URL` explicitly; do not rely on `llm_proxy_url` to rewrite the endpoint.** `llm_proxy_url` does not rewrite the agent's model endpoint; setting only that field makes built-in agents (mini-swe-agent, claude-code, etc.) fail with `Unknown option 'model_base_url'`. Two reliable options:

1. Point `extra_env.OPENAI_BASE_URL` at the proxy (the example above, recommended).
2. Pass `agent.kwargs.base_url` explicitly.

Still pass `llm_proxy_url` as the recipe specifies (the SDK/slime hook forwards it); it identifies which proxy a rollout belongs to.

Token/logprob collection: `rollout_details` (token_ids/logprobs) is produced by the server's LLM call layer. Currently only some agents (e.g. `terminus-2` with `collect_rollout_details` enabled) populate it; CLI-style agents such as mini-swe-agent and claude-code do not. When RL needs token-level data, choose an agent that supports collection, or record it on the verl proxy side. The constraint "trajectories must pass through the RL proxy" is unchanged.

## Migrate an Existing Deployment

If the current deployment puts the provider's `OPENAI_API_KEY` and `OPENAI_BASE_URL` in the server's global environment (fine for pure evaluation), migrate to dual mode as follows:

1. Remove `OPENAI_API_KEY` / `OPENAI_BASE_URL` from the server's global environment; replace them with `EVAL_PROVIDER_API_KEY` (Secret `secretKeyRef` injection, see "Inject the Provider Secret").
2. Evaluation requests carry `extra_env` explicitly: endpoint + `${EVAL_PROVIDER_API_KEY}` reference.
3. RL requests carry `extra_env` explicitly pointing at the verl proxy; do not rely on `llm_proxy_url` to rewrite the endpoint.
4. Verify `config.env_profiles` contains no model credentials.
5. Configure separate quotas for `rl` / `evaluation` as needed.

About `env_profiles`: it holds infrastructure credentials for sandbox backends (e.g. `E2B_API_KEY`, `KUBECONFIG`), never model keys. The server validates at startup and rejects model-related variables: prefixes `ANTHROPIC_` / `OPENAI_` / `GEMINI_` / `LLM_`, plus the exact match `GOOGLE_API_KEY` (case-insensitive; `GOOGLE_APPLICATION_CREDENTIALS` is allowed for GKE). The error message points you to `agent.kwargs.extra_env`.

`env_profiles` `${VAR}` and request-level `extra_env` are two different resolution semantics: the former resolves once at process startup, fails startup on missing variables, and does not support `:-` defaults; the latter resolves per trial at agent construction and supports defaults.

## Verify Dual-Mode Operation

Before going live with dual mode, run this minimal joint-verification matrix:

1. Submit one evaluation rollout (`queue=evaluation`, provider endpoint). Confirm completed status, a normal reward, and no proxy address in the trajectory.
2. Submit one RL rollout (`queue=rl`, proxy endpoint). Confirm completed status and matching traffic visible on the verl proxy side.
3. Run both types concurrently (at least 50 each). Confirm no endpoint cross-over: evaluation trajectories contain no proxy address, RL trajectories contain no provider address; the two queues count quota independently.
4. Submit one request without `extra_env`. Confirm it no longer inherits provider defaults: after the migration it should fail for missing credentials or use the agent's own defaults, not hit the provider.
5. Reference a variable that was never injected (e.g. `${NOT_INJECTED}`). Confirm the trial fails with `ValueError` (fail-fast).