# Ray Direct Transport (RDT) on eRDMA

[Ray Direct Transport (RDT)](https://docs.ray.io/en/latest/ray-core/direct-transport/direct-transport.html), introduced in Ray 2.57, lets Ray move GPU tensors **directly between actors**, bypassing the Ray object store. Instead of copying a tensor to host memory, serializing it into the object store and deserializing it on the receiver, RDT keeps the tensor in GPU memory and sends it point-to-point with a high-performance transport (NCCL, GLOO or NIXL).

[eRDMA (Elastic RDMA)](https://help.aliyun.com/zh/ecs/user-guide/erdma-overview) is Alibaba Cloud's elastic RDMA network, available on ordinary ECS instance families. With eRDMA you get RDMA-speed cross-node GPU-to-GPU transfers on a regular ACK GPU node pool, no dedicated InfiniBand cluster required.

This guide shows how to:

1. Build a Ray image with the eRDMA userspace libraries, using an in-cluster BuildKit daemon.
2. Deploy a RayCluster whose GPU workers request `aliyun/erdma` devices.
3. Run an RDT demo that moves a 512 MiB GPU tensor between two nodes over eRDMA, and prove that the bytes really went over the RDMA path.

Everything below was validated on ACK 1.36 with KubeRay 1.5.1, Ray 2.57.0 and two `ecs.ebmgn9gc.64xlarge` (NVIDIA RTX PRO 5000 72GB Blackwell) nodes.

> **Which transport?** On eRDMA, use `tensor_transport="nccl"`. The NIXL transport does not work: NIXL's UCX backend needs a UD queue pair to bootstrap its RC connections, and eRDMA does not implement UD (`uct_iface_open(ud_verbs/erdma_0:1) failed: Address not valid`). See [Transports that do not work on eRDMA](#transports-that-do-not-work-on-erdma).

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) installed in the cluster
- A GPU node pool whose instances support eRDMA (for example `ecs.ebmgn9gc.*`), with the [eRDMA interface enabled](https://help.aliyun.com/zh/ecs/user-guide/configure-erdma-on-an-enterprise-level-instance)
- The **ack-erdma-controller** component installed, so nodes expose the `aliyun/erdma` extended resource
- A container registry you can push to, plus a docker-registry Secret for it in the cluster

Verify that at least two GPU nodes report both GPU and eRDMA capacity:

```bash
kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.capacity.nvidia\.com/gpu,ERDMA:.status.capacity.aliyun/erdma,GPU_NAME:.metadata.labels.aliyun\.accelerator/nvidia_name'
```

```
NAME                   GPU   ERDMA   GPU_NAME
cn-beijing.10.0.5.75   8     400     NVIDIA-RTX-PRO-5000-72GB-Blackwell
cn-beijing.10.0.5.76   8     400     NVIDIA-RTX-PRO-5000-72GB-Blackwell
```

If `ERDMA` is empty on a node, check the `alibabacloud-erdma-agent` pod on that node — a common cause is that the `erdma` kernel module fails to load after a kernel upgrade.

### Check the memlock limit on every eRDMA node

RDMA queue pairs are backed by pinned memory, so containers need an unlimited `RLIMIT_MEMLOCK`. Kubernetes has no per-pod knob for this: containers inherit the limit from containerd. If containerd was installed with the systemd default (64 KiB), every `ibv_create_qp` inside a pod fails with `ENOMEM` and NCCL reports:

```
NCCL WARN Call to ibv_create_qp failed with error Unknown error -12
```

Check each node:

```bash
systemctl show containerd -p LimitMEMLOCK
```

If it prints anything but `infinity`, raise it and restart containerd (running containers are not killed; new containers pick up the new limit):

```bash
mkdir -p /etc/systemd/system/containerd.service.d
printf '[Service]\nLimitMEMLOCK=infinity\n' > /etc/systemd/system/containerd.service.d/10-memlock.conf
systemctl daemon-reload
systemctl restart containerd
```

Inside a worker pod, `ulimit -l` must then print `unlimited`.

## 1. Build a Ray Image with eRDMA Userspace Libraries

The stock `rayproject/ray-llm` image does not ship the ibverbs **erdma provider**, so nothing in the container can see the eRDMA device. The [`Dockerfile`](Dockerfile) in this directory adds the eRDMA userspace stack (`libibverbs1`, `ibverbs-providers`, `ibverbs-utils`, `librdmacm1`) from the Alibaba Cloud eRDMA APT repository on top of the Ray 2.57.0 base image, and pins a NCCL version that works on eRDMA (see below).

The image is built **inside the cluster** with BuildKit for two reasons: the eRDMA APT repository (`mirrors.cloud.aliyuncs.com`) is only reachable from inside a VPC, and pulling/pushing a multi-GB Ray image is far faster from ECS than from a laptop.

> **Base image note:** this guide uses `registry-cn-hangzhou.ack.aliyuncs.com/dev/ray-llm:2.57.0-py312-cu130`, a relayed copy of `rayproject/ray-llm:2.57.0-py312-cu130`, because docker.io is not reachable from regions in China. That relay repository is pull-only, so the derived image is pushed to a personal ACR repository instead. Replace the registry, repository and Secret names with your own.

> **NCCL note:** the NCCL that torch 2.11 bundles (2.28.9) **segfaults inside `ncclNetInit()`** as soon as it probes an eRDMA device, and so does 2.28.7. 2.28.3, 2.29.7 and 2.30.7 are fine, so the Dockerfile installs `nvidia-nccl-cu13==2.30.7`. Without this, every NCCL job on eRDMA dies with `SIGSEGV ... ncclNetInit()`.

### 1.1 Deploy BuildKit

[`buildkitd.yaml`](buildkitd.yaml) deploys a `buildkitd` daemon (Deployment + Service, backed by a 200 GiB disk for the build cache — unpacking the Ray base image needs well over 100 GiB and would otherwise get the pod evicted for exhausting the node's ephemeral storage) and a `buildkit-client` pod with `buildctl` and your registry credentials mounted:

```bash
kubectl apply -f buildkitd.yaml
kubectl wait --for=condition=Ready pod -l app=buildkitd --timeout=600s
kubectl wait --for=condition=Ready pod/buildkit-client --timeout=600s
```

The client pod needs credentials for **both** registries: pull from the base image registry and push to your own. Merge the two docker configs into one Secret if needed:

```bash
kubectl get secret regcred-cn-hangzhou -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d > /tmp/a.json
kubectl get secret <your-push-secret> -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d > /tmp/b.json
jq -s '{auths: (.[0].auths + .[1].auths)}' /tmp/a.json /tmp/b.json > /tmp/merged.json
kubectl create secret generic buildkit-push-creds --from-file=config.json=/tmp/merged.json
```

### 1.2 Build and Push

```bash
IMAGE=registry.cn-beijing.aliyuncs.com/<your-namespace>/ray-llm:2.57.0-py312-cu130-erdma

kubectl exec buildkit-client -- mkdir -p /ctx
kubectl cp Dockerfile buildkit-client:/ctx/Dockerfile
kubectl exec buildkit-client -- buildctl --addr tcp://buildkitd:1234 build \
  --frontend dockerfile.v0 \
  --local context=/ctx \
  --local dockerfile=/ctx \
  --output type=image,name=$IMAGE,push=true \
  --progress plain
```

Expected output (abbreviated; the first build takes ~20 min, most of it pulling and pushing the ~11 GiB of layers):

```
#6 [2/3] RUN apt-get update -qq && apt-get install -y -qq wget ...
#6 DONE 42.6s
#7 [3/3] RUN pip install --no-cache-dir ... nvidia-nccl-cu13==2.30.7
#7 DONE 38.1s
#8 exporting to image
#8 pushing layers 484.0s done
#8 pushing manifest for registry.cn-beijing.aliyuncs.com/.../ray-llm:2.57.0-py312-cu130-erdma
#8 DONE 485.1s
```

If the push fails with `insufficient_scope: authorization failed`, your credentials only allow pulling from that repository — push to a repository you own.

## 2. Deploy the RayCluster

[`ray-cluster-erdma.yaml`](ray-cluster-erdma.yaml) creates a RayCluster with a CPU-only head and 2 GPU workers. What matters for eRDMA, compared with a basic RayCluster:

- Workers request `aliyun/erdma: 1` alongside `nvidia.com/gpu: 1`.
- Workers run with `hostNetwork: true`, `hostIPC: true` and `dnsPolicy: ClusterFirstWithHostNet`, and add the `IPC_LOCK` capability so ibverbs can pin memory.
- NCCL is pointed at the eRDMA device: `NCCL_IB_DISABLE=0`, `NCCL_IB_HCA=erdma_0`, `NCCL_SOCKET_IFNAME=eth0`.
- The head sets `num-cpus: "0"` **and** `num-gpus: "0"`. The head container has no GPU limit, so it can see all GPUs of the node it lands on; without `num-gpus: "0"` Ray advertises them and GPU actors can be scheduled onto a pod that has no eRDMA device.
- `nodeSelector` targets the eRDMA-capable GPU nodes (adjust the GPU model label for your node pool).

```bash
kubectl apply -f ray-cluster-erdma.yaml
kubectl get pods -l ray.io/cluster=ray-erdma-cluster -o wide
```

```
NAME                                       READY   STATUS    RESTARTS   AGE   IP          NODE
ray-erdma-cluster-gpu-erdma-worker-nldp2   1/1     Running   0          2m1s  10.0.5.76   cn-beijing.10.0.5.76
ray-erdma-cluster-gpu-erdma-worker-zttzj   1/1     Running   0          2m1s  10.0.5.75   cn-beijing.10.0.5.75
ray-erdma-cluster-head-9v5sd               1/1     Running   0          2m1s  10.0.5.82   cn-beijing.10.0.5.76
```

### Verify eRDMA Inside the Workers

```bash
WORKER=$(kubectl get pod -l ray.io/cluster=ray-erdma-cluster,ray.io/node-type=worker -o jsonpath='{.items[0].metadata.name}')
kubectl exec $WORKER -c ray-worker -- bash -lc 'ulimit -l; ibv_devinfo -d erdma_0 | head -12'
```

```
unlimited
hca_id: erdma_0
        transport:                      eRDMA (0)
        fw_ver:                         0.2.0
        ...
                port:   1
                        state:                  PORT_ACTIVE (4)
                        link_layer:             Ethernet
```

Also confirm the pinned NCCL made it into the image:

```bash
kubectl exec $WORKER -c ray-worker -- bash -lc 'python -c "import torch, cupy.cuda.nccl as n; print(n.get_version())"'
# 23007
```

## 3. Run the RDT Demo

[`rdt_nccl_demo.py`](rdt_nccl_demo.py) creates a `Producer` and a `Consumer` actor, forced onto **different nodes** with a `STRICT_SPREAD` placement group. The producer returns a 512 MiB CUDA tensor from a method annotated with `@ray.method(tensor_transport="nccl")`; Ray moves it GPU-to-GPU with NCCL over eRDMA instead of through the object store:

```python
@ray.remote(num_gpus=1)
class Producer:
    @ray.method(tensor_transport="nccl")
    def make_tensor(self, n: int):
        return torch.ones(n, dtype=torch.float32, device="cuda")


# The receiving actor needs enable_tensor_transport=True as well.
@ray.remote(num_gpus=1, enable_tensor_transport=True)
class Consumer:
    def consume(self, tensor: torch.Tensor):
        ...


create_collective_group([producer, consumer], backend="nccl")
```

Two easy-to-miss requirements:

- **The consumer needs `enable_tensor_transport=True`.** Only actors that declare a tensor transport get the background thread that services RDT receives. Without it the receive is queued behind `consume()`, which is itself blocked waiting for the tensor, and the transfer fails with `ObjectRef ... not found in RDT object store after 60.0s`.
- **The NCCL transport needs an explicit collective group** over the participating actors (`ray.experimental.collective.create_collective_group`).

Run it from the head pod:

```bash
HEAD=$(kubectl get pod -l ray.io/cluster=ray-erdma-cluster,ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl cp rdt_nccl_demo.py $HEAD:/tmp/rdt_nccl_demo.py -c ray-head
kubectl exec $HEAD -c ray-head -- python /tmp/rdt_nccl_demo.py
```

```
producer node=c59eae7f8c63a1f51638d8c2554486ef4288e98408cd6b39d009b12e gpu=NVIDIA RTX PRO 5000 72GB Blackwell
consumer node=2ef590a9e0e27dc130d85579fb46e1ce6c09d0903027d2c0419b39c8 gpu=NVIDIA RTX PRO 5000 72GB Blackwell
iter 0: 512 MiB -> cuda:0 in 747.3 ms (0.67 GiB/s), checksum OK
iter 1: 512 MiB -> cuda:0 in 43.5 ms (11.51 GiB/s), checksum OK
iter 2: 512 MiB -> cuda:0 in 42.2 ms (11.84 GiB/s), checksum OK
RDT over eRDMA: OK
```

The first iteration includes NCCL communicator setup. Steady-state, 512 MiB crosses the network in ~42 ms, about 11.8 GiB/s. That is not the ceiling: `ecs.ebmgn9gc.64xlarge` is specified at ~380 Gbps across its two eRDMA interfaces, and a WRITE-based transfer over a single device measures 20.4 GiB/s (see [`nixl/`](nixl/)), so a single-device NCCL path without GPUDirect RDMA is using roughly half of one interface.

### Confirm the Transfer Really Used eRDMA

NCCL silently falls back to TCP when the RDMA path is unusable, so read the device counters. On the producer's node, before and after a run:

```bash
kubectl exec $WORKER -c ray-worker -- cat /sys/class/infiniband/erdma_0/ports/1/hw_counters/hw_tx_bytes_cnt
```

Three 512 MiB transfers move 1.53 GiB over the device:

```
eRDMA tx during RDT run: 1.53 GiB
```

As a control, rerun with `NCCL_IB_DISABLE=1` (forcing NCCL onto sockets). The job still succeeds, but the eRDMA counter does not move at all:

```
TCP-run eRDMA tx delta: 0.000 GiB
```

The NCCL log inside the worker (`/tmp/nccl-*.log`, enabled by `NCCL_DEBUG=INFO` in the demo) is the other place to look:

```
NET/IB : Using [0]erdma_0:1/RoCE [RO]; OOB eth0:10.0.5.76<0>
NET/IB: ncclIbReceiverQpsCreateToRts: QP created: port=1 dev=0 devName=erdma_0 ...
```

To separate "eRDMA/NCCL is broken" from "Ray RDT is misconfigured", run NCCL with Ray out of the picture: [`erdma-nccl-check-pods.yaml`](erdma-nccl-check-pods.yaml) starts one plain pod on each eRDMA node, and [`nccl_allreduce_check.py`](nccl_allreduce_check.py) runs a two-rank `all_reduce` between them (usage in the script's docstring). On the validated setup a 0.5 GiB `all_reduce` takes 80–90 ms over eRDMA; with `NCCL_IB_DISABLE=1` the wall time is similar but the device counter stays flat, which is how you tell the two paths apart.

## Other Transports

- **NIXL** does not work with the prebuilt wheels. They bundle their own UCX and their own `libibverbs`, so UCX sees no RDMA device at all, and once that is worked around UCX's `rc_verbs` transport still insists on a shared receive queue, which eRDMA does not implement (`ibv_create_srq() failed: Operation not supported`). It *can* be made to work, either by rebuilding UCX with Alibaba Cloud's eRDMA patch or by building NIXL with the Mooncake backend; both paths are documented and benchmarked in [`nixl/`](nixl/). Expect ~0.84 GiB/s GPU-to-GPU with NIXL/UCX against ~11.5 GiB/s with NCCL, so reach for NIXL only when something else in your stack requires it.
- **GLOO** works, but it is a CPU transport: tensors are copied to host memory first, which defeats the purpose here.

## Notes and Caveats

- **Object mutability**: unlike regular Ray objects, tensors passed through RDT are **not copied** — producer and consumer may reference the same GPU memory. Do not mutate a tensor after returning it. See [RDT object mutability](https://docs.ray.io/en/latest/ray-core/direct-transport/direct-transport.html#object-mutability).
- **`ray.get` on RDT references** from the driver or a non-GPU process falls back to the object store and copies the data; keep the hot path actor-to-actor.
- **GPUDirect RDMA is disabled** on these instances (`NET/IB : GPU Direct RDMA Disabled for HCA 0 'erdma_0'`), so data still hops through host memory. That costs surprisingly little here, because PCIe is much faster than the NIC and NCCL overlaps the staging copies with the transfers; where the same staging is done naively it collapses to well under 1 GiB/s, which is exactly what happens to NIXL/UCX (see [`nixl/`](nixl/)).
- **hostNetwork** means at most one Ray worker pod per node; size `replicas` to your node count.
- **One collective group per actor pair**: an actor can participate in only one collective group per backend at a time. Ray destroys the group when a transfer fails, so a failed run can leave detached `NCCLUniqueIDStore` actors behind; clean them up before retrying.

## Cleanup

```bash
kubectl delete -f ray-cluster-erdma.yaml
kubectl delete -f buildkitd.yaml
```
