# NIXL on eRDMA

The main guide uses the NCCL tensor transport because the NIXL transport does
not work out of the box on eRDMA. This directory documents the steps required
to make NIXL work on eRDMA and the measured results. Both approaches below were
validated on two `ecs.ebmgn9gc.64xlarge` nodes (RTX PRO 5000, two eRDMA
interfaces, ~380 Gbps in total).

## Quick start: use the prebuilt image

Everything below is built into an image by [`Dockerfile`](Dockerfile), built
in-cluster by [`build-job.yaml`](build-job.yaml):

```bash
kubectl apply -f build-job.yaml     # creates the context PVC and the Job
kubectl logs -f job/nixl-image-build
```

The build runs as a Job rather than through `kubectl exec` so that it survives
losing your network, and it takes ~10 minutes with a warm BuildKit cache. Stage
the source tarballs on the context PVC first - the header of the Dockerfile
lists every file and where to fetch it, since github is not reachable from
inside the cluster. The resulting image carries `/opt/ucx-erdma`,
`/opt/nixl`, `libtransfer_engine.so` and the `UCX_*` settings the patch needs,
so `ucx_perftest` and `import nixl` work without additional setup.

The two shell scripts, [`build_ucx_erdma.sh`](build_ucx_erdma.sh) and
[`build_nixl_mooncake.sh`](build_nixl_mooncake.sh), do the same steps
interactively inside a running pod; they are the more convenient option while
iterating on flags.

## Why stock NIXL fails

The prebuilt `nixl` / `nixl-cu13` wheels bundle their own UCX (1.21) and their
own renamed copy of `libibverbs`. Two things break:

1. The bundled `libibverbs` has no provider configuration, so UCX sees no RDMA
   device at all (`network device 'erdma_0:1' is not available`). Pointing the
   bundled copy at the system library is enough to fix this:

   ```bash
   D=$(python -c "import nixl_cu13, os; print(os.path.dirname(nixl_cu13.__file__)+'.libs')")
   ln -sf /usr/lib/x86_64-linux-gnu/libibverbs.so.1 "$D"/libibverbs-*.so.*
   ```

2. UCX's `rc_verbs` transport unconditionally creates a shared receive queue,
   and eRDMA does not implement SRQ:

   ```
   UCX ERROR erdma_0: ibv_create_srq() failed: Operation not supported
   ```

   There is no runtime workaround; UCX has to be patched.

## Node prerequisites

Both paths need the eRDMA driver in compatibility mode, and a driver new
enough to report SRQ capability and support UD emulation:

```bash
cat /sys/module/erdma/parameters/compat_mode        # must be Y
ibv_devinfo -d erdma_0 -v | grep max_srq            # must be > 0
```

If `compat_mode` is `N`, or `max_srq` is 0, reinstall and reload the driver:

```bash
wget http://mirrors.cloud.aliyuncs.com/erdma/env_setup.sh
bash env_setup.sh --url http://mirrors.cloud.aliyuncs.com/erdma/erdma_installer-1.5.4.tar.gz
rmmod erdma && modprobe erdma compat_mode=Y
printf 'options erdma compat_mode=Y\n' > /etc/modprobe.d/erdma-compat.conf
```

> On a node where NVIDIA OFED sources are present in `/usr/src/ofa_kernel` but
> the running kernel uses the in-tree `ib_core`, the DKMS build picks up the
> OFED symbols and the module then refuses to load
> (`erdma: Unknown symbol ib_umem_get_peer`). Hide the OFED tree during the
> build: `mv /usr/src/ofa_kernel /usr/src/ofa_kernel.hidden`, rerun
> `dkms build/install`, move it back, `depmod -a`.

Also make sure `ulimit -l` is `unlimited` inside the pod - see the memlock
section of the main guide.

## Path 1: rebuild UCX with the eRDMA patch

Alibaba Cloud publishes a UCX patch that replaces the SRQ with per-QP receive
queues. [`build_ucx_erdma.sh`](build_ucx_erdma.sh) applies it and builds UCX.
The patch is written against 1.19.0 but applies cleanly to 1.21.0, which is
what NIXL 1.3 is compiled against.

```bash
./build_ucx_erdma.sh 1.21.0 /opt/ucx121-erdma
```

Required environment at runtime:

```bash
export UCX_RC_VERBS_USE_SRQ=n
export UCX_RC_VERBS_RX_CQ_LEN=131072
export UCX_UD_VERBS_TX_MIN_INLINE=128
export UCX_NET_DEVICES=erdma_0:1
```

Verify with UCX's own benchmark, on two nodes (note that `tcp` is absent from
`UCX_TLS`, so a result at all proves eRDMA was used):

