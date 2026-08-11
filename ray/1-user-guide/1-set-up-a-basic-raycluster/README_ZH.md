# 部署并使用基础 RayCluster

本文介绍如何在 ACK 集群上通过 KubeRay 部署一个基础 RayCluster（1 个 head + 1 个 worker），并运行一个简单的 Ray Actor 示例程序。

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)

## 1. 部署 RayCluster

```bash
kubectl apply -f raycluster.yaml
```

预期输出：

```
raycluster.ray.io/demo-ray-cluster created
```

等待 Pod 就绪：

```bash
kubectl get pods
```

预期输出（1 个 head Pod 和 1 个 worker Pod 均为 Running）：

```
NAME                                          READY   STATUS    RESTARTS   AGE
demo-ray-cluster-head-mgdnb                   1/1     Running   0          2m16s
demo-ray-cluster-worker-group-worker-lvbr2    1/1     Running   0          2m16s
```

## 2. 查看 Ray 集群状态

登录 head Pod，通过 `ray status` 查看集群状态：

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $HEAD_POD -- ray status
```

预期输出（head 节点 `num-cpus` 设为 0 不承担计算任务，集群共 2 个可用 CPU）：

```
======== Autoscaler status: 2026-08-10 06:08:43.387414 ========
Node status
---------------------------------------------------------------
Active:
 (no active nodes)
Idle:
 1 worker-group
 1 headgroup
Pending:
 (no pending nodes)
Recent failures:
 (no failures)

Resources
---------------------------------------------------------------
Total Usage:
 0.0/2.0 CPU
 0B/6.00GiB memory
 0B/1.52GiB object_store_memory

From request_resources:
 (none)
Pending Demands:
 (no resource demands)
```

## 3. 运行 Ray 程序示例

将本目录下的 `ray_actor_rand_and_sum.py` 拷贝到 head Pod 中运行：

```bash
kubectl cp ray_actor_rand_and_sum.py $HEAD_POD:/tmp/ray_actor_rand_and_sum.py
kubectl exec -it $HEAD_POD -- python /tmp/ray_actor_rand_and_sum.py
```

脚本通过 `ray.init(address="auto")` 自动连接本机 Ray 集群，由 `RandIntActor` 产生一个 1~100 的随机数，再由 `AddActor` 将其加 5 返回。预期输出（随机数每次运行不同）：

```
Random number: 61
Final result: 66
```

## 清理

```bash
kubectl delete -f raycluster.yaml
```
