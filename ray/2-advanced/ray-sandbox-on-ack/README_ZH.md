# 在 ACK 上运行 Ray Sandbox

本文面向需要在阿里云容器服务 Kubernetes 版（ACK）上运行 Ray Sandbox 的用户，提供从零搭建到生产化的完整操作指导。
文中全部操作均提供命令行（aliyun CLI 与 kubectl）实现方式，并已在 ACK 托管集群（东京地域，Kubernetes 1.36.2-aliyun.1）上实测验证通过。

## 背景
### Ray Sandbox 与 gVisor
Ray 自 2.58 起提供实验性接口 `ray.experimental.sandbox`，允许 Ray 作业在集群节点上按需创建相互隔离的沙箱容器，并在沙箱内执行文件写入、命令执行等操作。该能力适用于在 Ray 集群中运行不可信代码的场景，例如大模型生成的代码、用户上传的脚本等。

Ray Sandbox 使用 gVisor 作为隔离运行时。gVisor 是 Google 开源的应用内核，其核心组件 runsc 以用户态进程形式拦截并处理容器的系统调用，使容器与宿主内核之间多一层隔离。Ray 节点通过 runsc 启动沙箱容器，沙箱内的代码即使被执行，也无法直接访问宿主内核。

### 两种部署形态
在 ACK 上可以采用两种形态部署 Ray Sandbox，区别在于 Ray 节点本身的隔离级别：

| 对比项 | 基础形态 | 进阶形态 |
| --- | --- | --- |
| Ray 节点类型 | 普通 ECS 节点（runc 容器） | ACK 安全沙箱节点（rund，microVM） |
| 沙箱运行模式 | rootless gVisor | rootful gVisor |
| 隔离层次 | 沙箱（gVisor）一层 | 节点级 microVM + 沙箱（gVisor）两层 |
| 沙箱 CPU/内存限额 | 不生效（rootless 模式的已知行为） | 生效 |
| 节点内核参数调整 | 需要开启 `user.max_user_namespaces` | 不需要 |
| 适用场景 | 开发测试、一般多租户 | 强隔离要求的生产环境 |

### 前提条件
- 已安装并配置 aliyun CLI（`aliyun configure` 完成凭据配置），且账号具备 ACK、ECS、VPC、ACR 的操作权限。
- 已安装 kubectl 与 Helm（可选）。
- 本文涉及的地域、集群、实例等标识均以环境变量引用，请先按下表设置：

```bash
export REGION=ap-northeast-1        # 集群所在地域，本文以东京为例
export CLUSTER_ID=<集群 ID>          # 第一步创建集群后获得
export GV_NODEPOOL_ID=<基础节点池 ID>    # 基础形态章节创建后获得
export RUND_NODEPOOL_ID=<进阶节点池 ID>  # 进阶形态章节创建后获得
```

### 版本约定
本文所有步骤均在以下版本组合下验证通过：

| 组件 | 版本 |
| --- | --- |
| ACK Kubernetes | 1.36.2-aliyun-1（容器运行时 containerd 2.3.4） |
| KubeRay | v1.7.0（ACK 组件 kuberay-operator） |
| Ray | 2.58.0（镜像 rayproject/ray:2.58.0-py312） |
| gVisor | 20260921.0（打包格式，详见基础形态第三节） |

## 第一步：创建 ACK 集群
操作对象：

在目标地域创建一个 ACK 托管版 Pro 集群。以下请求体不包含既有 VPC 标识，ACK 将自动创建 VPC 与三个可用区的交换机，并创建系统节点池（2 台 ecs.g9i.xlarge）。

操作步骤：

1. 将以下内容保存为 `create-cluster.json`：

```json
{
  "name": "ray-sandbox-guide",
  "cluster_type": "ManagedKubernetes",
  "cluster_spec": "ack.pro.small",
  "kubernetes_version": "1.36.2-aliyun.1",
  "region_id": "ap-northeast-1",
  "zone_ids": ["ap-northeast-1c", "ap-northeast-1d", "ap-northeast-1e"],
  "snat_entry": true,
  "endpoint_public_access": true,
  "service_cidr": "192.168.0.0/16",
  "proxy_mode": "ipvs",
  "addons": [
    {"name": "terway-controlplane"},
    {"name": "terway-eniip", "config": "{\"IPVlan\":\"false\",\"NetworkPolicy\":\"true\"}"},
    {"name": "csi-plugin"},
    {"name": "managed-csiprovisioner"},
    {"name": "coredns"},
    {"name": "metrics-server"}
  ],
  "nodepools": [
    {
      "nodepool_info": {"name": "system"},
      "kubernetes_config": {"runtime": "containerd", "runtime_version": "2.3.4"},
      "scaling_group": {
        "instance_types": ["ecs.g9i.xlarge"],
        "image_type": "AliyunLinux3ContainerOptimized",
        "instance_charge_type": "PostPaid",
        "system_disk_category": "cloud_essd",
        "system_disk_size": 40,
        "data_disks": [{"category": "cloud_essd", "size": 120}],
        "desired_size": 2
      }
    }
  ]
}
```

2. 创建集群：

```bash
aliyun cs POST /clusters --region ${REGION} \
  --header "Content-Type=application/json" \
  --body "$(cat create-cluster.json)"
```

返回 JSON 中的 `cluster_id` 即为集群标识，请设置为环境变量 `CLUSTER_ID`。

3. 轮询集群状态直至进入 `running`：

```bash
aliyun cs GET /clusters/${CLUSTER_ID} --region ${REGION} \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["state"])'
```

预期结果与停止条件：

输出为 `running`（通常需要 10 至 15 分钟）。若状态为 `failed` 或长时间停留在 `initial`，请停止后续步骤并查看返回体中的 `error_msg` 字段。

> 说明：aliyun CLI 的 cs 命令必须显式指定 `--region`，否则将使用 CLI 配置中的默认地域发起请求。

## 第二步：配置集群访问与 KubeRay
操作对象：

获取集群 kubeconfig，并通过 ACK 组件管理安装 kuberay-operator。