```bash
export UCX_TLS=rc_verbs,ud_verbs,self,sm
# node A
/opt/ucx121-erdma/bin/ucx_perftest -c 0
# node B
/opt/ucx121-erdma/bin/ucx_perftest <node-A-ip> -t tag_bw -s 8388608 -n 300 -c 1
```

```
Final:  300  1133.820  1135.543  1135.543  7045.09  7045.09  881  881
ucp_worker.c:1903 UCX INFO perftest inter-node cfg#0 tag(rc_verbs/erdma_0:1)
```

7.0 GB/s over `rc_verbs/erdma_0:1`.

### Plugging the patched UCX under the prebuilt NIXL wheel

The wheel's UCX plugin dlopens UCX from the wheel's own lib directory, and UCX
in turn loads its transport modules from the directory next to the library it
was loaded from. Both have to be redirected:

```bash
D=$(python -c "import nixl_cu13, os; print(os.path.dirname(nixl_cu13.__file__)+'.libs')")
for n in ucm ucp ucs uct; do ln -sf /opt/ucx121-erdma/lib/lib$n.so.0 "$D"/lib$n-*.so.0.0.0; done
for f in /opt/ucx121-erdma/lib/ucx/*.so; do ln -sf "$f" "$D/ucx/$(basename "$f")"; done
ln -sf /usr/lib/x86_64-linux-gnu/libibverbs.so.1 "$D"/libibverbs-*.so.*
```

Two requirements: the UCX build must be 1.21 (1.19 is missing
`ucp_device_mem_list_release`, which the plugin needs), and it must be
configured with `--enable-mt`, otherwise NIXL fails with "UCX library does not
support multi-threading".

Building NIXL from source against `/opt/ucx121-erdma` (see
[`build_nixl_mooncake.sh`](build_nixl_mooncake.sh)) avoids all of this and is
the cleaner option if you are producing an image anyway.

## Path 2: NIXL with the Mooncake backend

The Mooncake Transfer Engine talks to verbs directly and does not use SRQ, so
it works on eRDMA with an unpatched stack. NIXL ships a Mooncake plugin, but
it is not compiled into the wheels and it links against
`libtransfer_engine.so`, which the `mooncake-transfer-engine` PyPI package
does not contain - Mooncake has to be built from source.
[`build_nixl_mooncake.sh`](build_nixl_mooncake.sh) does that and then builds
NIXL with both the UCX and Mooncake backends.

Mooncake needs a metadata service. The simplest one ships with the pip
package:

```bash
python -m mooncake.http_metadata_server --port 18080
```

Then, on each node:

```bash
export NIXL_PLUGIN_DIR=/opt/nixl/lib/x86_64-linux-gnu/plugins
export LD_LIBRARY_PATH=/opt/nixl/lib/x86_64-linux-gnu:/opt/ucx121-erdma/lib:/usr/local/lib:$LD_LIBRARY_PATH
export MC_METADATA_SERVER=http://<metadata-node>:18080/metadata
export MC_PROTOCOL=rdma
```

A quick check of the transfer engine alone (bypassing NIXL) is also useful;
the benchmark binary is in the pip package:

```bash
# CUDA 12 runtime is needed by the prebuilt binary: pip install nvidia-cuda-runtime-cu12
transfer_engine_bench --mode=target    --use_vram=false --buffer_size=4294967296 \
  --protocol=rdma --device_name=erdma_0 --metadata_server=http://...:18080/metadata \
  --local_server_name=<A>:12420
transfer_engine_bench --mode=initiator --use_vram=false --buffer_size=4294967296 \
  --protocol=rdma --device_name=erdma_0 --metadata_server=http://...:18080/metadata \
  --segment_id=<A>:12420 --local_server_name=<B>:12421 --duration=10 \
  --operation=read --threads=12 --block_size=1048576
# Test completed: duration 10.39, batch count 241, throughput 2.90 GiB/s
```

Keep the `--buffer_size` of both sides at least `threads * batch_size *
block_size`; otherwise the initiator reads past the end of the remote segment
and every request fails.

## Cross-node NIXL check

[`nixl_check.py`](nixl_check.py) registers a 256 MiB buffer on each side,
exchanges the agent metadata over a plain TCP socket and issues a NIXL READ.
It works with either backend:

```bash
# target
python nixl_check.py --role target    --backend UCX      --mem vram
# initiator
python nixl_check.py --role initiator --backend UCX      --mem vram --peer <target-ip>
```

Measured on the validated setup (256 MiB, best of 5, cross-node). Both
directions and both memory types, because they differ enormously:

