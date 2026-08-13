# 在 eRDMA 上使用 NIXL

主文档使用 NCCL 作为 RDT 的传输后端，因为 NIXL 在 eRDMA 上开箱是跑不通的。本目录记录让 NIXL 跑起来需要做什么，以及最终能拿到什么效果。以下两条路都在两台 `ecs.ebmgn9gc.64xlarge`（RTX PRO 5000，两块 eRDMA 网卡，合计约 380 Gbps）上实测通过。

## 省事的做法：直接用预构建镜像

下面所有内容都已经通过 [`Dockerfile`](Dockerfile) 打进镜像，并由 [`build-job.yaml`](build-job.yaml) 在集群内构建：

```bash
kubectl apply -f build-job.yaml     # 创建上下文 PVC 和 Job
kubectl logs -f job/nixl-image-build
```

之所以用 Job 而不是 `kubectl exec`，是为了让构建不受本地网络断开影响；BuildKit 缓存热的情况下约 10 分钟。开始前需要先把源码 tarball 放到上下文 PVC 上——集群内访问不了 github，Dockerfile 头部列出了每个文件及其下载地址。构建出的镜像里已包含 `/opt/ucx-erdma`、`/opt/nixl`、`libtransfer_engine.so` 以及 patch 所需的 `UCX_*` 环境变量，`ucx_perftest` 和 `import nixl` 开箱可用。

[`build_ucx_erdma.sh`](build_ucx_erdma.sh) 和 [`build_nixl_mooncake.sh`](build_nixl_mooncake.sh) 这两个脚本在运行中的 Pod 里做同样的事，调参迭代时更方便。

## 官方 NIXL 为什么失败

预编译的 `nixl` / `nixl-cu13` wheel 自带一份 UCX（1.21）和一份改名后的 `libibverbs`，由此带来两个问题：

1. wheel 里的 `libibverbs` 没有 provider 配置，UCX 根本看不到 RDMA 设备（`network device 'erdma_0:1' is not available`）。把它指向系统的 libibverbs 即可解决：

   ```bash
   D=$(python -c "import nixl_cu13, os; print(os.path.dirname(nixl_cu13.__file__)+'.libs')")
   ln -sf /usr/lib/x86_64-linux-gnu/libibverbs.so.1 "$D"/libibverbs-*.so.*
   ```

2. UCX 的 `rc_verbs` 传输无条件创建共享接收队列，而 eRDMA 不支持 SRQ：

   ```
   UCX ERROR erdma_0: ibv_create_srq() failed: Operation not supported
   ```

   这一条没有运行期的绕过方法，必须给 UCX 打 patch。

## 节点前提

两条路都要求 eRDMA 驱动开启兼容模式，且版本足够新（能上报 SRQ 能力、支持 UD 模拟）：

```bash
cat /sys/module/erdma/parameters/compat_mode        # 必须是 Y
ibv_devinfo -d erdma_0 -v | grep max_srq            # 必须大于 0
```

如果 `compat_mode` 是 `N`，或 `max_srq` 为 0，重新安装并加载驱动：

```bash
wget http://mirrors.cloud.aliyuncs.com/erdma/env_setup.sh
bash env_setup.sh --url http://mirrors.cloud.aliyuncs.com/erdma/erdma_installer-1.5.4.tar.gz
rmmod erdma && modprobe erdma compat_mode=Y
printf 'options erdma compat_mode=Y\n' > /etc/modprobe.d/erdma-compat.conf
```

> 如果节点上存在 NVIDIA OFED 源码目录 `/usr/src/ofa_kernel`，但运行的内核用的是内置 `ib_core`，DKMS 会按 OFED 的符号来编译，编出来的模块加载时会报 `erdma: Unknown symbol ib_umem_get_peer`。解决办法是编译时先把 OFED 目录藏起来：`mv /usr/src/ofa_kernel /usr/src/ofa_kernel.hidden`，重新 `dkms build/install`，再移回来并 `depmod -a`。

另外确认 Pod 内 `ulimit -l` 为 `unlimited`，参见主文档的 memlock 一节。

## 路线 1：用 eRDMA patch 重新编译 UCX

