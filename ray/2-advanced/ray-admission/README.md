# Ray Admission

Ray Admission is an experimental, standalone Python package for controlling
Ray task and actor submissions with per-Job resource quotas. The controller is
a detached Ray actor named `ray-submission-queue` in the fixed
`ray-submission-queue` Ray namespace. The `RAY_NAMESPACE` environment variable
selects the **quota namespace**; it does not change the controller location.

See [中文使用说明](README_ZH.md) for the complete workflow.
See [controller lifecycle and storage design](DESIGN.md) for failure handling,
retry guarantees, and the GCS KV record storage implementation.

## Install

Install the same wheel into the administrator, driver, and Ray worker Python
environments. The package uses Ray private Python interfaces and was validated
with Ray 2.58.

```bash
python build_wheel.py
python -m pip install dist/ray_admission-0.1.0-py3-none-any.whl
ray-admission list
```

## Start and configure

```bash
export RAY_ADMISSION_ADMIN_TOKEN='a-long-random-secret'
ray-admission queue --address auto start
export RAY_NAMESPACE=my-team
ray-admission queue --address auto quota set <ray-job-id> '{"CPU": 4, "GPU": 1}'
ray-admission queue --address auto status
```

The Job ID is `ray.get_runtime_context().get_job_id()` in the business driver.
Unconfigured `(quota namespace, Job ID)` pairs are unlimited. `quota set`
replaces the full resource vector; `quota delete <ray-job-id>` removes it.
Quota configuration, the administrator token digest, and active resource
reservations are saved as individual Ray GCS KV records. The detached controller restarts after an
unexpected actor failure and restores task ObjectRefs and actor IDs to monitor
their completion. In-flight admission calls are retried by Ray; waiting requests
re-enter the per-Job queue. Counters and the exact pre-crash waiting order are
not restored. A reservation granted just before a crash but never tracked by
its submitter remains reserved until that submitter releases it. Cluster-wide
durability follows the Ray GCS persistence configuration. No external database
is required. Saved ObjectRefs cannot be transferred between clusters. Released records are
compacted below the client's durable acknowledgment watermark. Each process's
session watermark remains stored to reject delayed retries.
Earlier GCS snapshots are migrated before serving requests after the old
controller stops. Partial record writes are recovered using a redo journal;
GCS KV does not provide multi-key transactions or independent writer fencing.
An actor that died while the controller was down is released when its GCS state
reports `DEAD`; if that state is unavailable, its reservation remains held.

## Enable in a business driver

```python
import ray
from ray_admission.queue import enable_admission

ray.init(address="auto")
enable_admission()

@ray.remote(num_cpus=1)
def work():
    return "done"

print(ray.get(work.remote()))
```

Set `RAY_NAMESPACE=my-team` in the driver (for example with the Kubernetes
Downward API) to use the quota configured for `my-team`. Configure its quota
before it submits work. New task and actor workers inherit this quota namespace
through the worker startup hook.

The queue uses cooperative Python interception; it does not provide isolation
from untrusted code or support for Ray Client, Java, or C++ submissions.
