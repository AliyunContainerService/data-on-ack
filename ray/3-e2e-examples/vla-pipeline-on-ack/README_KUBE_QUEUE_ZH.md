# 在 ACK 上使用 ack-kube-queue 管理 VLA 任务

[English](./README_KUBE_QUEUE.md) | 简体中文

本文以第一视角视频转 LeRobot 数据集的 VLA 数据处理任务为例，介绍如何在 ACK 集群中使用 ack-kube-queue 对 RayJob 排队、按配额放行并查看执行结果。VLA 算子、模型、镜像和 OSS 挂载的准备方法沿用 [VLA 管线指南](https://github.com/AliyunContainerService/data-on-ack/blob/main/ray/3-e2e-examples/vla-pipeline-on-ack/README_ZH.md)。本文重点说明任务队列的接入过程。

## 整体流程

1. 准备 ACK 集群、GPU 节点、VLA 镜像、模型和数据。
2. 在 ACK 控制台的云原生 AI 套件页面安装 ack-kube-queue，并启用 RayJob 排队功能。
3. 为 VLA 命名空间配置 ElasticQuotaTree 叶子配额。
4. 以 `spec.suspend: true` 提交 RayJob，观察任务进入队列。
5. 提高可用配额，观察任务出队并执行，最后检查 LeRobot 产物。

## 前置条件

- 已创建支持 ack-kube-queue 的 ACK 集群，已安装托管 KubeRay Operator，并配置可访问集群的 `kubectl`。集群类型和版本要求以 [ack-kube-queue 组件说明](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/product-overview/ack-kube-queue)为准。
- 集群有至少一台可调度的 GPU 节点，并有足够的 CPU 和内存运行 Ray Head。示例 RayJob 请求合计 **8 CPU、32 GiB 内存、1 GPU**；实际节点还需给系统组件留出余量。
- 已按照 [VLA 管线指南](https://github.com/AliyunContainerService/data-on-ack/blob/main/ray/3-e2e-examples/vla-pipeline-on-ack/README_ZH.md)准备输入视频、MoGe、HaWoR、MANO 等模型，以及包含 Ray、Data-Juicer 和 VLA 依赖的镜像。镜像中的 Data-Juicer 版本须与业务脚本要求一致。
- 已准备所有 Ray Pod 均可读取的输入和代码 PVC，以及可写入的输出 PVC。输入和输出应使用不同的 OSS 前缀。私有镜像的拉取凭据应创建在 VLA 任务命名空间中；不要将 AccessKey 或令牌写入 RayJob YAML。
- 操作者具有在 ACK 控制台安装和配置云原生 AI 套件组件的权限，以及创建命名空间、ElasticQuotaTree 和 RayJob 所需的 Kubernetes RBAC 权限。

以下示例使用 `vla-demo` 命名空间。请将镜像和 PVC 名称替换为实际值。若集群已有 ElasticQuotaTree，应在现有树中增加叶子配额，不要创建第二棵树；ACK 集群目前仅支持 `kube-system` 中的一棵 ElasticQuotaTree。

## 步骤 1 —— 在 ACK 控制台安装 ack-kube-queue

1. 登录 [容器服务管理控制台](https://cs.console.aliyun.com/)，在左侧导航栏选择**集群列表**，进入目标集群。
2. 在集群左侧导航栏选择**应用 → 云原生 AI 套件**。
3. 若尚未部署云原生 AI 套件，在部署页面勾选 **Kube-Queue** 并完成部署；若已部署套件，在组件列表中找到 **ack-kube-queue**，单击**部署**。具体界面步骤参见 [使用任务队列 ack-kube-queue 管理 AI/ML 工作负载](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues)。
4. 在组件参数配置中启用 **RayJob** 排队功能。ack-kube-queue 默认仅开启原生 Job 排队；如果组件已安装，可通过该页面修改组件参数以启用 RayJob。

安装完成后，确认页面中的组件状态为**已部署**，并检查集群中的工作负载和 CRD：

```bash
kubectl -n kube-queue get deploy
kubectl get crd queueunits.scheduling.x-k8s.io
```

RayJob 还依赖托管 KubeRay Operator，安装方式参见 [在 ACK 中安装 KubeRay 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)。

## 步骤 2 —— 创建 VLA 命名空间和队列配额

```bash
kubectl create namespace vla-demo
```

下面的配额树只关联 `vla-demo`。初始叶子配额为 1 CPU、1 GiB，低于示例 RayJob 的请求，用于观察排队。已有配额树的集群应由管理员将同样的叶子配置合并到现有树中，并按集群总资源调整根节点配额。

```yaml
# vla-quota.yaml
apiVersion: scheduling.sigs.k8s.io/v1beta1
kind: ElasticQuotaTree
metadata:
  name: vla-demo
  namespace: kube-system
spec:
  root:
    name: root
    max:
      cpu: "16"
      memory: 64Gi
      nvidia.com/gpu: "1"
    min:
      cpu: "16"
      memory: 64Gi
      nvidia.com/gpu: "1"
    children:
      - name: root.vla-demo
        namespaces: [vla-demo]
        max:
          cpu: "1"
          memory: 1Gi
        min:
          cpu: "1"
          memory: 1Gi
```

```bash
kubectl apply -f vla-quota.yaml
kubectl get elasticquotatree -n kube-system
kubectl get queue -A
```

ack-kube-queue 会依据 ElasticQuotaTree 的叶子节点创建队列，并将该命名空间中的 RayJob 路由到对应队列。详见 [ElasticQuotaTree 与任务队列说明](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/improve-resource-utilization-with-elasticquotatree-and-task-queues)。

## 步骤 3 —— 提交 VLA RayJob

先按 [VLA 管线指南](https://github.com/AliyunContainerService/data-on-ack/blob/main/ray/3-e2e-examples/vla-pipeline-on-ack/README_ZH.md)构建镜像并部署输入、输出 PVC。镜像应包含与脚本匹配的 Data-Juicer 版本。**本例只有 1 个 GPU Worker**，因此还需把指南中的业务脚本调为单样本、单 GPU 配置：`LIMIT=1`、`GPU_BATCH_SIZE=1`、`MOGE_ACTOR_MIN=MOGE_ACTOR_MAX=1`、`COMBINED_ACTOR_MIN=COMBINED_ACTOR_MAX=1`。保留 `MOGE_GPUS=0.25` 和 `COMBINED_GPUS=0.25`。否则指南默认的多个 GPU Actor 可能长期等待更多 GPU。将下例的镜像、PVC、Ray 版本和业务入口替换为自己的值。`/data/oss/code/vla_pipeline.py` 是实际运行视频处理与 LeRobot 导出的脚本。

```yaml
# vla-rayjob.yaml
apiVersion: ray.io/v1
kind: RayJob
metadata:
  name: vla-egodex-demo
  namespace: vla-demo
spec:
  suspend: true
  submissionMode: K8sJobMode
  entrypoint: python /data/oss/code/vla_pipeline.py
  shutdownAfterJobFinishes: true
  ttlSecondsAfterFinished: 3600
  submitterPodTemplate:
    spec:
      restartPolicy: Never
      imagePullSecrets:
        - name: vla-acr-pull
      containers:
        - name: ray-job-submitter
          image: <ACR_REGISTRY>/<NAMESPACE>/<VLA_IMAGE>:<TAG>
  rayClusterSpec:
    rayVersion: "2.56.0" # 与镜像中的 Ray 版本一致
    headGroupSpec:
      rayStartParams:
        num-cpus: "0"
        dashboard-host: "0.0.0.0"
      template:
        spec:
          imagePullSecrets:
            - name: vla-acr-pull
          containers:
            - name: ray-head
              image: <ACR_REGISTRY>/<NAMESPACE>/<VLA_IMAGE>:<TAG>
              resources:
                requests: {cpu: "2", memory: 8Gi}
                limits: {cpu: "3", memory: 12Gi}
              env:
                - name: PYTHONPATH
                  value: /data/oss/code
              volumeMounts:
                - {name: input, mountPath: /data/oss, readOnly: true}
                - {name: output, mountPath: /data/output}
          volumes:
            - name: input
              persistentVolumeClaim: {claimName: vla-input-pvc, readOnly: true}
            - name: output
              persistentVolumeClaim: {claimName: vla-output-pvc}
    workerGroupSpecs:
      - groupName: gpu-worker
        replicas: 1
        minReplicas: 1
        maxReplicas: 1
        rayStartParams:
          num-cpus: "6"
        template:
          spec:
            imagePullSecrets:
              - name: vla-acr-pull
            containers:
              - name: ray-worker
                image: <ACR_REGISTRY>/<NAMESPACE>/<VLA_IMAGE>:<TAG>
                resources:
                  requests: {cpu: "6", memory: 24Gi, nvidia.com/gpu: "1"}
                  limits: {cpu: "6", memory: 28Gi, nvidia.com/gpu: "1"}
                env:
                  - name: PYTHONPATH
                    value: /data/oss/code
                volumeMounts:
                  - {name: input, mountPath: /data/oss, readOnly: true}
                  - {name: output, mountPath: /data/output}
            volumes:
              - name: input
                persistentVolumeClaim: {claimName: vla-input-pvc, readOnly: true}
              - name: output
                persistentVolumeClaim: {claimName: vla-output-pvc}
```

```bash
kubectl apply -f vla-rayjob.yaml
kubectl get rayjob vla-egodex-demo -n vla-demo
kubectl get queueunit -n vla-demo
kubectl describe queueunit vla-egodex-demo-ray-qu -n vla-demo
```

此时 RayJob 请求 8 CPU、32 GiB 和 1 GPU，超过叶子配额。预期 QueueUnit 保持等待状态，`status.message` 中出现 `Insufficient quota`，而 RayJob 的 `spec.suspend` 仍为 `true`。QueueUnit 名称由组件生成；如果与示例不同，以 `kubectl get queueunit -n vla-demo` 的实际结果为准。

## 步骤 4 —— 提高配额并观察任务执行

将 `vla-quota.yaml` 中叶子节点的 `max` 和 `min` 改为以下值，根节点配额保持不变，然后重新应用：

```yaml
max:
  cpu: "16"
  memory: 64Gi
  nvidia.com/gpu: "1"
min:
  cpu: "16"
  memory: 64Gi
  nvidia.com/gpu: "1"
```

```bash
kubectl apply -f vla-quota.yaml
kubectl get queueunit -n vla-demo -w
kubectl get rayjob vla-egodex-demo -n vla-demo -w
kubectl get pods -n vla-demo -o wide
```

预期 QueueUnit 转为 `Dequeued`，ack-kube-queue 将 RayJob 的 `spec.suspend` 改为 `false`，KubeRay 随后创建 Ray Head、GPU Worker 和提交器 Pod。RayJob 最终应显示 `jobDeploymentStatus: Complete` 与 `jobStatus: SUCCEEDED`。如果集群只有一台 8 CPU GPU 节点，应通过 `headGroupSpec.template.spec.nodeSelector` 或节点亲和性将 Ray Head 放到 CPU 节点，为 GPU Worker 留足 CPU。

## 步骤 5 —— 查看日志和产物

```bash
kubectl get rayjob vla-egodex-demo -n vla-demo \
  -o jsonpath='{.status.jobDeploymentStatus}{" "}{.status.jobStatus}{"\n"}'
kubectl logs -n vla-demo job/vla-egodex-demo --tail=100
```

成功时，业务日志会显示处理的输入行数、输出行数和 LeRobot 数据集目录。检查输出 PVC 对应的 OSS 前缀，确认存在 `lerobot_dataset/data/`、`meta/` 和 `videos/` 下的 episode 文件。示例的实际 VLA 任务会依次执行视频抽帧、MoGe 相机标定、HaWoR/MegaSAM 手部与场景重建、动作计算、字幕生成及 LeRobot 导出；具体输出内容由输入视频和脚本参数决定。

## 常见问题

| 现象 | 排查方向 |
| --- | --- |
| 没有 QueueUnit | 确认已开启 RayJob 扩展、RayJob 设置了 `spec.suspend: true`，并检查命名空间是否关联到 ElasticQuotaTree 叶子节点。 |
| QueueUnit 长时间等待 | 查看 `kubectl describe queueunit` 的消息，确认 CPU、内存、GPU 请求均不超过可用配额。 |
| 已出队但 Pod 仍 Pending | 检查节点剩余资源、GPU 插件、节点亲和性和存储卷事件；配额放行不保证节点一定能容纳 Pod。 |
| `ImagePullBackOff` 或 401 | 检查镜像地址和该命名空间的拉取凭据是否能访问目标 ACR 仓库。 |
| 入口脚本报依赖版本不匹配 | 对齐镜像中的 Ray、Data-Juicer 和 VLA 脚本版本后重新构建镜像。 |

## 参考资料

- [VLA 管线 Demo](https://github.com/AliyunContainerService/data-on-ack/blob/main/ray/3-e2e-examples/vla-pipeline-on-ack/README_ZH.md)
- [使用 ack-kube-queue 管理 AI/ML 工作负载](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues)
- [ElasticQuotaTree](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/elasticquotatree)