操作步骤：

1. 获取 kubeconfig 并写入独立文件：

```bash
aliyun cs GET /k8s/${CLUSTER_ID}/user_config --region ${REGION} \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["config"])' \
  > ~/.kube/config-ray-sandbox
export KUBECONFIG=~/.kube/config-ray-sandbox
kubectl get nodes
```

2. 安装 kuberay-operator 组件：

```bash
aliyun cs POST /clusters/${CLUSTER_ID}/components/install \
  --region ${REGION} --secure \
  --header "Content-Type=application/json" \
  --body '[{"name": "kuberay-operator"}]'
```

3. 确认组件状态与 CRD 就绪：

```bash
aliyun cs GET /clusters/${CLUSTER_ID}/addon_instances \
  --region ${REGION} --secure \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); [print(a["name"], a["state"], a["version"]) for a in d["addons"] if a["name"]=="kuberay-operator"]'
kubectl get crd rayjobs.ray.io rayclusters.ray.io rayservices.ray.io
```

预期结果与停止条件：

组件输出为 `kuberay-operator active v1.7.0-release.5`（版本号以实际为准），三条 CRD 均存在。若 CRD 缺失，请停止并重新执行安装命令。

> 说明：组件管理类接口要求使用 HTTPS 访问，aliyun CLI 需要附加 `--secure` 参数。

## 基础形态：普通节点 + rootless gVisor
### 一、创建专用节点池
操作对象：

创建带标签与污点的节点池，使 Ray 工作负载与普通负载隔离。

操作步骤：

1. 查询集群自动创建的交换机标识：

```bash
VPC_ID=$(aliyun cs GET /clusters/${CLUSTER_ID} --region ${REGION} \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["vpc_id"])')
aliyun vpc DescribeVSwitches --RegionId ${REGION} --VpcId ${VPC_ID} \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); [print(v["VSwitchId"]) for v in d["VSwitches"]["VSwitch"]]'
```

2. 将以下内容保存为 `nodepool-gvisor.json`，`vswitch_ids` 替换为上一步输出的交换机标识：

```json
{
  "nodepool_info": {"name": "ray-gvisor"},
  "kubernetes_config": {
    "runtime": "containerd",
    "runtime_version": "2.3.4",
    "labels": [{"key": "ray-sandbox", "value": "gvisor"}],
    "taints": [{"key": "ray-sandbox", "value": "gvisor", "effect": "NoSchedule"}]
  },
  "scaling_group": {
    "vswitch_ids": ["<vsw-xxx>", "<vsw-yyy>", "<vsw-zzz>"],
    "instance_types": ["ecs.g9i.2xlarge"],
    "image_type": "AliyunLinux3ContainerOptimized",
    "instance_charge_type": "PostPaid",
    "system_disk_category": "cloud_essd",
    "system_disk_size": 40,
    "data_disks": [{"category": "cloud_essd", "size": 120}],
    "desired_size": 1
  }
}
```

3. 创建节点池：

```bash
aliyun cs POST /clusters/${CLUSTER_ID}/nodepools --region ${REGION} \
  --header "Content-Type=application/json" \
  --body "$(cat nodepool-gvisor.json)"
```

返回 JSON 中的 `nodepool_id` 请设置为环境变量 `GV_NODEPOOL_ID`。

预期结果与停止条件：

执行 `kubectl get nodes -l ray-sandbox=gvisor` 出现 1 个 Ready 节点。若节点池任务失败，请停止并查询任务详情。

### 二、开启 user.max_user_namespaces
背景：

rootless gVisor 依赖用户命名空间（user namespace）映射容器内 root。gVisor 官方要求宿主机 `user.max_user_namespaces` 不小于 15000，
而 Alibaba Cloud Linux 3 节点镜像默认值为 0，必须调整。
ACK 节点池操作系统参数接口支持对存量与新增节点滚动生效。

操作步骤：

1. 调用节点池操作系统参数接口：

```bash
aliyun cs PUT /clusters/${CLUSTER_ID}/nodepools/${GV_NODEPOOL_ID}/node_config \
  --region ${REGION} \
  --header "Content-Type=application/json" \
  --body '{"os_config":{"sysctl":{"user.max_user_namespaces":"65536"}},"rolling_policy":{"max_parallelism":1}}'
```

返回 JSON 中的 `task_id` 标识本次滚动任务。

2. 任务完成后，通过云助手在节点上验证。节点实例标识可通过 `kubectl get node <节点名> -o jsonpath='{.spec.providerID}'` 获取：

```bash
aliyun ecs RunCommand --RegionId ${REGION} --Type RunShellScript \
  --CommandContent "sysctl user.max_user_namespaces" \
  --InstanceId <节点实例 ID>
aliyun ecs DescribeInvocationResults --RegionId ${REGION} \
  --InvocationId <上一步返回的 InvokeId> \
  | python3 -c 'import json,sys,base64; r=json.load(sys.stdin)["Invocation"]["InvocationResults"]["InvocationResult"][0]; print(base64.b64decode(r["Output"]).decode())'
```

预期结果与停止条件：

输出 `user.max_user_namespaces = 65536`。若仍为 0，请停止并确认任务状态为成功。

### 三、部署并验证 RayJob
#### 背景：gVisor 发布物变更
gVisor 自 20260831.0 起不再发布独立的 runsc 二进制文件，仅提供打包文件 `gvisor.tar.zstd`（或 `gvisor.tar.bz2`），
包内包含 runsc、containerd-shim-runsc-v1 以及 `gvisor-bin/` 目录。
新版 runsc 以 STRICT 策略依赖同目录下的 `gvisor-bin/gvisor_sentry`，仅解压 runsc 单文件将无法启动沙箱。
Ray 官方样例 `ray-job.sandbox.yaml` 中引用的 `release/latest/.../runsc` 地址已随该变更失效（返回 404）。
本文使用固定版本 20260921.0，并在下载后执行 SHA-512 校验。

操作步骤：

