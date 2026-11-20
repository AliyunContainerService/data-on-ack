# Rollout Server 使用与配置

本指南说明 ACK Helm 仓库的 `ack-rollout-server` 如何选择 Harbor 环境后端、使用 E2B 协议或 `SandboxSet`，以及如何控制并发。部署与归档验收见[轨迹管理入门](README_ZH.md)；[Ray 轨迹后处理](../4-rolloutserver-ray-trajectory-postprocessing/README_ZH.md)以本指南为前提。

## 前提与输入

集群已安装 `ack-sandbox-controller`；需要 Ray 后处理时还应安装 KubeRay Operator。准备好集群地域、namespace、可拉取的 Harbor 任务镜像及固定 digest、所用 agent 与模型、密钥 Secret 名称。不要把百炼 API Key 写进 Helm values、提交元数据或镜像。OSS 归档是可选功能；启用时另外准备 Bucket、独立对象前缀和已绑定的 OSS CSI PVC。集群已有 Rollout Server 时先检查 release 的 chart 版本和值，再升级。

```bash
helm repo add ack-incubator https://aliacs-app-catalog.oss-cn-hangzhou.aliyuncs.com/charts-incubator/
helm repo update ack-incubator
helm show chart ack-incubator/ack-rollout-server
helm show values ack-incubator/ack-rollout-server > rollout-defaults.yaml
```

示例字段按 chart `0.8.0` 编写；安装时固定并记录实际版本，升级前核对对应版本的 values 和渲染结果。地域要同时体现在镜像仓库地址、OSS endpoint 和模型 Base URL；不要把某个地域的地址直接复制到另一地域。

## 选择环境后端

### ACK SandboxClaim 与 SandboxSet

`config.environment_import_path: harbor.environments.ack:ACKEnvironment` 是 ACK 后端。设 `config.use_sandbox_claim: true` 后，试次通过 SandboxClaim 取得沙箱；前提是集群安装了相应控制器及 CRD。`config.namespace` 指定创建 Claim 的 namespace。预建 SandboxSet 时，让提交的环境参数指向该池；任务镜像通过 `override_claim_image` 原地切换。为任务镜像配置可持续刷新的 `imagePullSecrets`，并在运行时核对 Claim→Sandbox Pod→实际 `imageID`。只看到一个 Ready Pod 不能证明它加载了目标任务镜像。

```yaml
config:
  namespace: rl-demo
  environment_import_path: harbor.environments.ack:ACKEnvironment
  use_sandbox_claim: true
  claim_timeout: 300
```

提交已有 SandboxSet 中的 Harbor 任务时，`metadata` 中这样指定池并启用任务镜像替换（`override_claim_image` 是布尔值，不是镜像地址）：

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

`task_path` 要与提交的任务压缩包顶层目录同名；任务目录中的 `task.toml` 提供需要替换的镜像。也可用仓库中的提交脚本生成同样的参数：`SERVER=http://localhost:8080 TASK_DIR=./tasks/example-task AGENT=my-agent POOL=my-sandboxset bash submit-task.sh`。确认 SandboxClaim 和实际 Pod 的 `imageID` 与任务镜像一致。

预建池大小、任务镜像切换和执行并发是三个不同参数。池副本数应覆盖实际同时运行的试次，并留出镜像拉取与依赖安装时间。可以由 Server 管理池，也可以预建 `SandboxSet`；两者应先选定一种，以免任务名自动生成的池与预建池不一致。提交前用一个真实任务确认后端实际使用的池名。

### E2B 协议

对实现 E2B API 的 Sandbox Manager，使用 Harbor 的 `E2BEnvironment`。它需要 API URL、API Key（若服务要求）以及可用的沙箱模板或池；仅切换 `environment_import_path` 不会自动安装 Manager。下例把凭据放在 Kubernetes Secret 中，Server 通过 `extraEnv` 读取，并用 `env_profile` 注入到所选后端；实际 URL 和池名取决于你的 Manager 部署。

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

这里的 E2B API Key 用于环境服务；百炼模型 Key 是另一项凭据。若仅用一个后端，也可按 chart 的 `config.environment_import_path` 配置；多后端场景显式声明 `backends`、队列和允许的覆盖字段，避免提交进入错误后端。

## 安装、并发和持久化

把选择的后端片段写入 `rollout-values.yaml` 后，先渲染再安装：

```bash
helm template rollout ack-incubator/ack-rollout-server --version 0.8.0 \
  -n rl-demo -f rollout-values.yaml > rollout-rendered.yaml
helm upgrade --install rollout ack-incubator/ack-rollout-server --version 0.8.0 \
  -n rl-demo --create-namespace -f rollout-values.yaml
```

`single` 模式的 `single.workers` 是执行进程数，`config.max_concurrent_trials` 是 Server 的试次并发上限；实际同时能运行的数量还受 SandboxSet 可用副本与集群配额约束。`single.ingressProcs` 只控制 HTTP 进程，不增加试次容量。`multi` 模式可独立调整 `multi.worker.replicas`，但需要满足 chart 对共享 RWX 运行存储和 Redis 的要求。先小规模验证，再按任务资源消耗调整；不要把示例数值当通用推荐值。

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

