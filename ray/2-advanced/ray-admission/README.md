# Ray Admission

Ray Admission is an experimental, standalone Python package for controlling
Ray task and actor submissions with per-Job resource quotas. The controller is
a detached Ray actor named `ray-submission-queue` in the fixed
`ray-submission-queue` Ray namespace. The `RAY_NAMESPACE` environment variable
selects the **quota namespace**; it does not change the controller location.

See [中文使用说明](README_ZH.md) for the complete workflow.

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

## Enable in a business driver

```python
import ray
from ray_admission.queue import enable_admission

ray.init(address="auto")
print(ray.get_runtime_context().get_job_id())
input("Configure the Job quota, then press Enter: ")
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
from untrusted code, durable controller state, or support for Ray Client, Java,
or C++ submissions.