1. 将以下内容保存为 `rayjob-basic.yaml`：

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ray-sandbox
---
apiVersion: ray.io/v1
kind: RayJob
metadata:
  name: rayjob-sandbox
  namespace: ray-sandbox
spec:
  entrypoint: |
    python3 -c "
    import ray
    from ray.experimental import sandbox

    ray.init()

    sb = sandbox.create(
        image='python:3.12-slim',
        workdir='/workspace',
        cpu=1.0,
        memory='1Gi',
    )

    script = '''
    import platform
    import sys

    print('=== Hello from inside Ray Sandbox! ===')
    print(f'Python Version : {sys.version}')
    print(f'Platform       : {platform.platform()}')
    with open('/proc/meminfo') as f:
        print(f'MemTotal       : {f.readline().split()[1]} kB')
    '''
    ray.get(sb.write_file.remote('/workspace/main.py', script))

    result = ray.get(sb.exec.remote('python3 /workspace/main.py'))
    print(f'Exit code: {result.exit_code}')
    print('Sandbox output:')
    print(result.stdout)

    ray.get(sb.delete.remote())
    print('RayJob completed successfully!')
    "
  shutdownAfterJobFinishes: true
  ttlSecondsAfterFinished: 600
  rayClusterSpec:
    rayVersion: '2.58.0'
    headGroupSpec:
      rayStartParams:
        dashboard-host: '0.0.0.0'
      template:
        spec:
          nodeSelector:
            ray-sandbox: gvisor
          tolerations:
          - key: ray-sandbox
            operator: Equal
            value: gvisor
            effect: NoSchedule
          containers:
          - name: ray-head
            image: rayproject/ray:2.58.0-py312
            imagePullPolicy: IfNotPresent
            securityContext:
              seccompProfile:
                type: Unconfined
              appArmorProfile:
                type: Unconfined
            lifecycle:
              postStart:
                exec:
                  command:
                  - /bin/sh
                  - -c
                  - |
                    set -e
                    GVISOR_VERSION=20260921.0
                    URL=https://storage.googleapis.com/gvisor/releases/release/${GVISOR_VERSION}/$(uname -m)
                    cd /tmp
                    wget -q ${URL}/gvisor.tar.zstd ${URL}/gvisor.tar.zstd.sha512
                    sha512sum -c gvisor.tar.zstd.sha512
                    zstd -dc gvisor.tar.zstd | sudo tar -x -C /usr/local/bin runsc gvisor-bin
                    rm -f gvisor.tar.zstd gvisor.tar.zstd.sha512
            resources:
              requests:
                cpu: "1"
                memory: "4Gi"
              limits:
                cpu: "2"
                memory: "4Gi"
    workerGroupSpecs:
    - groupName: worker-group
      replicas: 1
      minReplicas: 1
      maxReplicas: 3
      rayStartParams: {}
      template:
        spec:
          nodeSelector:
            ray-sandbox: gvisor
          tolerations:
          - key: ray-sandbox
            operator: Equal
            value: gvisor
            effect: NoSchedule
          containers:
          - name: ray-worker
            image: rayproject/ray:2.58.0-py312
            imagePullPolicy: IfNotPresent
            securityContext:
              seccompProfile:
                type: Unconfined
              appArmorProfile:
                type: Unconfined
            lifecycle:
              postStart:
                exec:
                  command:
                  - /bin/sh
                  - -c
                  - |
                    set -e
                    GVISOR_VERSION=20260921.0
                    URL=https://storage.googleapis.com/gvisor/releases/release/${GVISOR_VERSION}/$(uname -m)
                    cd /tmp
                    wget -q ${URL}/gvisor.tar.zstd ${URL}/gvisor.tar.zstd.sha512
                    sha512sum -c gvisor.tar.zstd.sha512
                    zstd -dc gvisor.tar.zstd | sudo tar -x -C /usr/local/bin runsc gvisor-bin
                    rm -f gvisor.tar.zstd gvisor.tar.zstd.sha512
            resources:
              requests:
                cpu: "2"
                memory: "8Gi"
              limits:
                cpu: "4"
                memory: "8Gi"
```

要点说明：

- postStart 钩子在每个 Ray 容器启动后下载并校验 gVisor 打包文件，将 `runsc` 与 `gvisor-bin` 两个成员一并解压至 `/usr/local/bin`。
Ray 镜像中 zstd 位于 `/home/ray/anaconda3/bin`，不在 sudo 的 secure_path 内，因此先以普通用户解压数据流，再经 sudo 以 root 身份提取文件。
- 容器默认以 ray 用户（uid 1000）运行，沙箱工作于 rootless 模式（`rootless` 参数缺省为 `true`）。
- seccomp 与 AppArmor 设置为 Unconfined，避免宿主机安全策略拦截 gVisor 所需的用户命名空间操作。

2. 部署并轮询：

```bash
kubectl apply --dry-run=client -f rayjob-basic.yaml
kubectl apply -f rayjob-basic.yaml
kubectl -n ray-sandbox get rayjob rayjob-sandbox \
  -o jsonpath='{.status.jobDeploymentStatus}/{.status.jobStatus}'