阿里云提供了一个 UCX patch，把 SRQ 换成每 QP 独立的接收队列。[`build_ucx_erdma.sh`](build_ucx_erdma.sh) 会打上这个 patch 并完成编译。patch 是针对 1.19.0 发布的，但可以干净地应用到 1.21.0 上，而 1.21.0 正是 NIXL 1.3 编译所依赖的版本。

```bash
./build_ucx_erdma.sh 1.21.0 /opt/ucx121-erdma
```

运行时需要的环境变量：

```bash
export UCX_RC_VERBS_USE_SRQ=n
export UCX_RC_VERBS_RX_CQ_LEN=131072
export UCX_UD_VERBS_TX_MIN_INLINE=128
export UCX_NET_DEVICES=erdma_0:1
```

用 UCX 自带的 benchmark 在两个节点间验证（注意 `UCX_TLS` 里没有 `tcp`，所以只要能跑出结果就说明走的是 eRDMA）：

```bash
export UCX_TLS=rc_verbs,ud_verbs,self,sm
# 节点 A
/opt/ucx121-erdma/bin/ucx_perftest -c 0
# 节点 B
/opt/ucx121-erdma/bin/ucx_perftest <节点A的IP> -t tag_bw -s 8388608 -n 300 -c 1
```

```
Final:  300  1133.820  1135.543  1135.543  7045.09  7045.09  881  881
ucp_worker.c:1903 UCX INFO perftest inter-node cfg#0 tag(rc_verbs/erdma_0:1)
```

走 `rc_verbs/erdma_0:1`，7.0 GB/s。

### 把自编译的 UCX 塞到预编译 NIXL wheel 下面

wheel 的 UCX 插件从 wheel 自己的 lib 目录 dlopen UCX，而 UCX 又会从「加载它的那个库所在目录」旁边去找 transport 模块，两处都要替换：

```bash
D=$(python -c "import nixl_cu13, os; print(os.path.dirname(nixl_cu13.__file__)+'.libs')")
for n in ucm ucp ucs uct; do ln -sf /opt/ucx121-erdma/lib/lib$n.so.0 "$D"/lib$n-*.so.0.0.0; done
for f in /opt/ucx121-erdma/lib/ucx/*.so; do ln -sf "$f" "$D/ucx/$(basename "$f")"; done
ln -sf /usr/lib/x86_64-linux-gnu/libibverbs.so.1 "$D"/libibverbs-*.so.*
```

两个踩过的坑：UCX 必须是 1.21（1.19 缺少插件需要的 `ucp_device_mem_list_release` 符号），并且必须带 `--enable-mt` 编译，否则 NIXL 会报 "UCX library does not support multi-threading"。

如果本来就要做镜像，直接用 [`build_nixl_mooncake.sh`](build_nixl_mooncake.sh) 基于 `/opt/ucx121-erdma` 从源码构建 NIXL 会干净得多。

## 路线 2：NIXL 使用 Mooncake 后端

Mooncake Transfer Engine 直接调用 verbs 且不使用 SRQ，所以在未打 patch 的环境下也能跑 eRDMA。NIXL 自带 Mooncake 插件，但预编译 wheel 里没有编进去，而且该插件链接的是 `libtransfer_engine.so`——PyPI 上的 `mooncake-transfer-engine` 包里并没有这个库，因此 Mooncake 必须源码构建。[`build_nixl_mooncake.sh`](build_nixl_mooncake.sh) 完成 Mooncake 构建，并编译出同时带 UCX 和 Mooncake 后端的 NIXL。

Mooncake 需要一个 metadata 服务，最简单的一个就在 pip 包里：

```bash
python -m mooncake.http_metadata_server --port 18080
```

然后在每个节点上：

```bash
export NIXL_PLUGIN_DIR=/opt/nixl/lib/x86_64-linux-gnu/plugins
export LD_LIBRARY_PATH=/opt/nixl/lib/x86_64-linux-gnu:/opt/ucx121-erdma/lib:/usr/local/lib:$LD_LIBRARY_PATH
export MC_METADATA_SERVER=http://<metadata节点>:18080/metadata
export MC_PROTOCOL=rdma
```

先单独验证 transfer engine（绕开 NIXL）也很有用，benchmark 二进制就在 pip 包里：

