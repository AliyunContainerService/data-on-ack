"""Run against a dedicated etcd: PYTHONPATH=src python benchmarks/storage.py."""

import json
import os
import statistics
import time
import uuid

from ray_admission._storage import EtcdStore, _b64


def main():
    results = []
    for count in (10, 100, 1000):
        store = EtcdStore(
            os.environ["RAY_ADMISSION_ETCD_ENDPOINT"],
            "/admission-benchmark/" + uuid.uuid4().hex,
            "benchmark",
        )
        record = {
            "state": "RESERVED",
            "key": ["team", "job"],
            "resources": {"CPU": 1},
            "revision": 1,
        }
        try:
            for offset in range(0, count, 32):
                store.commit(
                    [
                        (f"permit/{i:06d}", record)
                        for i in range(offset, min(count, offset + 32))
                    ]
                )
            durations = []
            before = store.bytes_written
            for revision in range(2, 102):
                started = time.perf_counter()
                store.commit([("permit/000000", {**record, "revision": revision})])
                durations.append((time.perf_counter() - started) * 1000)
            bytes_per_transition = (store.bytes_written - before) / 100
            started = time.perf_counter()
            restored = store.load()
            recovery_ms = (time.perf_counter() - started) * 1000
            results.append(
                {
                    "active_records": count,
                    "bytes_per_transition": bytes_per_transition,
                    "p50_ms": round(statistics.median(durations), 3),
                    "p95_ms": round(sorted(durations)[94], 3),
                    "recovery_ms": round(recovery_ms, 3),
                    "restored": len(restored),
                }
            )
        finally:
            store._call(
                "deleterange",
                {"key": _b64(store.prefix), "range_end": _b64(store.prefix[:-1] + "0")},
            )
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
