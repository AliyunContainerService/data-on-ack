# 在 ACK 上从 Ray 使用 cGPU 共享显卡

[cGPU](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/product-overview/ack-ai-installer)
是阿里云的容器 GPU 共享技术:多个 Pod 共享一块物理 GPU,由内核模块把每个 Pod 的显存(以及可选的算力)限制在一个切片内,互不挤占。在 ACK 上,共享 GPU 通过扩展资源 `aliyun.com/gpu-mem`(单位 GiB)申请,而不是整卡的 `nvidia.com/gpu`。

Ray 不识别 `aliyun.com/gpu-mem` 资源。本文说明如何使 KubeRay worker 调度到 cGPU 切片、并将该切片作为一块 GPU 暴露给 Ray,以及如何验证显存隔离。

本文内容在 ACK Pro(K8s 1.34)集群上验证,节点规格为 `ecs.gn7i-c8g1.2xlarge`(NVIDIA A10,22 GiB),将 8 GiB 切片分配给单个 Ray worker。

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

## 必须显式声明 GPU 数量

KubeRay **只**从容器的 `nvidia.com/gpu` limit 推导 Ray 的 `--num-gpus`。而 cGPU worker 申请的是 `aliyun.com/gpu-mem`,KubeRay 推导结果为 0,Ray 以 `num-gpus=0` 启动,任何 `@ray.remote(num_gpus=1)` 的 actor 都会一直 Pending。

因此需要在该 worker group 的 `rayStartParams` 中显式声明 GPU 数量:

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

`CUDA_VISIBLE_DEVICES` 由 gpushare-device-plugin 注入,cGPU 内核模块将该 Pod 可见的显存限制到申请值。对 Ray 而言,该切片等价于一块显存受限的独占 GPU。

## 1. 部署 RayCluster

[`ray-cluster-cgpu.yaml`](ray-cluster-cgpu.yaml) 是一个纯 CPU head 加一个申请 8 GiB 切片、并声明了 `num-gpus: "1"` 的 worker。请将镜像和 `imagePullSecrets` 替换为实际使用的值。

```bash
kubectl apply -f ray-cluster-cgpu.yaml
kubectl get pod -l ray.io/cluster=cgpu-ray -o wide
```

```
NAME                                READY   STATUS    NODE
cgpu-ray-head-xxxxx                 1/1     Running   cn-hangzhou.10.200.12.34
cgpu-ray-cgpu-worker-worker-xxxxx   1/1     Running   cn-hangzhou.10.200.12.34
```

进入 worker 容器查看显存上限——`nvidia-smi` 应显示切片大小而非整卡:

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

Ray 识别到一块 GPU:

```bash
HEAD=$(kubectl get pod -l ray.io/cluster=cgpu-ray,ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec $HEAD -c ray-head -- ray status | grep GPU
# 0.0/1.0 GPU
```

## 2. 从 Ray 验证

[`cgpu_ray_test.py`](cgpu_ray_test.py) 启动一个 GPU actor,报告其使用的设备,并分别在切片容量内、外尝试分配显存。

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

actor 的设备报告 **总显存 8.18 GiB**(切片,而非 A10 的 22 GiB),4 GiB 张量分配成功,16 GiB 张量因超出切片容量返回 OOM。显存上限由 cGPU 强制执行。

## 说明

- **在单个切片上运行多个 actor**:在 Pod 级别申请一次切片(`aliyun.com/gpu-mem`),保持 `num-gpus: "1"`,每个 actor 使用小数份额 `@ray.remote(num_gpus=0.5)`。Ray 的小数份额仅用于 Pod 内的逻辑调度;实际显存上限为该 Pod 内所有 actor 共享的 cGPU 切片容量。
- **多个 Pod 共享一块物理 GPU**:将多个 cGPU worker Pod(各自申请切片)调度到同一节点;gpushare 调度器按 `aliyun.com/gpu-mem` 进行装箱,cGPU 对各 Pod 分别隔离。这些 Pod 可以属于不同的 RayCluster。
- **算力隔离**:将节点标签改为 `ack.node.gpu.schedule=core_mem`(而非 `cgpu`)会额外暴露 `aliyun.com/gpu-core.percentage` 用于算力份额限制;Ray 侧配置方式相同。
- **同集群中的整卡 worker** 仍使用 `nvidia.com/gpu`,无需 `num-gpus` 覆盖,KubeRay 可正常推导。

## 清理

```bash
kubectl delete -f ray-cluster-cgpu.yaml
```