```bash
# 预编译二进制需要 CUDA 12 运行时：pip install nvidia-cuda-runtime-cu12
transfer_engine_bench --mode=target    --use_vram=false --buffer_size=4294967296 \
  --protocol=rdma --device_name=erdma_0 --metadata_server=http://...:18080/metadata \
  --local_server_name=<A>:12420
transfer_engine_bench --mode=initiator --use_vram=false --buffer_size=4294967296 \
  --protocol=rdma --device_name=erdma_0 --metadata_server=http://...:18080/metadata \
  --segment_id=<A>:12420 --local_server_name=<B>:12421 --duration=10 \
  --operation=read --threads=12 --block_size=1048576
# Test completed: duration 10.39, batch count 241, throughput 2.90 GiB/s
```

两侧的 `--buffer_size` 至少要有 `threads * batch_size * block_size` 那么大，否则 initiator 会读到远端 segment 之外，所有请求都会失败。

## 跨节点 NIXL 验证脚本

[`nixl_check.py`](nixl_check.py) 在两侧各注册一块 256 MiB 的 buffer，通过一个普通 TCP socket 交换 agent 元数据，然后发起一次 NIXL READ。两种后端都能用：

```bash
# target 侧
python nixl_check.py --role target    --backend UCX      --mem vram
# initiator 侧
python nixl_check.py --role initiator --backend UCX      --mem vram --peer <target-ip>
```

实测数据（256 MiB，5 次取最好，跨节点）。两个方向、两种内存都列出来，因为差别极大：

| 后端     | 内存类型 | READ | WRITE |
|----------|----------|------|-------|
| UCX      | DRAM     | 2.7 GiB/s | 19~20 GiB/s |
| UCX      | VRAM     | 0.4~0.85 GiB/s | 0.3~0.9 GiB/s |
| Mooncake | DRAM     | 4.1 GiB/s | 未测 |
| Mooncake | VRAM     | 不支持 | 不支持 |

VRAM 两行给的是区间，因为同样的配置反复跑波动超过 2 倍；DRAM 两行则稳定在几个百分点内。

eRDMA 设备计数器的增量与传输量一致，可据此确认数据路径：

```bash
cat /sys/class/infiniband/erdma_0/ports/1/hw_counters/hw_rx_bytes_cnt
```

注意在多 ENI 的实例上 Mooncake 可能选中 `erdma_1`，两个设备的计数器都要看。

## 到底走的是 RDMA 还是回退到 TCP 了？

在相信上面任何数字之前都值得先确认这一点，因为 UCX 的回退是静默的。[`verify_transport.sh`](verify_transport.sh) 同时用两种方式判定：一是在每次跑动前后 diff eRDMA 设备的字节计数器，二是从 UCX 自己的 `UCX_LOG_LEVEL=info` 输出里抓它选中的 lane；然后再用只允许 TCP 的传输列表跑一遍作为对照。

```
RDMA-only TLS (no tcp in the list):
READ  dram  rc_verbs     2.71 GiB/s   erdma bytes 1.28 GiB (payload 1.25)  rma(rc_verbs/erdma_0:1)
WRITE dram  rc_verbs    19.34 GiB/s   erdma bytes 1.27 GiB (payload 1.25)  rma(rc_verbs/erdma_0:1)
READ  vram  rc_verbs     0.41 GiB/s   erdma bytes 1.37 GiB (payload 1.25)  rma(rc_verbs/erdma_0:1)
WRITE vram  rc_verbs     0.30 GiB/s   erdma bytes 1.32 GiB (payload 1.25)  rma(rc_verbs/erdma_0:1)

TCP-only TLS, for contrast:
READ  dram  tcp          3.64 GiB/s   erdma bytes 0.00 GiB (payload 1.25)  am(tcp/eth0)
WRITE dram  tcp          5.68 GiB/s   erdma bytes 0.00 GiB (payload 1.25)  am(tcp/eth0)
READ  vram  tcp          0.58 GiB/s   erdma bytes 0.00 GiB (payload 1.25)  am(tcp/eth0)
WRITE vram  tcp          0.54 GiB/s   erdma bytes 0.00 GiB (payload 1.25)  am(tcp/eth0)
```