```

3. 作业完成后查看输出：

```bash
kubectl -n ray-sandbox get pods
kubectl -n ray-sandbox logs <submitter Pod 名称>
```

预期结果与停止条件：

`jobDeploymentStatus` 为 `Complete` 且 `jobStatus` 为 `SUCCEEDED`，日志包含以下内容：

```text
Exit code: 0
=== Hello from inside Ray Sandbox! ===
Platform       : Linux-4.19.0-gvisor-x86_64-with-glibc2.41
MemTotal       : 32053100 kB
RayJob completed successfully!
```

`Platform` 字段包含 `gvisor`，表明沙箱进程运行于 gVisor 内核之上。`MemTotal` 显示的是节点内存总量而非声明的 1 GiB：rootless 模式下 runsc 无法写入 cgroup 限额文件，
Ray 会静默忽略 `cpu` 与 `memory` 参数，此为 Ray 2.58.0 在 rootless 模式下的已知行为。如需限额生效，请使用进阶形态。

若 `jobDeploymentStatus` 为 `Failed`，请停止并参考故障排查章节。

## 进阶形态：安全沙箱节点 + rootful gVisor
### 一、原理与约束
ACK 安全沙箱（rund）将 Pod 运行于轻量虚拟机（microVM）内，guest 内核为 kangaroo 内核，与宿主内核隔离。
在 Kubernetes 1.35 及以上版本的 ACK 集群中，节点池声明 `node_components: [{"name": "sandbox-runtime"}]` 后，
sandboxed-container-controller 组件会自动创建名为 `sandbox` 的 RuntimeClass，购买节点实例时自动开启嵌套虚拟化（`CpuOptions.NestedVirtualization=enabled`）。
Ray 节点以 `runtimeClassName: sandbox` 调度到该类节点后，容器本身已处于 microVM 隔离之中；
在容器内再以 rootful 模式运行 runsc，可形成节点级虚拟化与 gVisor 应用内核的双层隔离，且 rootful 模式下沙箱的 CPU 与内存限额正常生效。

约束条件：

- 仅第九代 ECS 实例规格族中支持嵌套虚拟化的规格可用，且不同规格档位支持情况存在差异。实测 `ecs.g9i.8xlarge` 可用；`ecs.g9i.2xlarge` 在创建节点池时返回 `InvalidInstanceType.NotSupportCpuOptionsNestedVirtualization`。创建前请以实际报错为准调整规格。
- 节点池必须声明 `node_components: [{"name": "sandbox-runtime"}]`，否则创建的节点不具备嵌套虚拟化能力。
- rund guest 环境为 cgroup v1，且进程默认处于带冒号的 systemd slice 路径（如 `kubepods.slice:...`），runsc 无法直接使用，需在启动脚本中修复（见下文 bootstrap 脚本）。

### 二、创建安全沙箱节点池
操作步骤：

1. 将以下内容保存为 `nodepool-rund.json`，`vswitch_ids` 替换为集群交换机标识：

```json
{
  "nodepool_info": {"name": "ray-gvisor-rund"},
  "kubernetes_config": {
    "runtime": "containerd",
    "runtime_version": "2.3.4",
    "labels": [{"key": "ray-sandbox", "value": "rund"}],
    "taints": [{"key": "ray-sandbox", "value": "rund", "effect": "NoSchedule"}]
  },
  "node_components": [{"name": "sandbox-runtime"}],
  "scaling_group": {
    "vswitch_ids": ["<vsw-xxx>", "<vsw-yyy>", "<vsw-zzz>"],
    "instance_types": ["ecs.g9i.8xlarge"],
    "image_type": "AliyunLinux3ContainerOptimized",
    "instance_charge_type": "PostPaid",
    "system_disk_category": "cloud_essd",
    "system_disk_size": 40,
    "data_disks": [{"category": "cloud_essd", "size": 120}],
    "desired_size": 1
  }
}
```

2. 创建节点池：

```bash
aliyun cs POST /clusters/${CLUSTER_ID}/nodepools --region ${REGION} \
  --header "Content-Type=application/json" \
  --body "$(cat nodepool-rund.json)"
```

返回 JSON 中的 `nodepool_id` 请设置为环境变量 `RUND_NODEPOOL_ID`。

3. 节点 Ready 后确认 RuntimeClass 已自动创建：

```bash
kubectl get runtimeclass sandbox -o yaml
```

预期结果与停止条件：

RuntimeClass `sandbox` 存在，handler 为 `sandbox`，`overhead` 为 64Mi，`nodeSelector` 为 `alibabacloud.com/container-runtime=sandbox`。
若节点池任务失败并提示 `InvalidInstanceType.NotSupportCpuOptionsNestedVirtualization`，请停止并更换为支持嵌套虚拟化的实例规格。

### 三、探测节点环境
操作对象：

在编写工作负载前，以一个探针 Pod 确认 rund guest 的关键环境特征。

操作步骤：

将以下内容保存为 `rund-prereq.yaml` 并执行 `kubectl apply -f rund-prereq.yaml`，随后查看日志：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: rund-prereq
  namespace: ray-sandbox
spec:
  runtimeClassName: sandbox
  nodeSelector:
    ray-sandbox: rund
  tolerations:
  - key: ray-sandbox
    operator: Equal
    value: rund
    effect: NoSchedule
  restartPolicy: Never
  containers:
  - name: ray
    image: rayproject/ray:2.58.0-py312
    command:
    - /bin/bash
    - -lc
    - |
      echo '=== identity ==='
      id
      echo '=== kernel ==='
      uname -a
      echo '=== cgroup ==='
      cat /proc/self/cgroup
      echo '=== user namespace ==='
      sysctl user.max_user_namespaces
      echo '=== KVM ==='
      ls -l /dev/kvm 2>&1 || true
    securityContext:
      privileged: true
      runAsUser: 0
      seccompProfile:
        type: Unconfined
      appArmorProfile:
        type: Unconfined
    resources:
      requests:
        cpu: "2"
        memory: "4Gi"
      limits:
        cpu: "4"
        memory: "8Gi"
```

#### 预期结果（实测值）
| 探测项 | 实测值 | 含义 |
| --- | --- | --- |
| identity | uid=0(root) | privileged 容器内为 root |
| kernel | `5.10.134-013.9.1.kangaroo.al8` | kangaroo guest 内核，确认运行于 microVM 内 |
| cgroup | v1 挂载，路径形如 `kubepods.slice:...` | 冒号路径需修复后方可运行 runsc |
| user.max_user_namespaces | 32645 | guest 内默认值，随 microVM 规格变化 |
| /dev/kvm | 不存在 | guest 内不可再嵌套虚拟化 |

探测完成后删除探针 Pod：`kubectl -n ray-sandbox delete pod rund-prereq`。

