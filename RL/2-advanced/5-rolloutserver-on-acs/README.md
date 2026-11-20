# Run Rollout Server and Sandboxes on ACS

Run Rollout Server and its trial sandboxes on Alibaba Cloud Container Compute Service (ACS), in three
scenarios: Rollout Server deployed in an ACS cluster; an ACK cluster borrowing ACS compute for
sandboxes; and sandboxes served from ACS Agent Sandbox warm pools. All three are configured through
Helm values only; the chart is not modified.

**Why.** The number of trial sandboxes rises and falls with rollout concurrency, and each sandbox
lives for a single trial. Per-pod billing and second-level elasticity make ACS a better fit for this
load than a standing node pool, and Agent Sandbox warm pools also remove the wait for pod creation
and image pulls. ACS pods have their own constraints, though (spec rounding, no privileged
containers, storage and networking limits), so the default values cannot be used as-is.

For the basic deployment and trajectory persistence, see
[Manage trajectories with RolloutServer](../3-rolloutserver-trajectory-management/README.md).

## How it works

Rollout Server has two kinds of pods, scheduled independently:

```text
Helm values
  ├─ top-level nodeSelector / tolerations / resources ──→ server pods (ingress + worker + Redis sidecar)
  └─ config.*                                         ──→ trial sandboxes created by the server
        ├─ pod_overrides             → deep-merged into the sandbox pod (or the SandboxSet pod template)
        ├─ use_sandbox_claim: true   → SandboxSet warm pool + SandboxClaim
        └─ backends (E2BEnvironment) → claimed through the ack-sandbox-manager E2B API
```

| Scenario | Server pods | Sandboxes | Environment backend | Key values |
| --- | --- | --- | --- | --- |
| 1: Deploy in an ACS cluster | ACS pods | ACS pods | `ACKEnvironment`, pod mode | `persistence.nas`, `resources`, `global.imagePrefix` |
| 2: ACK cluster borrowing ACS compute | ECS nodes | ACS pods, via the virtual node | `ACKEnvironment`, pod mode | `config.pod_overrides.metadata.labels` |
| 3: ACS Agent Sandbox | ACK or ACS cluster | Agent Sandbox | `ACKEnvironment` SandboxClaim mode, or `E2BEnvironment` | `config.use_sandbox_claim` or `config.backends` |

## Prerequisites

- `ack-rollout-server` installable from the application marketplace. The values in this guide were
  render-checked against chart 0.8.0.
- `kubectl` and Helm 3, connected to the target cluster.
- Task images pullable from the cluster. Prefer a prebuilt `docker_image` in `task.toml`: ACS cannot
  run the default privileged image-build Job (see scenario 1, step 4).

```bash
export ROLLOUT_NAMESPACE=kube-rl
export ROLLOUT_RELEASE=ack-rollout-server
export ROLLOUT_SERVICE=ack-rollout-server
helm repo add aliyunhub https://aliacs-app-catalog.oss-cn-hangzhou.aliyuncs.com/charts-incubator/
helm repo update
```

## Step 0 — Values shared by every scenario

The chart's Role grants permissions only in the release namespace, but `config.namespace` (where
sandboxes are created) defaults to `default`. If the two differ, the server cannot create sandboxes
and every trial fails. Start every values file with:

```yaml
config:
  namespace: kube-rl          # must equal $ROLLOUT_NAMESPACE
```

Each scenario below adds to this, and installs or upgrades with the same command:

```bash
helm upgrade --install "$ROLLOUT_RELEASE" aliyunhub/ack-rollout-server \
  -n "$ROLLOUT_NAMESPACE" --create-namespace -f values-acs.yaml
kubectl -n "$ROLLOUT_NAMESPACE" get pod,svc,pvc
# expect: server pod Running, PVC Bound
```

## Scenario 1 — Deploy Rollout Server in an ACS cluster

Every pod in an ACS cluster is an ACS pod, so neither the server nor the sandboxes need a
nodeSelector or tolerations. What needs adjusting is storage, resource specs, image addresses,
service exposure, and how sandboxes handle privilege and image builds.

### Step 1 — Set the hot-data storage explicitly