| backend  | memory | READ | WRITE |
|----------|--------|------|-------|
| UCX      | DRAM   | 2.7 GiB/s | 19-20 GiB/s |
| UCX      | VRAM   | 0.4-0.85 GiB/s | 0.3-0.9 GiB/s |
| Mooncake | DRAM   | 4.1 GiB/s | not measured |
| Mooncake | VRAM   | not supported | not supported |

The VRAM rows are given as ranges because they vary by more than 2x between
otherwise identical runs; the DRAM rows are stable to within a few percent.

The eRDMA device counters move by exactly the transferred volume, confirming
the data path:

```bash
cat /sys/class/infiniband/erdma_0/ports/1/hw_counters/hw_rx_bytes_cnt
```

Note that Mooncake may pick `erdma_1` on multi-ENI instances, so check both
devices' counters.

## Verifying whether traffic uses RDMA or falls back to TCP

This should be checked before relying on any of these numbers, since UCX falls
back silently. [`verify_transport.sh`](verify_transport.sh) determines it two
ways: it diffs the eRDMA device byte counters around each run, and records the
lane UCX picked from its own `UCX_LOG_LEVEL=info` output. It then repeats
everything with a TCP-only transport list as a control.

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

The counters move by the payload volume on the RDMA rows and by zero on the TCP
rows, so the RDMA numbers are reliable. Two conclusions follow:

- **RDMA is advantageous only for the DRAM WRITE case** (19.3 vs 5.7 GiB/s). For
  a one-sided READ of host memory, plain TCP is *faster* than eRDMA
  (3.6 vs 2.7 GiB/s).
- **For GPU memory, TCP exceeds eRDMA in both directions** (0.54-0.58 vs
  0.30-0.41 GiB/s). The `cuda_copy` staging overhead dominates, so the underlying
  fabric has relatively little effect.

Note that TCP needs an ethernet device: with `UCX_NET_DEVICES=erdma_0:1` the
tcp transport has nothing to run on and backend creation fails outright, which
is a useful extra check that the RDMA-only rows could not have used TCP.

## Throughput bottleneck analysis

[`nixl_tuning_sweep.py`](nixl_tuning_sweep.py) sweeps transfer direction and
descriptor count; it produced the numbers above. Three findings are relevant
before drawing conclusions about NIXL's performance.

**The bottleneck is RDMA READ, not NIXL.** A one-sided READ of 256 MiB gets
2.7 GiB/s while the same stack doing a WRITE gets 20.4 GiB/s. Raw UCX
confirms this, and is in fact *slower* than NIXL on the same primitive:

```bash
ucx_perftest <peer> -t ucp_get    -s 8388608 -n 300 -c 1   #  1511 MB/s  RDMA READ
ucx_perftest <peer> -t tag_bw     -s 8388608 -n 300 -c 1   #  7035 MB/s  two-sided (rendezvous WRITE)
ucx_perftest <peer> -t ucp_put_bw -s 8388608 -n 300 -c 1   # 16893 MB/s  RDMA WRITE
```

So a benchmark that only exercises `initialize_xfer("READ", ...)` - as
[`nixl_check.py`](nixl_check.py) does - reflects roughly one eighth of the link
capacity. Use WRITE for bulk data on eRDMA.

**No UCX knob improves the GPU path.** [`ucx_knob_sweep.sh`](ucx_knob_sweep.sh)
measures the obvious candidates - `UCX_RNDV_FRAG_SIZE`,
`UCX_RNDV_FRAG_ALLOC_COUNT`, `UCX_RNDV_FRAG_MEM_TYPES`, `UCX_RNDV_SCHEME`,
`UCX_CUDA_COPY_MAX_REG_RATIO`, `UCX_RC_VERBS_MAX_RD_ATOMIC` - and none of them
exceeds the default beyond noise, while several halve throughput
(`RNDV_FRAG_MEM_TYPES=cuda`, `RNDV_SCHEME=get_zcopy` and `MAX_RD_ATOMIC=16` all
land near 0.4 GiB/s on VRAM READ). Using both eRDMA interfaces
(`UCX_NET_DEVICES=erdma_0:1,erdma_1:1` with `UCX_MAX_RNDV_RAILS=2`) does not
work either: the agent starts but the first transfer aborts on a UCX
assertion, `ucp_rkey.c:1067 Assertion (&(ep->worker)->async)->signal.tid ==
ucs_get_tid() failed`.

**Neither the in-flight window nor the descriptor count matters.**
`ucx_perftest -w 1 / -w 8 / -w 32` land within 1% of each other
(7035 / 7037 / 6953 MB/s), and splitting the NIXL transfer into 1, 4, 16 or 64
descriptors changes nothing either. Both are the obvious candidates, but neither
is the cause here.