### 四、部署并验证 RayJob
#### 设计要点
1. KubeRay 注解 `ray.io/overwrite-container-cmd: "true"` 将 Ray 容器启动命令转存至环境变量 `KUBERAY_GEN_RAY_START_CMD`，允许用户完全接管容器命令。
bootstrap 脚本完成 gVisor 安装与 cgroup 修复后，再通过 `exec /bin/bash -lc "${KUBERAY_GEN_RAY_START_CMD}"` 启动 Ray。
2. cgroup 修复逻辑：快照 `/proc/self/cgroup`，将各控制器重新挂载至 `/mnt/cgreal/<名称>`，创建 `rsclean` 子目录并把当前进程迁入，使进程脱离冒号路径。
该逻辑必须在与 Ray 启动命令相同的 shell 进程中执行，进程迁移才对其子进程生效。
3. RayJob 的 `entrypoint` 会被 KubeRay 提交器拼入一条更长的 shell 命令，多行入口点（尤其是 heredoc）在拼接后会被破坏。
因此将演示脚本置于 ConfigMap，入口点保持单行命令 `python3 /opt/ray-sandbox/entrypoint.py`。
4. 沙箱参数 `rootless: false` 启用 rootful 模式，`cpu` 与 `memory` 限额生效。

操作步骤：

1. 将以下内容保存为 `rayjob-rund-rootful.yaml`：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ray-sandbox-bootstrap
  namespace: ray-sandbox
data:
  bootstrap.sh: |
    #!/bin/bash
    set -euo pipefail

    GVISOR_VERSION=20260921.0
    URL=https://storage.googleapis.com/gvisor/releases/release/${GVISOR_VERSION}/$(uname -m)
    cd /tmp
    wget -q ${URL}/gvisor.tar.zstd ${URL}/gvisor.tar.zstd.sha512
    sha512sum -c gvisor.tar.zstd.sha512
    zstd -dc gvisor.tar.zstd | tar -x -C /usr/local/bin runsc gvisor-bin
    rm -f gvisor.tar.zstd gvisor.tar.zstd.sha512

    SNAP=/tmp/cg.snap
    CLEAN=rsclean
    REAL=/mnt/cgreal
    cat /proc/self/cgroup > "${SNAP}"
    mkdir -p "${REAL}"

    moved=0
    while IFS= read -r line; do
      rest="${line#*:}"
      ctrl="${rest%%:*}"
      [ -z "${ctrl:-}" ] && continue
      name="${ctrl//,/_}"
      name="${name//name=/sysd_}"
      mp="${REAL}/${name}"
      mkdir -p "${mp}"

      if ! mountpoint -q "${mp}"; then
        if [ "${ctrl#name=}" != "${ctrl}" ]; then
          mount -t cgroup -o "none,name=${ctrl#name=}" cgroup "${mp}" || continue
        else
          mount -t cgroup -o "${ctrl}" cgroup "${mp}" || continue
        fi
      fi

      mkdir -p "${mp}/${CLEAN}" || continue
      if [ -f "${mp}/${CLEAN}/cpuset.cpus" ]; then
        cat "${mp}/cpuset.cpus" > "${mp}/${CLEAN}/cpuset.cpus"
        cat "${mp}/cpuset.mems" > "${mp}/${CLEAN}/cpuset.mems"
      fi
      echo $$ > "${mp}/${CLEAN}/cgroup.procs" && moved=$((moved + 1))
    done < "${SNAP}"

    residual=$(awk -F: 'NF>3 {c++} END {print c+0}' /proc/self/cgroup)
    echo "cgroup controllers moved=${moved}, residual=${residual}"
    [ "${residual}" -eq 0 ]
    exec /bin/bash -lc "${KUBERAY_GEN_RAY_START_CMD}"
  entrypoint.py: |
    import ray
    from ray.experimental import sandbox

    ray.init()
    sb = sandbox.create(
        image="python:3.12-slim",
        workdir="/workspace",
        cpu=0.5,
        memory="256Mi",
        network="none",
        rootless=False,
    )

    result = ray.get(sb.exec.remote([
        "python3",
        "-c",
        "import os, platform; "
        "print('Platform:', platform.platform()); "
        "print('CPU count:', os.cpu_count()); "
        "print(open('/proc/meminfo').readline().strip())",
    ]))
    print(f"Exit code: {result.exit_code}")
    print(result.stdout)
    ray.get(sb.delete.remote())
---
apiVersion: ray.io/v1
kind: RayJob
metadata:
  name: rayjob-sandbox-rund
  namespace: ray-sandbox
spec:
  entrypoint: python3 /opt/ray-sandbox/entrypoint.py
  shutdownAfterJobFinishes: true
  ttlSecondsAfterFinished: 600
  rayClusterSpec:
    rayVersion: '2.58.0'
    headGroupSpec:
      rayStartParams:
        dashboard-host: '0.0.0.0'
      template:
        metadata:
          annotations:
            ray.io/overwrite-container-cmd: "true"
        spec:
          runtimeClassName: sandbox
          nodeSelector:
            ray-sandbox: rund
          tolerations:
          - key: ray-sandbox
            operator: Equal
            value: rund
            effect: NoSchedule
          containers:
          - name: ray-head
            image: rayproject/ray:2.58.0-py312
            command: ["/opt/ray-sandbox/bootstrap.sh"]
            securityContext:
              privileged: true
              runAsUser: 0
              seccompProfile:
                type: Unconfined
              appArmorProfile:
                type: Unconfined
            volumeMounts:
            - name: bootstrap
              mountPath: /opt/ray-sandbox
            resources:
              requests:
                cpu: "2"
                memory: "8Gi"
              limits:
                cpu: "4"
                memory: "8Gi"
          volumes:
          - name: bootstrap
            configMap:
              name: ray-sandbox-bootstrap
              defaultMode: 0755
    workerGroupSpecs:
    - groupName: worker-group
      replicas: 1
      minReplicas: 1
      maxReplicas: 1
      rayStartParams: {}
      template:
        metadata:
          annotations:
            ray.io/overwrite-container-cmd: "true"
        spec:
          runtimeClassName: sandbox
          nodeSelector:
            ray-sandbox: rund
          tolerations:
          - key: ray-sandbox
            operator: Equal
            value: rund
            effect: NoSchedule
          containers:
          - name: ray-worker
            image: rayproject/ray:2.58.0-py312
            command: ["/opt/ray-sandbox/bootstrap.sh"]
            securityContext:
              privileged: true
              runAsUser: 0
              seccompProfile:
                type: Unconfined
              appArmorProfile:
                type: Unconfined
            volumeMounts:
            - name: bootstrap
              mountPath: /opt/ray-sandbox
            resources:
              requests:
                cpu: "2"
                memory: "8Gi"
              limits:
                cpu: "4"
                memory: "8Gi"
          volumes:
          - name: bootstrap
            configMap:
              name: ray-sandbox-bootstrap
              defaultMode: 0755