RDMA 那几行的计数器增量与传输量吻合，TCP 那几行则完全为 0，因此 RDMA 的数字是可信的。但由此也带出两个不太舒服的结论：

- **只有 DRAM 的 WRITE 场景 RDMA 才真正占优**（19.3 对 5.7 GiB/s）。host 内存的单向 READ 上，普通 TCP 反而比 eRDMA 更快（3.6 对 2.7 GiB/s）。
- **显存场景下 TCP 在两个方向上都快过 eRDMA**（0.54~0.58 对 0.30~0.41 GiB/s）。`cuda_copy` 中转的开销大到让下面用什么网络几乎不重要了。

另外 TCP 需要一块以太网设备：如果 `UCX_NET_DEVICES` 被固定成 `erdma_0:1`，tcp 传输没有可用设备，backend 会直接创建失败——这本身也顺便证明了上面那几行 RDMA 结果不可能是 TCP 跑出来的。

## 吞吐究竟花在了哪里

[`nixl_tuning_sweep.py`](nixl_tuning_sweep.py) 会扫描传输方向和描述符数量，上面的数据就是它跑出来的。在给 NIXL 的性能下结论之前，有三点必须先知道。

**慢的是 RDMA READ，不是 NIXL。** 256 MiB 的单向 READ 只有 2.7 GiB/s，而同一套栈换成 WRITE 就是 20.4 GiB/s。原生 UCX 也印证了这一点，而且在同一个原语上比 NIXL 还慢：

```bash
ucx_perftest <peer> -t ucp_get    -s 8388608 -n 300 -c 1   #  1511 MB/s  RDMA READ
ucx_perftest <peer> -t tag_bw     -s 8388608 -n 300 -c 1   #  7035 MB/s  双边（rendezvous 走 WRITE）
ucx_perftest <peer> -t ucp_put_bw -s 8388608 -n 300 -c 1   # 16893 MB/s  RDMA WRITE
```

所以只测 `initialize_xfer("READ", ...)` 的 benchmark（[`nixl_check.py`](nixl_check.py) 就是这样）大约只反映了链路能力的八分之一。在 eRDMA 上搬大块数据请用 WRITE。

**没有任何 UCX 参数能救回显存路径。** [`ucx_knob_sweep.sh`](ucx_knob_sweep.sh) 测了所有显而易见的候选项——`UCX_RNDV_FRAG_SIZE`、`UCX_RNDV_FRAG_ALLOC_COUNT`、`UCX_RNDV_FRAG_MEM_TYPES`、`UCX_RNDV_SCHEME`、`UCX_CUDA_COPY_MAX_REG_RATIO`、`UCX_RC_VERBS_MAX_RD_ATOMIC`——没有一个比默认值高出噪声范围，而其中几个会直接腰斩（VRAM READ 场景下 `RNDV_FRAG_MEM_TYPES=cuda`、`RNDV_SCHEME=get_zcopy`、`MAX_RD_ATOMIC=16` 都掉到 0.4 GiB/s 附近）。同时用两块 eRDMA 网卡（`UCX_NET_DEVICES=erdma_0:1,erdma_1:1` 配 `UCX_MAX_RNDV_RAILS=2`）也走不通：agent 能建起来，但第一次传输就在 UCX 断言上崩溃，`ucp_rkey.c:1067 Assertion (&(ep->worker)->async)->signal.tid == ucs_get_tid() failed`。

**在飞请求数和描述符数量都没有影响。** `ucx_perftest -w 1 / -w 8 / -w 32` 三者相差不到 1%（7035 / 7037 / 6953 MB/s）；把 NIXL 的传输拆成 1、4、16、64 个描述符同样毫无变化。这两个是最容易想到的怀疑对象，但在这里都不是原因。

**真正的断崖在显存。** 没有 GPUDirect RDMA 时，UCX 会用 `cuda_copy` 把每个 GPU buffer 经 host 内存中转，这条路在**两个方向上**都掉到 0.26~0.84 GiB/s，比同一链路上 DRAM 的 WRITE 低一到两个数量级。这才是 NIXL 在 GPU 张量上输给 NCCL 的原因，而不是网络本身：NCCL 自己做中转，有专门的 proxy 线程、预注册的 bounce buffer 和多条并行 channel，在同样两台机器上能跑到 11.5 GiB/s。

