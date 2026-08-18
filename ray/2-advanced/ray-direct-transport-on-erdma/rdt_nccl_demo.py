"""Ray Direct Transport (RDT) over eRDMA, using the NCCL tensor transport.

Two GPU actors are placed on different nodes (STRICT_SPREAD placement group).
The producer returns a CUDA tensor from a method annotated with
tensor_transport="nccl": Ray leaves the tensor in the producer's GPU memory
and moves it straight into the consumer's GPU with NCCL, which runs on top of
eRDMA. The tensor never enters the Ray object store and is never copied to
host memory.
"""

import time

import ray
import torch
from ray.experimental.collective import create_collective_group
from ray.util.placement_group import placement_group
from ray.util.scheduling_strategies import PlacementGroupSchedulingStrategy

# These can also be set on the worker pods (see ray-cluster-erdma.yaml); they
# are repeated here so the script is self-contained.
NCCL_ENV = {
    # Use the ibverbs (eRDMA) path, not sockets.
    "NCCL_IB_DISABLE": "0",
    # eRDMA device to use.
    "NCCL_IB_HCA": "erdma_0",
    # Interface for NCCL's out-of-band bootstrap.
    "NCCL_SOCKET_IFNAME": "eth0",
    # Handy while validating a new cluster; drop in production.
    "NCCL_DEBUG": "INFO",
    "NCCL_DEBUG_SUBSYS": "INIT,NET",
    "NCCL_DEBUG_FILE": "/tmp/nccl-%h-%p.log",
}

NUM_ELEMS = 128 * 1024 * 1024  # 512 MiB of float32


@ray.remote(num_gpus=1, runtime_env={"env_vars": NCCL_ENV})
class Producer:
    def where(self):
        return ray.get_runtime_context().get_node_id(), torch.cuda.get_device_name(0)

    @ray.method(tensor_transport="nccl")
    def make_tensor(self, n: int):
        return torch.ones(n, dtype=torch.float32, device="cuda")


# enable_tensor_transport=True is required on the *receiving* actor as well.
# It makes Ray run this actor's tasks on a background thread; without it the
# RDT receive is queued behind consume(), which is itself blocked waiting for
# the tensor, and the transfer times out after 60s.
@ray.remote(
    num_gpus=1, enable_tensor_transport=True, runtime_env={"env_vars": NCCL_ENV}
)
class Consumer:
    def where(self):
        return ray.get_runtime_context().get_node_id(), torch.cuda.get_device_name(0)

    def consume(self, tensor: torch.Tensor):
        assert tensor.is_cuda, f"expected a CUDA tensor, got {tensor.device}"
        torch.cuda.synchronize()
        return {
            "device": str(tensor.device),
            "numel": tensor.numel(),
            "sum": float(tensor.sum().item()),
        }


def main():
    ray.init(address="auto")

    # One bundle per node, so the two actors are guaranteed to land on
    # different machines and the transfer crosses the network.
    pg = placement_group(
        [{"GPU": 1, "CPU": 1}, {"GPU": 1, "CPU": 1}], strategy="STRICT_SPREAD"
    )
    ray.get(pg.ready())

    def opts(index):
        return {
            "scheduling_strategy": PlacementGroupSchedulingStrategy(
                placement_group=pg, placement_group_bundle_index=index
            )
        }

    producer = Producer.options(**opts(0)).remote()
    consumer = Consumer.options(**opts(1)).remote()

    p_node, p_gpu = ray.get(producer.where.remote())
    c_node, c_gpu = ray.get(consumer.where.remote())
    print(f"producer node={p_node} gpu={p_gpu}")
    print(f"consumer node={c_node} gpu={c_gpu}")
    assert p_node != c_node, "actors landed on the same node"

    # The NCCL transport needs an explicit collective group over the actors
    # that will exchange tensors.
    create_collective_group([producer, consumer], backend="nccl")

    nbytes = NUM_ELEMS * 4
    for i in range(3):
        gpu_ref = producer.make_tensor.remote(NUM_ELEMS)
        start = time.perf_counter()
        result = ray.get(consumer.consume.remote(gpu_ref))
        elapsed = time.perf_counter() - start
        assert result["sum"] == float(NUM_ELEMS), result["sum"]
        print(
            f"iter {i}: {nbytes / 2**20:.0f} MiB -> {result['device']} "
            f"in {elapsed * 1000:.1f} ms ({nbytes / elapsed / 2**30:.2f} GiB/s), "
            f"checksum OK"
        )

    print("RDT over eRDMA: OK")


if __name__ == "__main__":
    main()
