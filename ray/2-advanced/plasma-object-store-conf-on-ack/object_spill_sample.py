import subprocess
import time

import numpy as np
import ray

ray.init(address="auto")

# Each object is a 1024x1024 float64 matrix, i.e. 8 MiB.
# The object store on each node is capped at 1 GiB (object-store-memory),
# so putting 240 objects (~1.88 GiB) forces Ray to spill the overflow
# into the configured object_spilling_directory (/spill).
NUM_OBJECTS = 240
arr = np.ones((1024, 1024), dtype=np.float64)

refs = [ray.put(arr) for _ in range(NUM_OBJECTS)]
print(f"Put {len(refs)} objects, ~{len(refs) * 8} MiB total")

# Spilling happens asynchronously; give the raylet a moment to settle.
time.sleep(10)

# While the references are alive, the objects that do not fit into the
# in-memory object store occupy the spill directory.
print("Spill directory usage while the objects are alive:")
subprocess.run(["du", "-sh", "/spill"])

# Keep the references alive so you can also inspect the cluster from
# another terminal, e.g.:
#   kubectl exec <head-pod> -- ray memory --stats-only
print("Holding the references for 30 seconds; inspect the cluster from another terminal...")
time.sleep(30)

# When the driver exits, Ray releases the references and deletes the
# spilled files, so /spill will be emptied again.
print("Driver exiting; references released.")
print("The spilled objects under /spill are deleted once the script exits.")