By default the chart requests `ReadWriteMany` for the hot-data volume with an empty
`storageClassName`. The default StorageClass in an ACS cluster is usually a cloud disk, which does
not support RWX, so the PVC stays Pending. Pick one by deployment mode:

```yaml
# single mode (default): one replica + Recreate upgrades, only one pod mounts it; a cloud disk works.
deploymentMode: single
persistence:
  enabled: true
  nas:
    storageClassName: <cloud-disk StorageClass>   # kubectl get storageclass
    accessModes: [ReadWriteOnce]
    size: 100Gi
```

```yaml
# multi mode: ingress / worker / viewer share the hot volume; it must be RWX, e.g. NAS.
deploymentMode: multi
persistence:
  enabled: true
  nas:
    storageClassName: <NAS StorageClass>
    accessModes: [ReadWriteMany]
redis:
  mode: external
  external:
    existingSecret: <redis Secret>                # key: redis-url
```

To archive finished trials to OSS, note that ACS supports OSS only as a static volume: create the
PV/PVC first, then set `persistence.archive.existingClaim`.

### Step 2 — Make resource specs land on an ACS spec

ACS sums the resources of all containers in a pod, rounds up to the nearest supported spec, bills
that spec, and gives the surplus to the first container. In single mode the server pod has two
containers, server and Redis: set requests equal to limits and make the total exactly one spec. This
example totals 2 vCPU / 4 GiB:

```yaml
resources:
  requests: {cpu: "1.5", memory: 3Gi}
  limits:   {cpu: "1.5", memory: 3Gi}
redis:
  resources:
    requests: {cpu: 500m, memory: 1Gi}
    limits:   {cpu: 500m, memory: 1Gi}
```

In multi mode, tune `multi.ingress.resources`, `multi.worker.resources` and `viewer.resources`
separately. The chart has no custom-label parameter for server pods, so they always use the default
`general-purpose` compute class.

### Step 3 — Image addresses

The marketplace console replaces `__ACK_REGION_ID__` for you; a Helm CLI install must set it. The
server image prefix and the Redis image are two independent values and must change together. ACS
pods pull images over the VPC; without a NAT gateway, use the `-vpc` domain:

```yaml
global:
  imagePrefix: registry-cn-hangzhou-vpc.ack.aliyuncs.com
redis:
  image:
    repository: registry-cn-hangzhou-vpc.ack.aliyuncs.com/acs/redis
```

With a NAT gateway you can drop `-vpc`.

### Step 4 — Sandbox pods

Sandboxes are ACS pods too. To choose a compute class, add labels through `pod_overrides`:

```yaml
config:
  namespace: kube-rl
  pod_overrides:
    metadata:
      labels:
        alibabacloud.com/compute-class: general-purpose   # or performance
        alibabacloud.com/compute-qos: default             # best-effort is cheaper but preemptible
```

A sandbox's CPU, memory and ephemeral-storage requests come from `cpus`, `memory_mb` and
`storage_mb` in `task.toml`, and are rounded up and billed the same way. Ephemeral storage above
30 GiB is billed separately.

ACS does not allow privileged containers, which matters in two places:

- Do not set `privileged: true` in `pod_overrides`; the sandbox pod will fail to start.
- When a task image is missing, the server builds it with a privileged DinD Job by default, which
  cannot start on ACS. Use a prebuilt `docker_image`, or switch to a remote BuildKit:

  ```yaml
  config:
    use_buildkit: true
    buildkit_address: tcp://<buildkitd address>:1234
  ```

### Step 5 — Expose the service

The default Service is `ClusterIP`; training jobs reach
`ack-rollout-server.kube-rl.svc.cluster.local:8080` inside the cluster. ACS does not support
NodePort, so use a LoadBalancer for access from outside the cluster, and enable
`auth.existingSecret` at the same time:

```yaml
service:
  type: LoadBalancer
auth:
  existingSecret: <auth Secret>                   # key: tokens
```

### Step 6 — Verify