```

2. 部署并轮询：

```bash
kubectl apply --dry-run=client -f rayjob-rund-rootful.yaml
kubectl apply -f rayjob-rund-rootful.yaml
kubectl -n ray-sandbox get rayjob rayjob-sandbox-rund \
  -o jsonpath='{.status.jobDeploymentStatus}/{.status.jobStatus}'
```

3. 作业完成后查看提交器日志：

```bash
kubectl -n ray-sandbox logs <submitter Pod 名称>
```

预期结果与停止条件：

`jobDeploymentStatus` 为 `Complete` 且 `jobStatus` 为 `SUCCEEDED`，日志包含：

```text
Exit code: 0
Platform: Linux-4.19.0-gvisor-x86_64-with-glibc2.41
CPU count: 2
MemTotal:         262144 kB
```

`MemTotal` 为 262144 kB，即声明的 256 MiB 精确生效，表明 rootful 模式下内存限额已由 cgroup 落实。
`CPU count` 反映的是 CPU 亲和性掩码中的核数，0.5 核配额通过 cgroup CPU quota 生效，不改变亲和性，两者不矛盾。

若作业失败，请停止并参考故障排查章节。

## 生产实践
### 一、将 runsc 内置到 Ray 镜像
背景：

基础形态与进阶形态均在容器启动时从 gVisor 发布地址下载 runsc，存在外网依赖与版本漂移风险。生产环境建议将 runsc 在镜像构建阶段固化，并将镜像托管至阿里云容器镜像服务（ACR）。

以下环境变量用于本节操作：

```bash
export ACR_INSTANCE_ID=<ACR 企业版实例 ID>
export ACR_INSTANCE_NAME=<ACR 企业版实例名称>
export ACR_REGION=<ACR 实例所在地域>
export ACR_REGISTRY=${ACR_INSTANCE_NAME}-registry.${ACR_REGION}.cr.aliyuncs.com
```

操作步骤：

1. 准备 ACR 企业版实例的公网访问。开启公网端点：

```bash
aliyun cr UpdateInstanceEndpointStatus \
  --InstanceId ${ACR_INSTANCE_ID} \
  --EndpointType internet --Enable true \
  --region ${ACR_REGION}
```

公网端点启用完成后系统会写入默认白名单条目 `127.0.0.1/32`（拒绝全部来源），该条目会覆盖启用前添加的白名单，因此请在端点状态变为 RUNNING 之后再添加白名单。集群节点的出网地址为 NAT 网关弹性公网 IP，可按以下方式查询：

```bash
aliyun vpc DescribeNatGateways --RegionId ${REGION} --VpcId ${VPC_ID} \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); [print(i["IpAddress"]) for g in d["NatGateways"]["NatGateway"] for i in g["IpLists"]["IpList"]]'
```

添加白名单条目：

```bash
aliyun cr CreateInstanceEndpointAclPolicy \
  --InstanceId ${ACR_INSTANCE_ID} \
  --EndpointType internet \
  --Entry <NAT 弹性公网 IP>/32 \
  --Comment "ray-sandbox-guide egress" \
  --region ${ACR_REGION}
```

2. 创建公开命名空间（开启自动创建仓库）：

```bash
aliyun cr CreateNamespace \
  --InstanceId ${ACR_INSTANCE_ID} \
  --NamespaceName kuberay-guide \
  --AutoCreateRepo true \
  --DefaultRepoType PUBLIC \
  --region ${ACR_REGION}
```

3. 获取推送用临时令牌并写入集群 Secret。临时令牌有效期约 1 小时，过期后拉取将返回 401，需重新获取并更新 Secret；生产环境建议改用固定 Registry 密码或 ACR 免密组件 acr-credential-helper：

```bash
TOKEN=$(aliyun cr GetAuthorizationToken \
  --InstanceId ${ACR_INSTANCE_ID} --region ${ACR_REGION} \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["AuthorizationToken"])')
kubectl -n ray-sandbox create secret docker-registry ee-registry \
  --docker-server=${ACR_REGISTRY} \
  --docker-username=cr_temp_user \
  --docker-password="${TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -
unset TOKEN
```

4. 在集群内使用 kaniko 构建并推送镜像。将以下内容保存为 `kaniko.yaml` 并执行 `kubectl apply -f kaniko.yaml`：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kaniko-dockerfiles
  namespace: ray-sandbox
data:
  Dockerfile.rayrunsc: |
    FROM rayproject/ray:2.58.0-py312
    ARG GVISOR_VERSION=20260921.0
    USER root
    RUN set -e; \
        URL=https://storage.googleapis.com/gvisor/releases/release/${GVISOR_VERSION}/$(uname -m); \
        cd /tmp; \
        wget -q ${URL}/gvisor.tar.zstd ${URL}/gvisor.tar.zstd.sha512; \
        sha512sum -c gvisor.tar.zstd.sha512; \
        zstd -dc gvisor.tar.zstd | tar -x -C /usr/local/bin runsc gvisor-bin; \
        rm -f gvisor.tar.zstd gvisor.tar.zstd.sha512
    USER ray
---
apiVersion: batch/v1
kind: Job
metadata:
  name: kaniko-build-rayrunsc
  namespace: ray-sandbox
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 1800
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: kaniko
        image: gcr.io/kaniko-project/executor:v1.23.2
        args:
        - --dockerfile=/workspace/Dockerfile.rayrunsc
        - --context=dir:///workspace
        - --destination=${ACR_REGISTRY}/kuberay-guide/ray-runsc:2.58.0-py312-gvisor20260921.0
        volumeMounts:
        - name: workspace
          mountPath: /workspace
        - name: docker-config
          mountPath: /kaniko/.docker
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
      volumes:
      - name: workspace
        configMap:
          name: kaniko-dockerfiles
      - name: docker-config
        secret:
          secretName: ee-registry
          items:
          - key: .dockerconfigjson
            path: config.json
```