**GPU memory is the primary bottleneck.** Without GPUDirect RDMA, UCX stages every GPU
buffer through host memory with `cuda_copy`, and that path collapses to
0.26-0.84 GiB/s in *both* directions - one to two orders of magnitude below the
DRAM WRITE path on the same link. This, rather than the network, is why NIXL is
slower than NCCL for GPU tensors: NCCL performs its own staging with a dedicated
proxy thread, pre-registered bounce buffers and many parallel channels, and
reaches 11.5 GiB/s GPU-to-GPU between the same two nodes.

For reference, `ecs.ebmgn9gc.64xlarge` is specified at ~380 Gbps across its two
eRDMA interfaces (`EriQuantity: 2`), i.e. roughly 22 GiB/s per device - so the
20.4 GiB/s DRAM WRITE is near the per-device ceiling, and NCCL's 11.5 GiB/s is
about half of it. The `100 Gb/sec (4X EDR)` reported by
`/sys/class/infiniband/erdma_0/ports/1/rate` is a nominal IB encoding, not the
real cap.

> Caveat on the WRITE numbers: NIXL reports a WRITE as `DONE` once the local
> side is drained, so per-operation timings can exceed the wire rate. The
> figures above are bracketed by a NIXL notification, which the target receives
> only after the data has landed. A plain TCP round trip would prove nothing -
> the socket is not ordered against the RDMA path.

## RDT over eRDMA (Ray integration)

[`rdt_nixl_vs_nccl.py`](rdt_nixl_vs_nccl.py) deploys two GPU actors on
different nodes and moves a 256 MiB CUDA tensor through Ray Direct Transport,
comparing all three backends. Results (steady-state, 256 MiB GPU→GPU):

| transport | iter 1 | iter 2 | status |
|---|---|---|---|
| `tensor_transport="nixl"` (UCX backend) | 4.5 GiB/s | **6.2 GiB/s** | ✅ |
| `tensor_transport="nixl"` (Mooncake backend) | — | — | ❌ SEGV |
| `tensor_transport="nccl"` (GDR) | 10.5 GiB/s | **11.0 GiB/s** | ✅ |

The NIXL/Mooncake failure is a **Ray-side limitation**: line 215 of Ray 2.57's
`nixl_tensor_transport.py` hard-codes the backend selection as
`return "LIBFABRIC" if _is_efa_available() else "UCX"` — it supports only UCX
and LIBFABRIC (AWS EFA), with no Mooncake branch. Even if the UCX plugin is
removed, Ray still attempts to register GPU memory with `backend=UCX`, and the
lower layer fails with `no available backends for mem type 'VRAM_SEG'`. This is
not a Mooncake or eRDMA problem — outside Ray (the `nixl_check.py` /
`nixl_tuning_sweep.py` scripts in this directory) the Mooncake backend alone
reaches 19 GiB/s GPU WRITE and 12 GiB/s GPU READ.

For the NIXL/UCX path that does work end-to-end in Ray: 6.2 GiB/s versus NCCL's
11.0 GiB/s means NIXL/UCX is currently about 56% of NCCL speed. The bottleneck
is UCX's `get_zcopy` implementation allowing only one RDMA READ in flight at a
time (see "Throughput bottleneck analysis" above), while the hardware can
sustain 4× that (`ib_read_bw -q 4` = 215 Gb/s). A future UCX that pipelines
multiple outstanding READs would close this gap.

## Limitations

- **NCCL with GDR remains the highest-throughput option for GPU tensors in
  Ray** (11 GiB/s versus NIXL/UCX's 6.2 GiB/s). Use NIXL only when the stack
  requires it (e.g. vLLM PD disaggregation).
- **NIXL/Mooncake does not work inside Ray** due to the hard-coded backend
  selection described above. Outside Ray it delivers 19 GiB/s GPU WRITE, the
  highest measured throughput. If Mooncake integration is required, build NIXL
  with only the Mooncake plugin (remove `libplugin_UCX.so` from the plugin
  directory) and patch Ray's backend selection.
- **The GPU numbers above require GDR.** Without it (e.g. when the erdma module
  was rebuilt against in-tree ib_core, losing `ib_umem_get_peer`), VRAM paths
  drop below 1 GiB/s. The nodes must run the original OFED RDMA stack with
  `nvidia_peermem` loaded.
- Both approaches require a patched UCX or a source-built Mooncake, so they are
  warranted only when building a custom image, which is what
  [`Dockerfile`](Dockerfile) and [`build-job.yaml`](build-job.yaml) provide.