```bash
kubectl -n "$ROLLOUT_NAMESPACE" get pod -l app.kubernetes.io/name=ack-rollout-server \
  -o custom-columns='NAME:.metadata.name,INSTANCE:.metadata.annotations.alibabacloud\.com/instance-id,SPEC:.metadata.annotations.alibabacloud\.com/pod-use-spec'
# expect: INSTANCE starts with acs-; SPEC is 2-4Gi

kubectl -n "$ROLLOUT_NAMESPACE" port-forward svc/"$ROLLOUT_SERVICE" 8080:8080 &
curl -s http://localhost:8080/health | python3 -m json.tool
# expect: status ok, workers non-empty
```

Then submit an `oracle` task as in step 4 of
[Manage trajectories with RolloutServer](../3-rolloutserver-trajectory-management/README.md);
a reward of 1.0 means the sandbox path works.

## Scenario 2 — ACK cluster borrowing ACS compute for sandboxes

The server stays on ECS nodes; only trial sandboxes go to ACS. Requires an ACK managed Pro cluster
(Kubernetes ≥ 1.26), ACS activated and authorized, and the ACK Virtual Node component ≥ v2.13.0.

### Step 1 — Schedule sandboxes to ACS with labels

```yaml
config:
  namespace: kube-rl
  pod_overrides:
    metadata:
      labels:
        alibabacloud.com/acs: "true"
        alibabacloud.com/compute-class: general-purpose
        alibabacloud.com/compute-qos: default
```

| Label | Required | Description |
| --- | --- | --- |
| `alibabacloud.com/acs` | Yes | `"true"`: the scheduler hands the sandbox pod to ACS. |
| `alibabacloud.com/compute-class` | No (default `general-purpose`) | Compute class, e.g. `performance`. |
| `alibabacloud.com/compute-qos` | No (default `default`) | `best-effort` is cheaper but preemptible; use it for retryable rollouts. |

`pod_overrides` is merged into sandbox pods only, not into image-build Jobs, so builds still run on
ECS nodes.

Do not use `config.node_selector: {type: virtual-kubelet}` plus `config.tolerations` instead:

- The server applies `node_selector` and `tolerations` to the privileged DinD build Job as well,
  which then lands on the virtual node and cannot start.
- A pod that lands on the virtual node without declaring an ACS compute class runs on ECI, not ACS.

Use the nodeSelector method only when sandboxes mount cloud disks from a `WaitForFirstConsumer`
StorageClass, which the `alibabacloud.com/acs` label does not support; in that case you must also
set the `compute-class` label.

### Step 2 — Server scheduling (optional)

Server pods run on ECS nodes by default. If the nodes are tainted, use the top-level values; they do
not affect sandboxes:

```yaml
tolerations:
  - key: CriticalAddonsOnly
    operator: Equal
    value: "true"
    effect: NoSchedule
```

### Step 3 — Verify

Submit a task and inspect the sandbox pod while the trial runs:

```bash
kubectl -n "$ROLLOUT_NAMESPACE" get pod -l app=sandbox \
  -o custom-columns='NAME:.metadata.name,NODE:.spec.nodeName,INSTANCE:.metadata.annotations.alibabacloud\.com/instance-id'
# expect: NODE starts with virtual-kubelet-, INSTANCE starts with acs-
```

## Scenario 3 — Run sandboxes on ACS Agent Sandbox

Agent Sandbox keeps a warm pool in a SandboxSet; trials claim sandboxes that are already ready.
Cluster requirements:

- ACS cluster: Agent Sandbox activated, ACS Virtual Node ≥ v2.17.0.
- ACK cluster: ACK Virtual Node ≥ v2.13.0.
- Both need ack-agent-sandbox-controller ≥ v0.5.14-release.1; the E2B path also needs
  ack-sandbox-manager ≥ v0.6.0.

Rollout Server can use it in two ways:

| Path | Environment backend | Who creates the warm pool | Use when |
| --- | --- | --- | --- |
| SandboxClaim | `ACKEnvironment` + `use_sandbox_claim: true` | The server, per task environment | Many task environments; you want pools created automatically |
| E2B API | `E2BEnvironment` | You, in advance | You already have a shared pool, or reuse the E2B ecosystem |

### Step 1a — SandboxClaim path

