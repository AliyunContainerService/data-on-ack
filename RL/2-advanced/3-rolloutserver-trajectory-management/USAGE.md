# Rollout Server configuration and use

This guide covers the ACK Helm chart `ack-rollout-server`: choosing an environment backend, using either the E2B protocol or `SandboxSet`, and sizing concurrency. Start with the [trajectory management guide](README.md); [Ray postprocessing](../4-rolloutserver-ray-trajectory-postprocessing/README.md) depends on this configuration.

## Prerequisites and inputs

The cluster has `ack-sandbox-controller` installed. Install KubeRay Operator when Ray postprocessing is needed. Know the cluster region and namespace, Harbor task manifests and immutable image digests, agent, model, and the names of credential Secrets. Keep Bailian keys out of Helm values, submission metadata, and images. For OSS archiving, also prepare a bucket, a fresh object prefix, and a bound OSS CSI PVC.

```bash
helm repo add ack-incubator https://aliacs-app-catalog.oss-cn-hangzhou.aliyuncs.com/charts-incubator/
helm repo update ack-incubator
helm show chart ack-incubator/ack-rollout-server
helm show values ack-incubator/ack-rollout-server > rollout-defaults.yaml
```

The fields below match chart `0.8.0`; pin and record the installed version, and inspect its values and rendered manifests before upgrades. Match the registry, OSS endpoint, and Bailian Base URL to the chosen regions.

## Choose an environment backend

For ACK SandboxClaim and SandboxSet, use `harbor.environments.ack:ACKEnvironment` with `config.use_sandbox_claim: true`. The controller and CRDs must already exist. `config.namespace` is where Claims are created. A precreated SandboxSet is selected in submission environment parameters, and `override_claim_image` swaps the task image. Verify the actual Claim, Sandbox Pod, and `imageID`; a Ready Pod alone does not prove that the requested digest is running.

```yaml
config:
  namespace: rl-demo
  environment_import_path: harbor.environments.ack:ACKEnvironment
  use_sandbox_claim: true
  claim_timeout: 300
```

For a Harbor task submitted to an existing SandboxSet, set the pool and enable the task image override in `metadata`. `override_claim_image` is a boolean, not an image URL:

```json
{
  "job_id": "example-eval",
  "task_id": "example-task-1",
  "task_path": "example-task",
  "agent": {"name": "my-agent"},
  "environment_kwargs": {
    "sandbox_set_name": "my-sandboxset",
    "override_claim_image": true
  }
}
```

`task_path` must match the top-level directory in the submitted task archive; its `task.toml` supplies the task image. The repository's submission script generates the same parameters: `SERVER=http://localhost:8080 TASK_DIR=./tasks/example-task AGENT=my-agent POOL=my-sandboxset bash submit-task.sh`. Check the SandboxClaim and the actual Pod `imageID` against the task image.

Choose whether the Server manages a pool or you precreate one. The pool replica count, image swap time, and Server concurrency limit are separate concerns. Confirm the actual pool name with one real task before scaling.

For an E2B-compatible Sandbox Manager, use `harbor.environments.e2b:E2BEnvironment`. The Manager, API URL, credentials where required, and a usable pool or template must already exist. This example injects environment-service credentials from a Secret; the Bailian model key is separate.

```yaml
extraEnv:
  - name: E2B_POOL_API_KEY
    valueFrom:
      secretKeyRef:
        name: e2b-pool-credentials
        key: api-key
config:
  backends:
    - name: e2b-pool
      import_path: harbor.environments.e2b:E2BEnvironment
      queues: [e2b]
      defaults:
        sandbox_set_name: my-pool
      allow_overrides: [sandbox_set_name]
      env_profile: e2b-pool
  env_profiles:
    - name: e2b-pool
      env:
        E2B_API_KEY: ${E2B_POOL_API_KEY}
        E2B_API_URL: http://sandbox-manager.<namespace>.svc.cluster.local:8080
```

For multiple backends, declare queues and allowed overrides explicitly. Select the intended backend at submission rather than assuming a default.

## Install and size concurrency

Put the selected backend fragment in `rollout-values.yaml`, render it, then install:

```bash
helm template rollout ack-incubator/ack-rollout-server --version 0.8.0 \
  -n rl-demo -f rollout-values.yaml > rollout-rendered.yaml
helm upgrade --install rollout ack-incubator/ack-rollout-server --version 0.8.0 \
  -n rl-demo --create-namespace -f rollout-values.yaml
```

In `single` mode, `single.workers` controls executor processes and `config.max_concurrent_trials` caps Server trials. Available SandboxSet replicas and cluster quota also bound throughput. `single.ingressProcs` controls HTTP processes, not trial capacity. In `multi` mode, `multi.worker.replicas` scales workers separately, but the chart requires shared RWX run storage and suitable Redis configuration. Start with a small verified workload and size from measured resource use.

```yaml
deploymentMode: single
single:
  ingressProcs: 2
  workers: 4
config:
  max_concurrent_trials: 4
persistence:
  archive:
    existingClaim: rollout-archive-oss
```

Live task archives, run state, and logs need hot storage. `persistence.archive.existingClaim` names a precreated OSS CSI PVC for completed trial tarballs; the PV/PVC supplies bucket, endpoint, identity, and prefix. The chart cannot infer them from the Claim name.

