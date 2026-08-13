# 在 eRDMA 上运行 Ray Direct Transport（RDT）

[Ray Direct Transport（RDT）](https://docs.ray.io/en/latest/ray-core/direct-transport/direct-transport.html)是 Ray 2.57 引入的能力，可以让 Ray 在 **Actor 之间直接传输 GPU 张量**，绕过 Ray 对象存储。传统路径需要把张量拷回 host 内存、序列化写入对象存储，再在接收端反序列化；而 RDT 让张量始终留在显存中，通过高性能传输后端（NCCL、GLOO 或 NIXL）点对点发送。

[eRDMA（弹性 RDMA）](https://help.aliyun.com/zh/ecs/user-guide/erdma-overview)是阿里云在普通 ECS 实例规格上提供的弹性 RDMA 网络。借助 eRDMA，你可以在普通 ACK GPU 节点池上获得 RDMA 级别的跨节点 GPU 到 GPU 传输，无需专用 InfiniBand 集群。

本文介绍如何：

1. 使用集群内的 BuildKit 构建带 eRDMA 用户态库的 Ray 镜像。
2. 部署 GPU worker 申请 `aliyun/erdma` 设备的 RayCluster。
3. 运行 RDT 示例，在两个节点之间传输 512 MiB 的 GPU 张量，并验证流量确实走了 RDMA 链路。

以下内容均在 ACK 1.36 + KubeRay 1.5.1 + Ray 2.57.0、两台 `ecs.ebmgn9gc.64xlarge`（NVIDIA RTX PRO 5000 72GB Blackwell）节点上实测通过。

> **该选哪个传输后端？** 在 eRDMA 上请使用 `tensor_transport="nccl"`。NIXL 后端用不了：NIXL 的 UCX 后端需要 UD 队列对来建立 RC 连接，而 eRDMA 不支持 UD（`uct_iface_open(ud_verbs/erdma_0:1) failed: Address not valid`）。详见[在 eRDMA 上不可用的传输后端](#在-erdma-上不可用的传输后端)。

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)
- 有一个支持 eRDMA 的 GPU 节点池（例如 `ecs.ebmgn9gc.*`），并已[开启 eRDMA 网卡](https://help.aliyun.com/zh/ecs/user-guide/configure-erdma-on-an-enterprise-level-instance)
- 已安装 **ack-erdma-controller** 组件，使节点上报 `aliyun/erdma` 扩展资源
- 一个有推送权限的容器镜像仓库，以及集群中对应的 docker-registry Secret

确认至少有两个 GPU 节点同时上报 GPU 和 eRDMA 资源：

```bash
kubectl get nodes -o custom-columns='NAME:.metadata.name,GPU:.status.capacity.nvidia\.com/gpu,ERDMA:.status.capacity.aliyun/erdma,GPU_NAME:.metadata.labels.aliyun\.accelerator/nvidia_name'
```

```
NAME                   GPU   ERDMA   GPU_NAME
cn-beijing.10.0.5.75   8     400     NVIDIA-RTX-PRO-5000-72GB-Blackwell
cn-beijing.10.0.5.76   8     400     NVIDIA-RTX-PRO-5000-72GB-Blackwell
```

如果某个节点的 `ERDMA` 为空，检查该节点上的 `alibabacloud-erdma-agent` Pod。常见原因是内核升级后 `erdma` 内核模块加载失败。

### 检查每个 eRDMA 节点的 memlock 限制

RDMA 队列对依赖锁定（pinned）内存，因此容器需要不受限的 `RLIMIT_MEMLOCK`。Kubernetes 没有 Pod 级别的配置项，容器的该限制继承自 containerd。如果 containerd 使用了 systemd 默认值（64 KiB），Pod 内所有 `ibv_create_qp` 都会因 `ENOMEM` 失败，NCCL 会报：

```
NCCL WARN Call to ibv_create_qp failed with error Unknown error -12
```

逐个节点检查：

```bash
systemctl show containerd -p LimitMEMLOCK
```

如果输出不是 `infinity`，调整后重启 containerd（已运行的容器不会被杀掉，新建容器才会应用新限制）：

```bash
mkdir -p /etc/systemd/system/containerd.service.d
printf '[Service]\nLimitMEMLOCK=infinity\n' > /etc/systemd/system/containerd.service.d/10-memlock.conf
systemctl daemon-reload
systemctl restart containerd
```

之后在 worker Pod 内执行 `ulimit -l` 应输出 `unlimited`。

## 1. 构建带 eRDMA 用户态库的 Ray 镜像

官方 `rayproject/ray-llm` 镜像不包含 ibverbs 的 **erdma provider**，因此容器内看不到 eRDMA 设备。本目录下的 [`Dockerfile`](Dockerfile) 在 Ray 2.57.0 基础镜像之上，从阿里云 eRDMA APT 源安装用户态组件（`libibverbs1`、`ibverbs-providers`、`ibverbs-utils`、`librdmacm1`），并固定一个在 eRDMA 上可用的 NCCL 版本（见下文）。

镜像在**集群内**用 BuildKit 构建，原因有两点：eRDMA APT 源（`mirrors.cloud.aliyuncs.com`）只能在 VPC 内访问；以及从 ECS 拉取/推送数 GB 的 Ray 镜像远快于本地笔记本。

> **基础镜像说明：** 由于国内地域无法直连 docker.io，本文使用 `rayproject/ray-llm:2.57.0-py312-cu130` 的转投副本 `registry-cn-hangzhou.ack.aliyuncs.com/dev/ray-llm:2.57.0-py312-cu130`。该转投仓库只有拉取权限，所以派生镜像推到了自己的 ACR 仓库。请把镜像仓库地址和 Secret 名称替换为你自己的。

> **NCCL 说明：** torch 2.11 自带的 NCCL（2.28.9）在探测 eRDMA 设备时会**在 `ncclNetInit()` 内直接 segfault**，2.28.7 同样有问题；2.28.3、2.29.7、2.30.7 正常，因此 Dockerfile 里安装了 `nvidia-nccl-cu13==2.30.7`。不做这一步，eRDMA 上所有 NCCL 任务都会以 `SIGSEGV ... ncclNetInit()` 崩溃。

### 1.1 部署 BuildKit

[`buildkitd.yaml`](buildkitd.yaml) 部署一个 `buildkitd` 守护进程（Deployment + Service，并挂载 200 GiB 云盘存放构建缓存——解压 Ray 基础镜像需要 100 GiB 以上空间，否则 Pod 会因耗尽节点临时存储被驱逐），以及一个挂载了 `buildctl` 和镜像仓库凭证的 `buildkit-client` Pod：

```bash
kubectl apply -f buildkitd.yaml
kubectl wait --for=condition=Ready pod -l app=buildkitd --timeout=600s
kubectl wait --for=condition=Ready pod/buildkit-client --timeout=600s
```

客户端 Pod 需要**两个**仓库的凭证：从基础镜像仓库拉取，向自己的仓库推送。可以把两份 docker config 合并成一个 Secret：

```bash
kubectl get secret regcred-cn-hangzhou -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d > /tmp/a.json
kubectl get secret <your-push-secret> -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d > /tmp/b.json
jq -s '{auths: (.[0].auths + .[1].auths)}' /tmp/a.json /tmp/b.json > /tmp/merged.json
kubectl create secret generic buildkit-push-creds --from-file=config.json=/tmp/merged.json
```

### 1.2 构建并推送

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

预期输出（已省略部分内容；首次构建约 20 分钟，绝大部分时间花在拉取和推送约 11 GiB 的层上）：

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

如果推送时报 `insufficient_scope: authorization failed`，说明当前凭证对该仓库只有拉取权限，请推送到你自己拥有的仓库。

## 2. 部署 RayCluster

[`ray-cluster-erdma.yaml`](ray-cluster-erdma.yaml) 创建一个纯 CPU head + 2 个 GPU worker 的 RayCluster。与普通 RayCluster 相比，eRDMA 相关的关键点：

- worker 在 `nvidia.com/gpu: 1` 之外还申请 `aliyun/erdma: 1`。
- worker 使用 `hostNetwork: true`、`hostIPC: true` 和 `dnsPolicy: ClusterFirstWithHostNet`，并添加 `IPC_LOCK` capability，使 ibverbs 能锁定内存。
- 通过环境变量把 NCCL 指向 eRDMA 设备：`NCCL_IB_DISABLE=0`、`NCCL_IB_HCA=erdma_0`、`NCCL_SOCKET_IFNAME=eth0`。
- head 同时设置 `num-cpus: "0"` **和** `num-gpus: "0"`。head 容器没有 GPU limit，因此能看到所在节点的全部 GPU；不设 `num-gpus: "0"` 的话 Ray 会把这些 GPU 上报为可调度资源，GPU Actor 可能被调度到没有 eRDMA 设备的 Pod 上。
- `nodeSelector` 指向支持 eRDMA 的 GPU 节点（请按你的节点池调整 GPU 型号标签）。

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

### 在 worker 内验证 eRDMA

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

同时确认固定版本的 NCCL 已经打进镜像：

```bash
kubectl exec $WORKER -c ray-worker -- bash -lc 'python -c "import torch, cupy.cuda.nccl as n; print(n.get_version())"'
# 23007
```

## 3. 运行 RDT 示例

[`rdt_nccl_demo.py`](rdt_nccl_demo.py) 创建一个 `Producer` 和一个 `Consumer` Actor，并用 `STRICT_SPREAD` 放置组把它们强制分散到**不同节点**。producer 的方法用 `@ray.method(tensor_transport="nccl")` 标注并返回 512 MiB 的 CUDA 张量，Ray 会用 NCCL 经 eRDMA 直接做 GPU 到 GPU 传输，而不经过对象存储：

```python
@ray.remote(num_gpus=1)
class Producer:
    @ray.method(tensor_transport="nccl")
    def make_tensor(self, n: int):
        return torch.ones(n, dtype=torch.float32, device="cuda")


# 接收端 Actor 同样需要 enable_tensor_transport=True
@ray.remote(num_gpus=1, enable_tensor_transport=True)
class Consumer:
    def consume(self, tensor: torch.Tensor):
        ...


create_collective_group([producer, consumer], backend="nccl")
```

两个容易忽略的要求：

- **接收端必须设置 `enable_tensor_transport=True`。** 只有声明了 tensor transport 的 Actor 才会获得处理 RDT 接收的后台线程。否则接收任务会排在 `consume()` 后面，而 `consume()` 本身正阻塞等待张量，最终以 `ObjectRef ... not found in RDT object store after 60.0s` 失败。
- **NCCL 后端需要显式创建 collective group**（`ray.experimental.collective.create_collective_group`）。

在 head Pod 上运行：

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

第一轮包含 NCCL 通信域建立开销。稳态下 512 MiB 跨节点传输约 42 ms，约 11.8 GiB/s。但这并不是上限：`ecs.ebmgn9gc.64xlarge` 规格上两块 eRDMA 网卡合计约 380 Gbps，实测单块网卡上基于 WRITE 的传输可以到 20.4 GiB/s（见 [`nixl/`](nixl/)），所以未开 GDR、只用单块网卡的 NCCL 路径大约只用掉了一块网卡的一半带宽。

### 确认流量确实走了 eRDMA

RDMA 路径不可用时 NCCL 会静默回退到 TCP，因此要读设备计数器。在 producer 所在节点上，运行前后各读一次：

```bash
kubectl exec $WORKER -c ray-worker -- cat /sys/class/infiniband/erdma_0/ports/1/hw_counters/hw_tx_bytes_cnt
```

三次 512 MiB 传输在设备上产生 1.53 GiB 流量：

```
eRDMA tx during RDT run: 1.53 GiB
```

作为对照，加上 `NCCL_IB_DISABLE=1`（强制 NCCL 走 socket）再跑一次。任务依然成功，但 eRDMA 计数器完全不动：

```
TCP-run eRDMA tx delta: 0.000 GiB
```

另一个观察点是 worker 内的 NCCL 日志（`/tmp/nccl-*.log`，由示例中的 `NCCL_DEBUG=INFO` 开启）：

```
NET/IB : Using [0]erdma_0:1/RoCE [RO]; OOB eth0:10.0.5.76<0>
NET/IB: ncclIbReceiverQpsCreateToRts: QP created: port=1 dev=0 devName=erdma_0 ...
```

想区分「eRDMA/NCCL 本身有问题」和「Ray RDT 配置有问题」时，可以完全绕开 Ray 跑一次 NCCL：[`erdma-nccl-check-pods.yaml`](erdma-nccl-check-pods.yaml) 在两个 eRDMA 节点上各起一个裸 Pod，[`nccl_allreduce_check.py`](nccl_allreduce_check.py) 在两者之间跑两 rank 的 `all_reduce`（用法见脚本 docstring）。实测环境下 0.5 GiB 的 `all_reduce` 走 eRDMA 约 80–90 ms；加 `NCCL_IB_DISABLE=1` 后耗时接近，但设备计数器保持不变——这正是区分两条路径的方法。

## 其他传输后端

- **NIXL** 用官方预编译 wheel 是跑不起来的：wheel 里自带 UCX 和改名后的 `libibverbs`，导致 UCX 完全看不到 RDMA 设备；即使绕过这一点，UCX 的 `rc_verbs` 传输仍然强制创建共享接收队列，而 eRDMA 不支持 SRQ（`ibv_create_srq() failed: Operation not supported`）。但它**是可以跑通的**：一条路是用阿里云的 eRDMA patch 重新编译 UCX，另一条路是源码构建带 Mooncake 后端的 NIXL，两条路的做法和实测数据都记录在 [`nixl/`](nixl/) 里。性能上 NIXL/UCX 的 GPU 间传输约 0.84 GiB/s，而 NCCL 约 11.5 GiB/s，所以只有在技术栈里确实需要 NIXL 时才值得走这条路。
- **GLOO** 可用，但它是 CPU 传输：张量会先拷到 host 内存，失去了这里的意义。

## 注意事项

- **对象可变性**：与普通 Ray 对象不同，通过 RDT 传递的张量**不会被拷贝**，producer 和 consumer 可能引用同一块显存。返回后不要再修改该张量。参见 [RDT object mutability](https://docs.ray.io/en/latest/ray-core/direct-transport/direct-transport.html#object-mutability)。
- **在 driver 或非 GPU 进程中对 RDT 引用调用 `ray.get`** 会回退到对象存储并拷贝数据；热路径应保持 Actor 到 Actor。
- **GPUDirect RDMA 在这类实例上未启用**（`NET/IB : GPU Direct RDMA Disabled for HCA 0 'erdma_0'`），数据仍会经过 host 内存中转。但这里代价很小，因为 PCIe 比网卡快得多，而 NCCL 会把中转拷贝与网络传输重叠起来；如果同样的中转做得比较朴素，吞吐会掉到 1 GiB/s 以下——NIXL/UCX 正是如此（见 [`nixl/`](nixl/)）。
- **hostNetwork** 意味着每个节点最多只能跑一个 Ray worker Pod，`replicas` 请按节点数设置。
- **每个 Actor 每种后端只能加入一个 collective group**。传输失败时 Ray 会销毁该 group，因此失败的运行可能残留游离的 `NCCLUniqueIDStore` Actor，重试前需要清理。

## 清理

```bash
kubectl delete -f ray-cluster-erdma.yaml
kubectl delete -f buildkitd.yaml
```
