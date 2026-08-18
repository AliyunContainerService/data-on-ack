# Use a cGPU Shared GPU from Ray on ACK

[cGPU](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/product-overview/ack-ai-installer)
is Alibaba Cloud's container GPU-sharing technology: several pods share one
physical GPU, each capped to a slice of GPU memory (and optionally compute) by
a kernel module, so one pod cannot exhaust another's memory. On ACK a shared
GPU is requested through the extended resource `aliyun.com/gpu-mem` (in GiB)
rather than the whole-card `nvidia.com/gpu`.

Ray does not recognize the `aliyun.com/gpu-mem` resource. This guide explains
how to schedule a KubeRay worker onto a cGPU slice and expose that slice to Ray
as a GPU, and how to verify the memory isolation.

Validated on an ACK Pro (K8s 1.34) cluster with an `ecs.gn7i-c8g1.2xlarge`
node (NVIDIA A10, 22 GiB), assigning an 8 GiB slice to a single Ray worker.

## Prerequisites

- An ACK **Pro** managed cluster (the managed ACK scheduler natively supports
  `aliyun.com/gpu-mem` scheduling; no separate scheduler extender is needed).
- The **cloud-native AI suite** (`ack-ai-installer`) installed, which deploys
  `gpushare-device-plugin` and `cgpu-installer`. It can only be installed from
  the ACK console / Helm — the `aliyun cs` OpenAPI rejects it with
  `AddonNotFound`.
- KubeRay operator. On ACK it is available as a managed component
  (`kuberay-operator`), reconciled on the control-plane side; you do not run
  the operator in your own namespace.
- GPU node(s) whose instance family is supported by cGPU (T4/A10/L-series/…);
  consumer cards are generally not supported.

Label each GPU node you want to share so the AI-suite DaemonSets schedule onto
it and cGPU is enabled:

```bash
kubectl label node <gpu-node> ack.node.gpu.schedule=cgpu
```

Verify the slice resource is exposed (unit is GiB):

```bash
kubectl get node <gpu-node> -o jsonpath='{.status.allocatable.aliyun\.com/gpu-mem}'
# e.g. 22   (the A10 has 22 GiB of allocatable shared memory)
```

## The GPU count must be declared explicitly

KubeRay derives Ray's `--num-gpus` **only** from the container's
`nvidia.com/gpu` limit. A cGPU worker requests `aliyun.com/gpu-mem` instead, so
KubeRay derives 0 GPUs, Ray starts with `num-gpus=0`, and any
`@ray.remote(num_gpus=1)` actor stays Pending indefinitely.

Declare the GPU count in that worker group's `rayStartParams`:

```yaml
workerGroupSpecs:
- groupName: cgpu-worker
  rayStartParams:
    num-gpus: "1"                 # KubeRay won't infer this from gpu-mem
  template:
    spec:
      containers:
      - name: ray-worker
        resources:
          limits:
            aliyun.com/gpu-mem: "8"   # a shared 8 GiB slice, not a whole card
```

`CUDA_VISIBLE_DEVICES` is injected by gpushare-device-plugin, and the cGPU
kernel module caps the pod's visible GPU memory to the requested value. To Ray,
the slice is equivalent to a memory-limited dedicated GPU.

## 1. Deploy the RayCluster

[`ray-cluster-cgpu.yaml`](ray-cluster-cgpu.yaml) has a CPU-only head and one
worker that requests an 8 GiB slice with `num-gpus: "1"` declared. Replace the
image and `imagePullSecrets` with your own.

```bash
kubectl apply -f ray-cluster-cgpu.yaml
kubectl get pod -l ray.io/cluster=cgpu-ray -o wide
```

```
NAME                                READY   STATUS    NODE
cgpu-ray-head-xxxxx                 1/1     Running   cn-hangzhou.10.200.12.34
cgpu-ray-cgpu-worker-worker-xxxxx   1/1     Running   cn-hangzhou.10.200.12.34
```

Check the memory cap from inside the worker — `nvidia-smi` should report the
slice size, not the whole card:

```bash
WORKER=$(kubectl get pod -l ray.io/cluster=cgpu-ray,ray.io/group=cgpu-worker -o jsonpath='{.items[0].metadata.name}')
kubectl exec $WORKER -c ray-worker -- bash -lc 'env | grep ALIYUN_COM_GPU_MEM; nvidia-smi --query-gpu=memory.total --format=csv'
```

```
ALIYUN_COM_GPU_MEM_CONTAINER=8
ALIYUN_COM_GPU_MEM_DEV=22
ALIYUN_COM_GPU_MEM_UNIT=GiB
memory.total [MiB]
8373 MiB          <- capped to ~8 GiB, not 22 GiB
```

Ray reports one GPU:

```bash
HEAD=$(kubectl get pod -l ray.io/cluster=cgpu-ray,ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec $HEAD -c ray-head -- ray status | grep GPU
# 0.0/1.0 GPU
```

## 2. Verify from Ray

[`cgpu_ray_test.py`](cgpu_ray_test.py) runs a GPU actor that reports the device
in use, then allocates memory within and beyond the slice.

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

The actor's device reports **8.18 GiB total** (the slice, not the A10's 22 GiB).
A 4 GiB tensor allocates successfully; a 16 GiB tensor is rejected with an
out-of-memory error because it exceeds the slice. The memory cap is enforced by
cGPU.

## Notes

- **Running multiple actors on one slice**: request the slice once at the
  pod level (`aliyun.com/gpu-mem`), keep `num-gpus: "1"`, and give each actor a
  fractional share with `@ray.remote(num_gpus=0.5)`. Ray's fractional share is
  logical scheduling within the pod; the actual memory cap is the cGPU slice
  shared by all actors in that pod.
- **Sharing one physical GPU across pods**: schedule several cGPU worker pods
  (each requesting a slice) onto the same node; the gpushare scheduler bins them
  by `aliyun.com/gpu-mem` and cGPU isolates each pod. The pods may belong to
  different RayClusters.
- **Compute isolation**: labelling the node `ack.node.gpu.schedule=core_mem`
  (instead of `cgpu`) additionally exposes `aliyun.com/gpu-core.percentage` for
  compute-share limits; the Ray-side configuration is the same.
- **Whole-GPU workers** in the same cluster still use `nvidia.com/gpu` and need
  no `num-gpus` override; KubeRay derives the count normally.

## Cleanup

```bash
kubectl delete -f ray-cluster-cgpu.yaml
```
