import ray
from ray.experimental.internal_kv import _internal_kv_del


def configure_storage():
    if ray.is_initialized():
        _internal_kv_del(b"", del_by_prefix=True, namespace="ray-submission-queue")