For each task environment the server looks up a SandboxSet of the same name, creates it if missing,
then claims a sandbox with a SandboxClaim and runs the agent in it. Agent Sandbox requires three
labels on the pod template; write them into the template through `pod_overrides`:

```yaml
config:
  namespace: kube-rl
  use_sandbox_claim: true
  claim_timeout: 300
  pod_overrides:
    metadata:
      labels:
        alibabacloud.com/acs: "true"
        alibabacloud.com/compute-class: agent-sandbox
        alibabacloud.com/compute-qos: default
```

| Parameter | Required | Description |
| --- | --- | --- |
| `config.use_sandbox_claim` | Yes | `true`: use SandboxSet + SandboxClaim. |
| `config.claim_timeout` | No (default `300`) | Seconds to wait for a sandbox to be claimed. |
| `config.pod_overrides.metadata.labels` | Yes | Written into the SandboxSet pod template; `compute-class` must be `agent-sandbox`. |
| `config.sandbox_labels` | No | Written into the SandboxSet, the SandboxClaim and the pod template; good for business tags. |
| `config.sandbox_env_vars` | No | Environment variables injected when a sandbox is claimed. |

An auto-created SandboxSet lives in `config.namespace` with 5 warm sandboxes by default, and warm
sandboxes are billed too. The server never modifies an existing SandboxSet of the same name, so
pre-create one to control the pool size. The SandboxClaim path does not use DinD builds; for missing
images, use a prebuilt `docker_image` or BuildKit.

### Step 1b — E2B API path

ack-sandbox-manager exposes an E2B-compatible API. The server claims sandboxes by SandboxSet name and
creates no pods itself, so `pod_overrides` has no effect on this path: the SandboxSet alone defines
the sandbox.

First create the SandboxSet as described in the Agent Sandbox documentation. The template must carry
the three ACS labels, and `spec.runtimes` must include `agent-runtime`:

```yaml
apiVersion: agents.kruise.io/v1alpha1
kind: SandboxSet
metadata:
  name: harbor-e2b-pool
  namespace: default
spec:
  runtimes:
    - name: agent-runtime
  replicas: 4
  template:
    metadata:
      labels:
        alibabacloud.com/acs: "true"
        alibabacloud.com/compute-class: agent-sandbox
        alibabacloud.com/compute-qos: default
    spec:
      containers:
        - name: sandbox
          image: <base image>
          resources:
            requests: {cpu: "1", memory: 2Gi}
            limits:   {cpu: "1", memory: 2Gi}
```

Store the ack-sandbox-manager API key in a Secret in the release namespace, then declare the E2B
backend:

```bash
kubectl -n "$ROLLOUT_NAMESPACE" create secret generic e2b-credentials \
  --from-literal=api-key=<E2B_API_KEY>
```

```yaml
config:
  namespace: kube-rl
  backends:
    - name: e2b-pool
      import_path: harbor.environments.e2b:E2BEnvironment
      default: true
      queues: ["default"]
      defaults:
        sandbox_set_name: harbor-e2b-pool
        override_claim_image: true
      allow_overrides: [sandbox_set_name, override_claim_image]
extraEnv:
  - name: E2B_API_KEY
    valueFrom:
      secretKeyRef: {name: e2b-credentials, key: api-key}
  - name: E2B_API_URL
    value: http://sandbox-manager.sandbox-system.svc.cluster.local:8080
  - name: E2B_SANDBOX_URL
    value: http://sandbox-gateway.sandbox-system.svc.cluster.local:7788
```

| Parameter | Required | Description |
| --- | --- | --- |
| `defaults.sandbox_set_name` | Yes | SandboxSet to claim from. Without it the client tries to build a template and gets 404. |
| `defaults.override_claim_image` | No (default `true`) | Replace the sandbox image in place with `docker_image` from `task.toml`; `false` keeps the SandboxSet image. |
| `allow_overrides` | No | Parameters a request may override via `environment_kwargs`; none by default. |
| `E2B_API_KEY` | Yes | Inject only from a Secret. Never put it under `config.*`, which renders into a ConfigMap readable by anyone in the namespace. |
| `E2B_API_URL`, `E2B_SANDBOX_URL` | Yes, for in-cluster access | In-cluster addresses of sandbox-manager and sandbox-gateway; for access from outside, set `E2B_DOMAIN` instead. |