运行中的 task archive、状态和日志需要热存储。`persistence.archive.existingClaim` 指向预创建的 OSS CSI PVC，用于已完成试次的 tar 归档；OSS CSI 的 Bucket、endpoint、鉴权和对象前缀属于 PV/PVC 配置，不能只填 chart 的 Claim 名。若不配置归档 PVC，先确认 Server 的本地归档和保留策略满足需求。

### 完成归档直写 OSS

下例让 Rollout Server 在试次完成后通过 OSS CSI 挂载点写入归档。先准备一个**新前缀**和具备该前缀读写权限的 OSS CSI 凭据 Secret（或按集群现有身份方案配置 CSI）；PVC 与 Server 必须在同一 namespace。将占位值替换为实际 Bucket、地域 endpoint 和 Secret 名称：

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

在 `rollout-values.yaml` 中设置：

```yaml
persistence:
  enabled: true
  nas:
    existingClaim: <rwx-hot-storage-pvc>
  archive:
    existingClaim: rollout-archive-oss
    mountPath: /mnt/archive
```

把上面的资源保存为 `rollout-archive-oss.yaml` 后，执行 `kubectl apply -f rollout-archive-oss.yaml` 和 `kubectl -n rl-demo get pvc rollout-archive-oss`，确认状态为 `Bound`。再按上文的 `helm template` 和 `helm upgrade --install` 命令安装，检查渲染出的 Server Pod 含 `archive` PVC 挂载。提交一条真实任务，等待归档完成后，在 OSS 的 `rollout-archive/example/` 下核对 `<run_id>.tar.gz` 和有轨迹时的 `<run_id>.trajectory.json`。归档写入可能晚于试次完成，等待时间由 `config.archive_after_seconds` 与扫描间隔控制。Ray 的 OSS 事件规则应监听这个前缀中的归档对象，并覆盖实际产生的对象创建事件类型。

### Agent 在 Sandbox 内直接写 OSS

这是另一条数据路径：给 SandboxSet 的 Pod 模板挂载**独立的可写 OSS CSI PVC**，agent 向容器内的挂载目录写文件。该 PVC 应与 SandboxSet 位于同一 namespace，绑定到专用的 OSS 输出前缀；按上一节 PV/PVC 的方式创建，但改用新的 PV/PVC 名、`path: agent-output/example/` 和允许写入的身份。只向这一前缀授权写权限。预建 SandboxSet 的模板示例（保留已有镜像、资源和 runtime 字段）：

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

若由 Harbor 创建 SandboxSet，可把同一 `volumes`/`volumeMounts` 放进建池命令的 `pod_overrides`；已存在池则先检查模板和运行中 Sandbox，再安排更新。提交任务时仍使用 `sandbox_set_name` 与 `override_claim_image: true`。agent 在沙箱中写入，例如：

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
print(f"oss_output_id={output_id}")  # 保留在 agent 日志中，供 Server run 关联
```

先在 Sandbox Pod 内确认 `/mnt/oss-output` 可写，再用日志中的 `oss_output_id` 从 OSS 的 `agent-output/example/<output_id>/agent-result.json` 读取并核对内容。不要在多个试次间复用同一对象名。**该示例只让 agent 自己写的文件直达 OSS**；Harbor 原生 `agent/trajectory.json`、verifier 结果及 Server 归档仍走各自的生命周期。若希望 Harbor 自动跳过结果回传，需要把 Trial 目录与 Sandbox 的 `/logs/agent` 映射到同一共享卷，并启用 Harbor `shared_volume` 能力；当前 Harbor 的 `shared_volume` 与 `use_sandbox_claim: true` 不兼容，不能仅靠上述 SandboxSet 挂载宣称原生轨迹已直写。`config.storage_backend: oss` 也不能替代 Sandbox 挂载或归档 PVC。热状态、WAL 仍留在 NAS/CPFS 等热存储。

模型凭据使用 Secret，并按 agent 的实际 SDK 环境变量名传递。Mini SWE Agent 通过 LiteLLM 调百炼 OpenAI 兼容接口时需要 `OPENAI_API_KEY` 与相应地域的 `OPENAI_BASE_URL`；不要只凭一次直连模型探针就认定 agent 进程拿到了凭据。试次结束后校验 Server 原生 `agent/trajectory.json`、`verifier/reward.txt` 和结果状态，不能用 `/health` 或 Oracle 奖励代替真实 agent 轨迹。

## 验收与排障

提交一个固定 digest 的真实 Harbor 任务，保存 job/run ID、输入哈希、Claim 名、Sandbox Pod 和 `imageID`；轮询 `/api/v1/runs/async/<run-id>/status` 与 `/result`。确认模型 token、verifier reward、原生轨迹、OSS 归档均来自同一 run。若后续接 [Ray 后处理](../4-rolloutserver-ray-trajectory-postprocessing/README_ZH.md)，再分别验收信号抵达和归因内容；turn 计数本身不等于失败归因。