说明：上述 YAML 中 `--destination` 的 `${ACR_REGISTRY}` 需在应用前替换为实际取值。
若使用 envsubst，请指定变量名列表（例如 `envsubst '${ACR_REGISTRY}'`），避免误替换 Dockerfile 中的 `${GVISOR_VERSION}` 等构建期变量；也可以直接写入完整地址。

5. 确认推送完成：

```bash
kubectl -n ray-sandbox logs -l job-name=kaniko-build-rayrunsc --tail=3
```

日志出现 `Pushed ${ACR_REGISTRY}/kuberay-guide/ray-runsc@sha256:...` 即为成功。

#### 使用内置镜像
将 RayJob 中的容器镜像替换为 `${ACR_REGISTRY}/kuberay-guide/ray-runsc:2.58.0-py312-gvisor20260921.0`，删除 postStart 钩子，并为 Pod 配置 `imagePullSecrets`：

- head 与 worker 的 Pod 模板中添加 `imagePullSecrets: [{name: ee-registry}]`。
- KubeRay 的作业提交器（submitter）默认沿用 head 镜像，但不继承 rayClusterSpec 中的 `imagePullSecrets`。
需要在 RayJob 中显式提供 `submitterPodTemplate`，且该模板必须包含 `restartPolicy: Never` 与名为 `ray-job-submitter` 的容器定义，否则提交器 Job 将因校验失败而无法创建：

```yaml
spec:
  submitterPodTemplate:
    spec:
      restartPolicy: Never
      imagePullSecrets:
      - name: ee-registry
      containers:
      - name: ray-job-submitter
        image: <ACR_REGISTRY>/kuberay-guide/ray-runsc:2.58.0-py312-gvisor20260921.0
```

以上配置已实测验证：head、worker 与 submitter 均可从 ACR 企业版实例拉取镜像并完成作业。

### 二、镜像托管与拉取方式对照
Ray Sandbox 链路中存在两类镜像消费者，其拉取机制与对仓库的要求不同：

| 消费者 | 拉取机制 | Docker Hub 公共仓库 | ACR 公开仓库 | ACR 私有仓库 |
| --- | --- | --- | --- | --- |
| Ray 节点镜像（kubelet/containerd） | containerd，支持 imagePullSecrets | 支持（实测） | 支持（配合 imagePullSecrets，实测） | 支持（配合 imagePullSecrets） |
| 沙箱镜像（Ray 拉取器） | Registry v2 HTTP 接口 + 匿名 Bearer 令牌，不支持凭据 | 支持（实测） | 需匿名拉取能力：个人版公开仓库默认支持；企业版需在控制台实例概览页开启「公开匿名拉取」 | 不支持（Ray 2.58.0） |

实测记录：在企业版实例未开启「公开匿名拉取」的情况下，匿名拉取公开仓库返回 `401 UNAUTHORIZED`（令牌可签发但不含 pull 权限）。
企业版匿名拉取开关的位置参见 ACR 文档「为什么在企业版实例匿名拉取镜像会失败」。
Ray 2.58.0 的镜像拉取器（`ray.experimental.sandbox` 内部实现）仅执行匿名令牌交换，未提供任何凭据配置入口，因此无法拉取任何需要认证的仓库；如需使用私有仓库中的沙箱镜像，请采用下节的预置方式。

### 三、预置沙箱镜像 tar
背景：

当沙箱镜像位于私有仓库或希望完全消除运行时外网依赖时，可将镜像扁平化为 tar 归档预置到节点，再将 `sandbox.create` 的 `image` 参数指向节点上的 tar 路径。Ray 会将该 tar 直接解压为沙箱根文件系统。

注意：tar 模式下镜像不携带镜像配置（环境变量、工作目录），Ray 生成的 OCI 配置中 `env` 为空数组，将导致沙箱内 `PATH` 为空、守护进程 `sleep` 无法找到。必须在 `sandbox.create` 中显式注入 `PATH`。

操作步骤：

1. 使用 crane 将镜像扁平化导出到节点目录。将以下内容保存为 `crane-export.yaml` 并执行 `kubectl apply -f crane-export.yaml`：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: crane-export-python
  namespace: ray-sandbox
spec:
  backoffLimit: 1
  ttlSecondsAfterFinished: 1800
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        ray-sandbox: gvisor
      tolerations:
      - key: ray-sandbox
        operator: Equal
        value: gvisor
        effect: NoSchedule
      containers:
      - name: crane
        image: gcr.io/go-containerregistry/crane:v0.21.9
        args: ["export", "python:3.12-slim", "/images/python-3.12-slim.tar"]
        securityContext:
          runAsUser: 0
        volumeMounts:
        - name: images
          mountPath: /images
        resources:
          requests:
            cpu: "0.5"
            memory: "512Mi"
          limits:
            cpu: "1"
            memory: "1Gi"
      volumes:
      - name: images
        hostPath:
          path: /var/lib/ray-sandbox-images
          type: DirectoryOrCreate
```

2. 在 RayJob 的 head 与 worker Pod 模板中将该目录挂载进容器，并将沙箱镜像指向 tar 路径，同时注入 `PATH`：

```yaml
          containers:
          - name: ray-head
            volumeMounts:
            - name: sandbox-images
              mountPath: /opt/sandbox-images
          volumes:
          - name: sandbox-images
            hostPath:
              path: /var/lib/ray-sandbox-images
              type: Directory
