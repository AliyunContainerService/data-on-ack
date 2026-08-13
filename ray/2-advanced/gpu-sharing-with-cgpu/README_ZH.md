# 在 ACK 上从 Ray 使用 cGPU 共享显卡

[cGPU](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/product-overview/ack-ai-installer)
是阿里云的容器 GPU 共享技术:多个 Pod 共享一块物理 GPU,由内核模块把每个 Pod 的显存(以及可选的算力)限制在一个切片内,互不挤占。在 ACK 上,共享 GPU 通过扩展资源 `aliyun.com/gpu-mem`(单位 GiB)申请,而不是整卡的 `nvidia.com/gpu`。

Ray 并不认识 `aliyun.com/gpu-mem`。本文给出让 KubeRay 的 worker 落到 cGPU 切片、并把它作为一块普通 GPU 暴露给 Ray 所需的那个关键改动,以及如何验证隔离确实生效。

已在 ACK Pro(K8s 1.34)+ 一台 `ecs.gn7i-c8g1.2xlarge`(NVIDIA A10,22 GiB)上实测,把 8 GiB 切片给到一个 Ray worker。

## 前提条件

- ACK **Pro** 托管集群(托管的 ACK 调度器原生支持 `aliyun.com/gpu-mem` 调度,不需要额外的 scheduler extender)。
- 已安装**云原生 AI 套件**(`ack-ai-installer`),它会部署 `gpushare-device-plugin` 和 `cgpu-installer`。**只能从 ACK 控制台/Helm 安装**——`aliyun cs` OpenAPI 会以 `AddonNotFound` 拒绝。
- KubeRay operator。在 ACK 上它是托管组件(`kuberay-operator`),在管控侧完成 reconcile,你不需要在自己的 namespace 里跑 operator。
- GPU 节点的实例族需被 cGPU 支持(T4/A10/L 系列等);消费级显卡一般不支持。

给每个要共享的 GPU 节点打标签,让 AI 套件的 DaemonSet 调度上去并启用 cGPU:

```bash
kubectl label node <gpu-node> ack.node.gpu.schedule=cgpu
```

确认切片资源已暴露(单位 GiB):

```bash
kubectl get node <gpu-node> -o jsonpath='{.status.allocatable.aliyun\.com/gpu-mem}'
# 例如 22 (A10 有 22 GiB 可分配共享显存)
```

## 最容易踩的坑

KubeRay **只**从容器的 `nvidia.com/gpu` limit 推导 Ray 的 `--num-gpus`。而 cGPU worker 申请的是 `aliyun.com/gpu-mem`,于是 KubeRay 算出 0 块 GPU,Ray 以 `num-gpus=0` 启动,任何 `@ray.remote(num_gpus=1)` 的 actor 会一直 Pending。

解决办法是在该 worker group 的 `rayStartParams` 里自己声明 GPU 数量:

```yaml
workerGroupSpecs:
- groupName: cgpu-worker
  rayStartParams:
    num-gpus: "1"                 # KubeRay 不会从 gpu-mem 推断，必须手写
  template:
    spec:
      containers:
      - name: ray-worker
        resources:
          limits:
            aliyun.com/gpu-mem: "8"   # 8 GiB 共享切片，而非整卡
```

`CUDA_VISIBLE_DEVICES` 由 gpushare-device-plugin 注入,cGPU 内核模块把该 Pod 可见的显存限制到申请值——所以在 Ray 里它表现得就像一块小号独占 GPU。

## 1. 部署 RayCluster

[`ray-cluster-cgpu.yaml`](ray-cluster-cgpu.yaml) 是一个纯 CPU head + 一个申请 8 GiB 切片、并声明了 `num-gpus: "1"` 的 worker。镜像和 `imagePullSecrets` 换成你自己的。

```bash
kubectl apply -f ray-cluster-cgpu.yaml
kubectl get pod -l ray.io/cluster=cgpu-ray -o wide
```

```
NAME                                READY   STATUS    NODE
cgpu-ray-head-xxxxx                 1/1     Running   cn-hangzhou.10.200.12.34
cgpu-ray-cgpu-worker-worker-xxxxx   1/1     Running   cn-hangzhou.10.200.12.34
```

进 worker 看显存上限——`nvidia-smi` 应显示切片大小而非整卡:

```bash
WORKER=$(kubectl get pod -l ray.io/cluster=cgpu-ray,ray.io/group=cgpu-worker -o jsonpath='{.items[0].metadata.name}')
kubectl exec $WORKER -c ray-worker -- bash -lc 'env | grep ALIYUN_COM_GPU_MEM; nvidia-smi --query-gpu=memory.total --format=csv'
```

```
ALIYUN_COM_GPU_MEM_CONTAINER=8
ALIYUN_COM_GPU_MEM_DEV=22
ALIYUN_COM_GPU_MEM_UNIT=GiB
memory.total [MiB]
8373 MiB          <- 被限到 ~8 GiB，而非 22 GiB
```

Ray 也正好看到一块 GPU:

```bash
HEAD=$(kubectl get pod -l ray.io/cluster=cgpu-ray,ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec $HEAD -c ray-head -- ray status | grep GPU
# 0.0/1.0 GPU
```

## 2. 从 Ray 验证

[`cgpu_ray_test.py`](cgpu_ray_test.py) 起一个 GPU actor,报告它拿到的设备,然后分别在切片内、切片外分配显存。

```bash
kubectl cp cgpu_ray_test.py $HEAD:/tmp/cgpu_ray_test.py -c ray-head
kubectl exec $HEAD -c ray-head -- python /tmp/cgpu_ray_test.py
```

```
cluster GPU: 1.0
actor info: {'ray_gpu_ids': [0], 'cuda_visible_devices': '0', 'device': 'NVIDIA A10', 'total_mem_GiB': 8.18}
alloc 4GiB : {'requested_GiB': 4, 'free_GiB': 3.95, 'total_GiB': 8.18}
alloc 16GiB: blocked as expected -> RayTaskError(OutOfMemoryError)
OK
```

actor 的设备报告 **总显存 8.18 GiB**(切片,而非 A10 的 22 GiB),4 GiB 张量能分配,16 GiB 张量被 OOM 拒绝——cGPU 的显存上限在 Ray 内部确实被强制执行。

## 说明

- **一个切片上塞多个 actor**:在 Pod 级别申请一次切片(`aliyun.com/gpu-mem`),保持 `num-gpus: "1"`,每个 actor 用小数份额 `@ray.remote(num_gpus=0.5)`。Ray 的小数是 Pod 内的逻辑记账;硬显存上限是这个 Pod 内所有 actor 共享的那块 cGPU 切片。
- **多个 Pod 共享一块物理 GPU**:把多个 cGPU worker Pod(各自申请切片)调度到同一节点;gpushare 调度器按 `aliyun.com/gpu-mem` 打包,cGPU 逐个隔离。它们可以属于不同的 RayCluster。
- **算力隔离**:把节点标签改成 `ack.node.gpu.schedule=core_mem`(而非 `cgpu`)会额外暴露 `aliyun.com/gpu-core.percentage` 做算力份额限制;Ray 侧接法完全一致。
- **同集群里的整卡 worker** 仍用 `nvidia.com/gpu`,不需要 `num-gpus` 覆盖——KubeRay 会正常推断。

## 清理

```bash
kubectl delete -f ray-cluster-cgpu.yaml
```
