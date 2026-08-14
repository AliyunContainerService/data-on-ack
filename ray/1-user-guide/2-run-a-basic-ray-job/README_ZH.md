# 运行基础 Ray Job

本文介绍如何在 ACK 集群上通过 KubeRay 运行 Ray Job，包含两种方式：

1. 向一个**已存在的 RayCluster** 提交 Ray Job
2. **新建 RayCluster** 运行 Ray Job（任务结束后集群自动清理）

示例代码通过 Kubernetes ConfigMap 保存并挂载到 Ray Pod 中，无需重新构建镜像。

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)

## 1. 将示例代码存入 ConfigMap

Ray Job 的入口命令是 `python /home/ray/job/ray_job_sample.py`。脚本通过 `ray.init(address="auto")` 自动连接 Ray 集群，用 Ray 远程任务计算 `[1, 2, 3, 4, 5]` 的平方，再用 Ray Actor 求和。

直接从本地文件创建 ConfigMap（data key 为文件名 `ray_job_sample.py`）：

```bash
kubectl create configmap ray-job-code --from-file=ray_job_sample.py
```

预期输出：

```
configmap/ray-job-code created
```

ConfigMap 挂载到 Ray Pod 的 `/home/ray/job`，因此脚本位于 `/home/ray/job/ray_job_sample.py`。

## 2. 方法一：向已存在的 RayCluster 提交 Ray Job

`ray-job-existing-cluster.yaml` 包含两部分：

1. 名为 `demo-ray-cluster` 的 RayCluster，示例代码 ConfigMap 挂载在 head Pod 的 `/home/ray/job`，并带有标签 `ray.io/cluster: demo-ray-cluster`
2. 一个通过 `clusterSelector` 定位该集群的 RayJob（不会新建 RayCluster）

如果你已有运行中的 RayCluster，只保留 RayJob 部分即可，但需要确保 ConfigMap 已挂载到其 head Pod，且集群带有 `clusterSelector` 使用的标签。

```bash
kubectl apply -f ray-job-existing-cluster.yaml
```

预期输出：

```
raycluster.ray.io/demo-ray-cluster created
rayjob.ray.io/rayjob-existing-cluster created
```

RayJob 会先处于 `Initializing` 状态等待集群就绪，然后提交任务：

```bash
kubectl get rayjob rayjob-existing-cluster
```

预期输出（Job ID 每次运行不同）：

```
NAME                      JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME   START TIME             END TIME               AGE
rayjob-existing-cluster   SUCCEEDED    Complete            demo-ray-cluster   2026-08-14T07:43:21Z   2026-08-14T07:45:23Z   2m3s
```

通过 submitter Pod 日志查看任务输出：

```bash
kubectl logs job/rayjob-existing-cluster
```

预期输出（节选）：

```
numbers: [1, 2, 3, 4, 5]
squares: [1, 4, 9, 16, 25]
total: 55
Job 'rayjob-existing-cluster-bq5pj' succeeded
```

由于 `shutdownAfterJobFinishes` 为 `false`，任务结束后已有集群继续运行。

## 3. 方法二：新建 RayCluster 运行 Ray Job

`ray-job-new-cluster.yaml` 将整个集群定义内嵌在 RayJob 中（`rayClusterSpec`）。Operator 会创建一个以 RayJob 命名的 RayCluster（1 个 head + 1 个 worker），提交任务，并在任务结束后自动关闭集群。

```bash
kubectl apply -f ray-job-new-cluster.yaml
```

预期输出：

```
rayjob.ray.io/rayjob-new-cluster created
```

集群创建过程中 RayJob 处于 `Initializing` 状态：

```bash
kubectl get rayjob rayjob-new-cluster
```

预期输出：

```
NAME                 JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME           START TIME             END TIME   AGE
rayjob-new-cluster                Initializing        rayjob-new-cluster-4jw7m   2026-08-14T07:45:49Z              5s
```

等待任务完成并查看结果：

```bash
kubectl wait --for=condition=complete job/rayjob-new-cluster --timeout=300s
kubectl get rayjob rayjob-new-cluster
```

预期输出：

```
NAME                 JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME           START TIME             END TIME               AGE
rayjob-new-cluster   SUCCEEDED    Complete            rayjob-new-cluster-4jw7m   2026-08-14T07:45:49Z   2026-08-14T07:46:25Z   37s
```

由于 `shutdownAfterJobFinishes` 为 `true`，任务成功后 RayCluster 的 Pod 会被自动删除：

```bash
kubectl get pods -l ray.io/cluster=rayjob-new-cluster-4jw7m
```

预期输出：

```
No resources found in default namespace.
```

## 4. 查看 Ray Job 运行状态

查看所有 RayJob 及其状态：

```bash
kubectl get rayjobs
```

查看单个 RayJob 的详细信息（Dashboard 地址、提交 ID、集群名等）：

```bash
kubectl describe rayjob rayjob-existing-cluster
```

```
Status:
  Dashboard URL:         demo-ray-cluster-head-svc.default.svc.cluster.local:8265
  Job Deployment Status: Complete
  Job Id:                rayjob-existing-cluster-bq5pj
  Job Status:            SUCCEEDED
  Message:               Job finished successfully.
  Ray Cluster Name:      demo-ray-cluster
```

从 Ray 集群内部（head Pod）查看任务：

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head -l ray.io/cluster=demo-ray-cluster -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $HEAD_POD -- ray job list
kubectl exec -it $HEAD_POD -- ray job logs rayjob-existing-cluster-bq5pj
```

## 清理

```bash
kubectl delete -f ray-job-existing-cluster.yaml
kubectl delete -f ray-job-new-cluster.yaml
kubectl delete configmap ray-job-code
```
