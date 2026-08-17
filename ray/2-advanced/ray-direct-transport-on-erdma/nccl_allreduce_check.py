"""Two-process NCCL all_reduce over eRDMA, without Ray in the picture.

Useful to separate "eRDMA/NCCL is broken" from "Ray RDT is misconfigured".
Run it in the two pods created by erdma-nccl-check-pods.yaml:

    CMD='export NCCL_IB_DISABLE=0 NCCL_IB_HCA=erdma_0 NCCL_SOCKET_IFNAME=eth0 \
        MASTER_ADDR=<node-a-ip> MASTER_PORT=29600 WORLD_SIZE=2; \
        python /tmp/nccl_allreduce_check.py'
    kubectl exec erdma-nccl-b -- bash -lc "export RANK=1; $CMD" &
    kubectl exec erdma-nccl-a -- bash -lc "export RANK=0; $CMD"

Set NCCL_IB_DISABLE=1 for a TCP baseline: the run still succeeds, but the
device counters (hw_tx_bytes_cnt) stay flat.
"""

import resource
import time

import torch
import torch.distributed as dist

# RDMA needs pinned memory. If this prints 65536 instead of -1 (unlimited),
# containerd on this node was started with the systemd default LimitMEMLOCK
# and ibv_create_qp will fail with ENOMEM.
print("memlock:", resource.getrlimit(resource.RLIMIT_MEMLOCK), flush=True)

dist.init_process_group(backend="nccl")
rank = dist.get_rank()
torch.cuda.set_device(0)

n = 128 * 1024 * 1024  # 512 MiB of float32
buf = torch.full((n,), float(rank + 1), device="cuda")
torch.cuda.synchronize()

for i in range(3):
    start = time.perf_counter()
    dist.all_reduce(buf)
    torch.cuda.synchronize()
    elapsed = time.perf_counter() - start
    if rank == 0:
        print(
            f"iter {i}: all_reduce {n * 4 / 2**30:.2f} GiB in {elapsed * 1000:.1f} ms",
            flush=True,
        )

print(f"rank {rank} OK sum={float(buf[0])}", flush=True)
dist.destroy_process_group()