作为参照，`ecs.ebmgn9gc.64xlarge` 规格上两块 eRDMA 网卡（`EriQuantity: 2`）合计约 380 Gbps，单块约 22 GiB/s——所以 20.4 GiB/s 的 DRAM WRITE 已经接近单卡上限，而 NCCL 的 11.5 GiB/s 只到其一半左右。`/sys/class/infiniband/erdma_0/ports/1/rate` 里显示的 `100 Gb/sec (4X EDR)` 是 IB 风格的名义编码，不是真实上限。

> 关于 WRITE 数据的一点说明：NIXL 在本端队列排空后就把 WRITE 标记为 `DONE`，所以单次计时可能超过线速。上面的数字是用 NIXL notification 卡住的——只有数据真正落到对端后，对端才会收到通知。用普通 TCP 往返是证明不了什么的：socket 与 RDMA 路径之间没有顺序保证。

## RDT over eRDMA（Ray 集成）

[`rdt_nixl_vs_nccl.py`](rdt_nixl_vs_nccl.py) 在两个不同节点上起 GPU actor，通过 Ray Direct Transport 传输 256 MiB CUDA 张量，对比三个后端。实测结果（稳态，256 MiB GPU→GPU）：

| 传输后端 | iter 1 | iter 2 | 状态 |
|---|---|---|---|
| `tensor_transport="nixl"`（UCX 后端） | 4.5 GiB/s | **6.2 GiB/s** | ✅ |
| `tensor_transport="nixl"`（Mooncake 后端） | — | — | ❌ SEGV |
| `tensor_transport="nccl"`（GDR） | 10.5 GiB/s | **11.0 GiB/s** | ✅ |

NIXL/Mooncake 崩溃是 **Ray 代码层的限制**：Ray 2.57 的 `nixl_tensor_transport.py` 第 215 行硬编码了后端选择逻辑 `return "LIBFABRIC" if _is_efa_available() else "UCX"`——只认 UCX 和 LIBFABRIC(AWS EFA)，完全没有 Mooncake 分支。即使把 UCX 插件移走，Ray 仍尝试用 `backend=UCX` 注册显存，底层报 `no available backends for mem type 'VRAM_SEG'` 直接失败。这不是 Mooncake 或 eRDMA 的问题——在 Ray 外（本目录的 `nixl_check.py` / `nixl_tuning_sweep.py`）单独用 Mooncake 后端，GPU WRITE 到 19 GiB/s、GPU READ 到 12 GiB/s。

NIXL/UCX 那条在 Ray 内跑通的路线：6.2 GiB/s 对 NCCL 的 11.0 GiB/s，约为 NCCL 的 56%。瓶颈在 UCX 的 `get_zcopy` 实现每次只允许 1 个 RDMA READ 在飞（参见上面"吞吐归因"一节），而硬件 `ib_read_bw -q 4` 可达 215 Gb/s。未来 UCX 若支持多 outstanding READ 流水线，这个差距会收敛。

## 限制

- **NCCL + GDR 仍然是 Ray 里 GPU 张量最快的选择**（11 GiB/s 对 NIXL/UCX 的 6.2 GiB/s）。只有技术栈里确实需要 NIXL 时（如 vLLM 的 PD 分离）才值得用它。
- **NIXL/Mooncake 在 Ray 内不可用**，原因是上述 Ray 代码硬编码问题。在 Ray 外它是实测最快的路径（GPU WRITE 19 GiB/s）——如果确实需要 Mooncake 集成，方案是只在 plugin 目录留 `libplugin_Mooncake.so` 并 patch Ray 的后端选择逻辑。
- **必须有 GDR** 才能获得以上显存性能。缺少 GDR 时（如 erdma 模块被对着 in-tree ib_core 重编、丢了 `ib_umem_get_peer`）VRAM 路径掉到 < 1 GiB/s。节点必须跑原厂 OFED RDMA 栈 + `nvidia_peermem`。
- 上述两条路都依赖打过 patch 的 UCX 或源码构建的 Mooncake，因此只有在你本来就要自定义镜像时才划算，而这正是 [`Dockerfile`](Dockerfile) 和 [`build-job.yaml`](build-job.yaml) 做的事。