### Write completed archives directly to OSS

The Server can write finished archives through an OSS CSI mount. Create a fresh object prefix and an OSS CSI credential Secret with access to it (or use the cluster's existing CSI identity mechanism). The PVC must be in the Server namespace. Replace the placeholders:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: rollout-archive-oss-pv
spec:
  capacity:
    storage: 100Gi
  accessModes: [ReadWriteMany]
  persistentVolumeReclaimPolicy: Retain
  storageClassName: ""
  csi:
    driver: ossplugin.csi.alibabacloud.com
    volumeHandle: rollout-archive-oss-pv
    nodePublishSecretRef:
      name: oss-csi-credentials
      namespace: rl-demo
    volumeAttributes:
      bucket: <bucket-name>
      path: rollout-archive/example/
      url: oss-<region>-internal.aliyuncs.com
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: rollout-archive-oss
  namespace: rl-demo
spec:
  accessModes: [ReadWriteMany]
  resources:
    requests:
      storage: 100Gi
  storageClassName: ""
  volumeName: rollout-archive-oss-pv
```

Set these values in `rollout-values.yaml`:

```yaml
persistence:
  enabled: true
  nas:
    existingClaim: <rwx-hot-storage-pvc>
  archive:
    existingClaim: rollout-archive-oss
    mountPath: /mnt/archive
```

Save the resources as `rollout-archive-oss.yaml`, run `kubectl apply -f rollout-archive-oss.yaml`, and check `kubectl -n rl-demo get pvc rollout-archive-oss` reports `Bound`. Then use the `helm template` and `helm upgrade --install` commands above; confirm the rendered Server Pod mounts the archive PVC. After a real run, check for `<run_id>.tar.gz` and, when an agent trajectory exists, `<run_id>.trajectory.json` under `rollout-archive/example/`. Archiving can lag run completion according to `config.archive_after_seconds` and the scan interval. Configure OSS event notifications for that prefix and the object creation event types actually emitted.

### Agent writes directly to OSS inside the Sandbox

This is a separate data path. Mount a **separate writable OSS CSI PVC** into the SandboxSet Pod template, and have the agent write to that mount. Create the PVC in the SandboxSet namespace, using the PV/PVC pattern above with new names, `path: agent-output/example/`, and an identity authorized to write only that prefix. A precreated SandboxSet template can include:

```yaml
apiVersion: agents.kruise.io/v1alpha1
kind: SandboxSet
metadata:
  name: my-sandboxset
  namespace: rl-demo
spec:
  replicas: 4
  runtimes:
    - name: agent-runtime
  template:
    spec:
      containers:
        - name: main
          image: <base-sandbox-image>
          volumeMounts:
            - name: agent-output
              mountPath: /mnt/oss-output
      volumes:
        - name: agent-output
          persistentVolumeClaim:
            claimName: agent-output-oss
```

If Harbor creates the pool, put the same `volumes` and `volumeMounts` in its `pod_overrides`; check an existing pool and its live Sandboxes before changing its template. Submit with `sandbox_set_name` and `override_claim_image: true` as above. The agent writes from inside the Sandbox, for example:

```python
import json
import uuid
from pathlib import Path

output_id = uuid.uuid4().hex
out = Path("/mnt/oss-output") / output_id
out.mkdir(parents=True, exist_ok=True)
(out / "agent-result.json").write_text(
    json.dumps({"output_id": output_id, "status": "completed"}), encoding="utf-8"
)
print(f"oss_output_id={output_id}")  # retain in agent logs to link back to the Server run
```

Verify `/mnt/oss-output` is writable inside the Sandbox Pod, then use the logged `oss_output_id` to read `agent-output/example/<output_id>/agent-result.json` from OSS. Use a distinct object name per run. **This directly writes files created by the agent only**; Harbor's native `agent/trajectory.json`, verifier results, and Server archive keep their own lifecycle. For Harbor to skip automatic artifact copy-back, its Trial directory and Sandbox `/logs/agent` must be mapped onto the same shared volume with Harbor's `shared_volume` capability. Current Harbor rejects `shared_volume` together with `use_sandbox_claim: true`, so a SandboxSet mount alone does not make native trajectories direct-write. `config.storage_backend: oss` replaces neither the Sandbox mount nor the archive PVC. Keep live state and WAL on NAS/CPFS hot storage.

Pass model credentials through a Secret using the variables the actual agent SDK reads. Mini SWE Agent via LiteLLM requires `OPENAI_API_KEY` and a region-appropriate `OPENAI_BASE_URL` for Bailian's OpenAI-compatible API. A direct model probe does not prove those variables reached the agent process.

## Acceptance checks

Run one real Harbor task with a pinned digest. Save job/run ID, input hashes, Claim, Sandbox Pod, and `imageID`; poll `/api/v1/runs/async/<run-id>/status` and `/result`. Confirm model token usage, verifier reward, native `agent/trajectory.json`, and archive all belong to the same run. A health check or Oracle reward is insufficient evidence of a real agent trajectory. For [Ray postprocessing](../4-rolloutserver-ray-trajectory-postprocessing/README.md), validate signal delivery and attribution content separately; a turn count is not failure attribution.
