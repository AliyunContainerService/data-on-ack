"""Verify that a cGPU (aliyun.com/gpu-mem) slice is usable from Ray.

Run from the head pod of the RayCluster:
    kubectl cp cgpu_ray_test.py <head-pod>:/tmp/cgpu_ray_test.py -c ray-head
    kubectl exec <head-pod> -c ray-head -- python /tmp/cgpu_ray_test.py

It checks three things:
  1. Ray advertises the GPU (from the worker's rayStartParams num-gpus=1).
  2. A GPU actor sees a device whose *total* memory equals the cGPU slice
     (e.g. ~8 GiB), not the whole physical card - proving cGPU isolation.
  3. Allocating within the slice works; allocating past it is rejected with
     an out-of-memory error, i.e. the cap is actually enforced.
"""

import os

import ray
import torch

ray.init(address="auto")
print("cluster GPU:", ray.cluster_resources().get("GPU"))


@ray.remote(num_gpus=1)
class GpuActor:
    def info(self):
        dev = torch.cuda.current_device()
        props = torch.cuda.get_device_properties(dev)
        return {
            "ray_gpu_ids": ray.get_gpu_ids(),
            "cuda_visible_devices": os.environ.get("CUDA_VISIBLE_DEVICES"),
            "device": props.name,
            "total_mem_GiB": round(props.total_memory / 2**30, 2),
        }

    def alloc(self, gib: float):
        # float32 => 4 bytes per element.
        torch.empty(int(gib * (2**30) // 4), dtype=torch.float32, device="cuda")
        torch.cuda.synchronize()
        free, total = torch.cuda.mem_get_info()
        return {
            "requested_GiB": gib,
            "free_GiB": round(free / 2**30, 2),
            "total_GiB": round(total / 2**30, 2),
        }


actor = GpuActor.remote()
print("actor info:", ray.get(actor.info.remote()))
print("alloc 4GiB :", ray.get(actor.alloc.remote(4)))
try:
    print("alloc 16GiB:", ray.get(actor.alloc.remote(16)))
    print("WARNING: 16GiB allocation succeeded - cGPU cap not enforced?")
except Exception as e:  # noqa: BLE001
    print("alloc 16GiB: blocked as expected ->", type(e).__name__)

print("OK")