```

```python
sb = sandbox.create(
    image='/opt/sandbox-images/python-3.12-slim.tar',
    workdir='/workspace',
    cpu=1.0,
    memory='1Gi',
    env={'PATH': '/usr/local/bin:/usr/bin:/bin'},
)
```

以上方式已实测验证通过。多节点环境下需保证 tar 文件在每个承载 Ray 容器的节点上均存在，可改用 DaemonSet 或节点镜像烘焙方式分发。

## 故障排查
### runsc 启动失败：`sidecar "gvisor_sentry" not usable ... --sidecar-usage-policy is set to STRICT`
原因：gVisor 自 20260831.0 起改为打包发布，runsc 以 STRICT 策略依赖同目录的 `gvisor-bin/gvisor_sentry`。
仅解压 runsc 单文件（包括沿用 Ray 官方样例中已失效的 `release/latest/.../runsc` 下载地址）会触发该错误。

处理：下载 `gvisor.tar.zstd` 并同时解压 `runsc` 与 `gvisor-bin` 两个成员，下载后执行 SHA-512 校验。

### 沙箱创建失败：用户命名空间相关报错
原因：rootless 模式要求宿主机 `user.max_user_namespaces` 不小于 15000，Alibaba Cloud Linux 3 节点镜像默认值为 0。

处理：通过节点池操作系统参数接口设置为 65536（见基础形态第二节），设置后对存量与新增节点均生效。

### 进阶形态 runsc 失败：cgroup 路径包含冒号
原因：rund guest 为 cgroup v1，进程默认处于 `kubepods.slice:...` 形式的 systemd 命名路径，runsc 无法在该类路径下管理 cgroup。

处理：使用进阶形态第四节中的 bootstrap 脚本，将各控制器重新挂载并迁移当前进程至规整路径后，再启动 Ray。修复逻辑必须与 Ray 启动命令处于同一 shell 进程。

### RayJob 提交器日志出现 SyntaxError 或 heredoc 终止符缺失
原因：KubeRay 提交器将 `entrypoint` 拼入一条更长的 shell 命令（前缀为健康检查与 `ray job submit`，后缀为 `ray job logs`）。多行入口点在拼接后引号层级被破坏；以 heredoc 结尾的入口点，其终止符行会被拼接内容污染而失效。

处理：将入口脚本放入 ConfigMap 并挂载到 Ray 容器，`entrypoint` 保持单行命令（如 `python3 /opt/ray-sandbox/entrypoint.py`）。

### RayJob 停滞于 Initializing：FailedToCreateRayJobSubmitter
查看事件：`kubectl -n ray-sandbox describe rayjob <名称>`。
若提示 `spec.template.spec.restartPolicy: Required value`，说明 `submitterPodTemplate` 缺少 `restartPolicy: Never`；
该模板还必须包含名为 `ray-job-submitter` 的容器定义。

### 拉取 ACR 企业版镜像返回 401
- 报错 `insufficient_scope: authorization failed`（未携带凭据的匿名拉取）：企业版实例未开启「公开匿名拉取」，请在控制台实例概览页开启，或为 Pod 配置 imagePullSecrets。
- 报错 `401 Unauthorized`（携带凭据）：临时令牌已过期（有效期约 1 小时），重新执行 GetAuthorizationToken 并更新 Secret；长期使用建议配置固定 Registry 密码或 acr-credential-helper。

### 添加 ACR 白名单后仍无法连通公网端点
公网端点启用完成时会写入默认条目 `127.0.0.1/32` 并覆盖既有白名单。请在端点状态为 RUNNING 后通过 `ListInstanceEndpoint` 确认白名单内容，再添加实际条目。

### 创建安全沙箱节点池失败：InvalidInstanceType.NotSupportCpuOptionsNestedVirtualization
所选实例规格不支持嵌套虚拟化。仅第九代 ECS 实例规格族中部分规格支持（实测 ecs.g9i.8xlarge 可用，ecs.g9i.2xlarge 不可用），请更换规格后重试。

### aliyun CLI 请求异常
- 报可用区或地域不匹配：cs 命令缺少显式 `--region`，默认使用 CLI 配置地域。
- 报 `InvalidProtocol.NeedSsl`：组件管理类接口要求 HTTPS，请附加 `--secure`。

## 清理资源
按以下顺序清理，避免残留计费资源：

1. 删除工作负载与命名空间：

```bash
kubectl delete namespace ray-sandbox
```

2. 删除节点池（先等待节点池内节点排空）：

```bash
aliyun cs DELETE /clusters/${CLUSTER_ID}/nodepools/${GV_NODEPOOL_ID} --region ${REGION}
aliyun cs DELETE /clusters/${CLUSTER_ID}/nodepools/${RUND_NODEPOOL_ID} --region ${REGION}
```

3. 删除集群：

```bash
aliyun cs DELETE /clusters/${CLUSTER_ID} --region ${REGION}
```

4. 清理 ACR 资源（按需）：

```bash
aliyun cr DeleteRepository --InstanceId ${ACR_INSTANCE_ID} --RepoId <仓库 ID> --region ${ACR_REGION}
aliyun cr DeleteNamespace --InstanceId ${ACR_INSTANCE_ID} --NamespaceName kuberay-guide --region ${ACR_REGION}
aliyun cr DeleteInstanceEndpointAclPolicy --InstanceId ${ACR_INSTANCE_ID} --EndpointType internet --Entry <条目> --region ${ACR_REGION}
aliyun cr UpdateInstanceEndpointStatus --InstanceId ${ACR_INSTANCE_ID} --EndpointType internet --Enable false --region ${ACR_REGION}
```

## 参考
- Ray 官方样例：`ray-operator/config/samples/ray-job.sandbox.yaml`
- gVisor 发布物：`https://storage.googleapis.com/gvisor/releases/release/`
- ACK OpenAPI：CreateCluster、CreateClusterNodePool、ModifyNodePoolNodeConfig、InstallClusterAddons
- ACR 文档：使用企业版实例推送和拉取镜像、Docker 登录/推送/拉取失败常见问题
