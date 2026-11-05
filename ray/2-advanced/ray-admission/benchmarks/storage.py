"""PYTHONPATH=src python benchmarks/storage.py; uses a local Ray GCS."""

import json
import statistics
import time
import uuid

import ray
from ray.experimental.internal_kv import _internal_kv_del
from ray_admission._storage import GcsStore


def main():
    ray.init(num_cpus=1, include_dashboard=False)
    results = []
    try:
        for count in (10, 100, 1000):
            namespace = "admission-benchmark-" + uuid.uuid4().hex
            store = GcsStore("benchmark", namespace=namespace)
            record = {
                "state": "RESERVED",
                "key": ["team", "job"],
                "resources": {"CPU": 1},
                "revision": 1,
            }
            for offset in range(0, count, 32):
                store.commit(
                    [
                        (f"permit/{i:06d}", record)
                        for i in range(offset, min(count, offset + 32))
                    ]
                )
            durations = []
            before = store.bytes_written
            writes = store.kv_writes
            for revision in range(2, 102):
                started = time.perf_counter()
                store.commit([("permit/000000", {**record, "revision": revision})])
                durations.append((time.perf_counter() - started) * 1000)
            started = time.perf_counter()
            restored = store.load()
            results.append(
                {
                    "active_records": count,
                    "bytes_per_update": (store.bytes_written - before) / 100,
                    "kv_writes_per_update": (store.kv_writes - writes) / 100,
                    "p50_ms": round(statistics.median(durations), 3),
                    "p95_ms": round(sorted(durations)[94], 3),
                    "recovery_ms": round((time.perf_counter() - started) * 1000, 3),
                    "restored": len(restored),
                }
            )
            _internal_kv_del(b"", del_by_prefix=True, namespace=namespace)
        print(json.dumps(results, indent=2))
    finally:
        ray.shutdown()


if __name__ == "__main__":
    main()
