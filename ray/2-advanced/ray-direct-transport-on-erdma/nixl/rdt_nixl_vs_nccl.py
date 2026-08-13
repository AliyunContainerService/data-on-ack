"""RDT over eRDMA: compare nixl (UCX backend), nixl (Mooncake backend), and nccl.

Two GPU actors placed on different nodes via STRICT_SPREAD. For each transport
the producer returns a CUDA tensor and the consumer receives it directly in GPU
memory, bypassing the object store.

Usage from the head pod:
    python /tmp/rdt_nixl_vs_nccl.py
"""

import os
import time

import ray
import torch
from ray.util.placement_group import placement_group
from ray.util.scheduling_strategies import PlacementGroupSchedulingStrategy

# For NCCL transport we need create_collective_group
from ray.experimental.collective import create_collective_group

NUM_ELEMS = 64 * 1024 * 1024  # 256 MiB of float32

# UCX env for the eRDMA patched stack
UCX_ENV = {
    "UCX_RC_VERBS_USE_SRQ": "n",
    "UCX_RC_VERBS_RX_CQ_LEN": "131072",
    "UCX_UD_VERBS_TX_MIN_INLINE": "128",
    "UCX_NET_DEVICES": "erdma_0:1",
    "UCX_TLS": "rc_verbs,ud_verbs,self,sm,cuda_copy,cuda_ipc",
    "NIXL_PLUGIN_DIR": "/opt/nixl/lib/x86_64-linux-gnu/plugins",
}

MOONCAKE_ENV = {
    "MC_METADATA_SERVER": "http://10.0.5.75:18080/metadata",
    "MC_PROTOCOL": "rdma",
    "NIXL_PLUGIN_DIR": "/opt/nixl/lib/x86_64-linux-gnu/plugins",
}

NCCL_ENV = {
    "NCCL_IB_DISABLE": "0",
    "NCCL_IB_HCA": "erdma_0",
    "NCCL_SOCKET_IFNAME": "eth0",
    "NCCL_NET_GDR_LEVEL": "SYS",
}


def make_actors(transport, env):
    """Create Producer and Consumer actor classes for a given transport."""

    @ray.remote(num_gpus=1, runtime_env={"env_vars": env})
    class Producer:
        def node(self):
            return ray.get_runtime_context().get_node_id()

        @ray.method(tensor_transport=transport)
        def make_tensor(self, n: int):
            return torch.ones(n, dtype=torch.float32, device="cuda")

    @ray.remote(num_gpus=1, enable_tensor_transport=True, runtime_env={"env_vars": env})
    class Consumer:
        def node(self):
            return ray.get_runtime_context().get_node_id()

        def consume(self, tensor: torch.Tensor):
            assert tensor.is_cuda, f"got {tensor.device}"
            torch.cuda.synchronize()
            return {"device": str(tensor.device), "sum": float(tensor.sum().item())}

    return Producer, Consumer


def run_test(transport, env, pg, label, need_collective=False):
    print(f"\n{'='*60}")
    print(f"  {label}  (tensor_transport={transport!r})")
    print(f"{'='*60}")

    Producer, Consumer = make_actors(transport, env)

    def opts(i):
        return {"scheduling_strategy": PlacementGroupSchedulingStrategy(
            placement_group=pg, placement_group_bundle_index=i)}

    producer = Producer.options(**opts(0)).remote()
    consumer = Consumer.options(**opts(1)).remote()

    p_node = ray.get(producer.node.remote())
    c_node = ray.get(consumer.node.remote())
    print(f"  producer node={p_node[:12]}  consumer node={c_node[:12]}")
    assert p_node != c_node, "same node!"

    if need_collective:
        create_collective_group([producer, consumer], backend="nccl")

    nbytes = NUM_ELEMS * 4
    for i in range(3):
        ref = producer.make_tensor.remote(NUM_ELEMS)
        t0 = time.perf_counter()
        result = ray.get(consumer.consume.remote(ref))
        elapsed = time.perf_counter() - t0
        expected = float(NUM_ELEMS)
        assert result["sum"] == expected, f"got {result['sum']}"
        gbs = nbytes / elapsed / 2**30
        print(f"  iter {i}: {nbytes/2**20:.0f} MiB in {elapsed*1000:.1f} ms "
              f"({gbs:.2f} GiB/s)")

    # Cleanup actors
    ray.kill(producer)
    ray.kill(consumer)
    print(f"  {label}: OK")


def main():
    ray.init(address="auto")
    print(f"cluster resources: {ray.cluster_resources()}")

    pg = placement_group(
        [{"GPU": 1, "CPU": 1}, {"GPU": 1, "CPU": 1}], strategy="STRICT_SPREAD"
    )
    ray.get(pg.ready())

    # 1) NIXL with UCX backend
    try:
        run_test("nixl", UCX_ENV, pg, "NIXL / UCX")
    except Exception as e:
        print(f"  NIXL/UCX FAILED: {e}")

    # 2) NIXL with Mooncake backend (needs metadata server running on 10.0.5.75:18080)
    try:
        run_test("nixl", MOONCAKE_ENV, pg, "NIXL / Mooncake")
    except Exception as e:
        print(f"  NIXL/Mooncake FAILED: {e}")

    # 3) NCCL (control)
    try:
        run_test("nccl", NCCL_ENV, pg, "NCCL (GDR)", need_collective=True)
    except Exception as e:
        print(f"  NCCL FAILED: {e}")

    print("\n=== ALL DONE ===")


if __name__ == "__main__":
    main()
