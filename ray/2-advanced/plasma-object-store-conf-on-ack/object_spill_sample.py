import numpy as np
import ray

ray.init(address="auto")

# Each object is a 1024x1024 float64 matrix, i.e. 8 MiB.
# The object store on each node is capped at 1 GiB (object-store-memory),
# so putting 240 objects (~1.88 GiB) forces Ray to spill the overflow
# to the configured object_spilling_directory (/spill).
NUM_OBJECTS = 240
arr = np.ones((1024, 1024), dtype=np.float64)

refs = [ray.put(arr) for _ in range(NUM_OBJECTS)]
print(f"Put {len(refs)} objects, ~{len(refs) * 8} MiB total")

# Keep the references alive. Check the spill stats afterwards with:
#   ray memory --stats-only
print("All objects are alive. Check spill stats with 'ray memory --stats-only'.")