When several pools use different credentials, use `config.env_profiles` and reference the variables
injected by `extraEnv` as `${VAR}`.

### Step 2 — Verify

```bash
kubectl get sbs -A
# expect: AVAILABLE > 0 for the SandboxSet

kubectl -n "$ROLLOUT_NAMESPACE" port-forward svc/"$ROLLOUT_SERVICE" 18080:8080 &
harbor run --path ./my-task --agent oracle \
  --rollout-server http://127.0.0.1:18080
# expect: trial completed, reward 1.0
```

## Troubleshooting

| Symptom | Check / fix |
| --- | --- |
| Hot-data PVC stuck Pending | The default SC is a cloud disk without RWX. In single mode set `accessModes: [ReadWriteOnce]` with a cloud-disk SC; in multi mode use a NAS SC. |
| Server `ImagePullBackOff` on ACS | Without NAT, change both `global.imagePrefix` and `redis.image.repository` to the `-vpc` domain; changing one is not enough. |
| Trial fails with `Forbidden` | `config.namespace` differs from the release namespace, which the Role does not cover. |
| Image-build pod never starts | ACS rejects privileged containers. Use a prebuilt `docker_image`, or set `use_buildkit` + `buildkit_address`. |
| Sandbox on the virtual node, but the instance ID does not start with `acs-` | It landed on ECI: no ACS compute class declared. Add the `alibabacloud.com/acs` or `compute-class` label. |
| Sandbox Pending with `untolerated taint` | `node_selector: {type: virtual-kubelet}` without a `virtual-kubelet.io/provider` toleration; prefer the label method. |
| SandboxClaim sandboxes keep the old config after changing `pod_overrides` | The server does not update an existing SandboxSet. Once no trial is running, delete that SandboxSet; the next submission recreates it with the new config. |
| SandboxClaim times out | Check AVAILABLE with `kubectl get sbs`; if the pool is empty or refills slowly, raise `claim_timeout` or pre-create a larger pool. |
| E2B submission returns 404 | `sandbox_set_name` is not set; the E2B path requires a warm pool. |
| `connection refused on port 49983` | The SandboxSet lacks `spec.runtimes: [{name: agent-runtime}]`, so the sandbox has no envd. |
| Changed only a Secret and nothing happened | Pods roll automatically only on ConfigMap changes: `kubectl -n "$ROLLOUT_NAMESPACE" rollout restart deploy/"$ROLLOUT_RELEASE"`. |

## Notes

- ACS bills the rounded spec; the `pod-use-spec` annotation shows the actual spec. Size sandboxes
  from the `task.toml` resources.
- A preempted `best-effort` sandbox fails its trial; use it only for retryable rollouts.
- Idle sandboxes in a warm pool are billed continuously. The SandboxClaim path creates one pool per
  task environment, so many environments multiply the total; with uniform environments, prefer the
  E2B path with one shared pool and `override_claim_image` to swap images.
- Do not override the sandbox `app` label: the server reclaims leaked sandboxes by
  `app=sandbox,session`.
- Server pods cannot set `compute-class` through values; in an ACS cluster they always use
  `general-purpose`.

## Related documentation

- [ACS pod overview](https://help.aliyun.com/zh/cs/user-guide/acs-pod-instance-overview)
- [Use ACS compute in an ACK managed Pro cluster](https://help.aliyun.com/zh/cs/user-guide/access-acs-computing-power-in-an-ack-cluster)
- [Mixed ECS and ACS scheduling in ACK Pro clusters](https://help.aliyun.com/zh/cs/user-guide/ack-cluster-supports-mixed-scheduling-of-ecs-and-acs-computing)
- [Create an Agent Sandbox in an ACS cluster](https://help.aliyun.com/zh/cs/user-guide/create-an-agent-sandbox)
- [Connect to Agent Sandbox with the E2B SDK](https://help.aliyun.com/zh/cs/user-guide/connect-to-agent-sandbox-using-the-e2b-sdk)
- [Pull ACR images without secrets](https://help.aliyun.com/zh/cs/user-guide/exempting-the-miracular-acr-image)
