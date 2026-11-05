"""Per-record GCS KV storage with single-writer redo recovery."""

import asyncio
import json

from ray.experimental.internal_kv import (
    _internal_kv_get,
    _internal_kv_put,
    _internal_kv_del,
    _internal_kv_list,
)


class GcsStore:
    MAX_OPS = 32
    NAMESPACE = "ray-submission-queue"
    PREFIX = b"records-v2/"

    def __init__(self, digest, *, namespace=NAMESPACE):
        self.namespace = namespace
        self.batches = 0
        self.bytes_written = 0
        self.kv_writes = 0
        legacy = self._get(b"controller-state-v1")
        if legacy is not None:
            state = json.loads(legacy)
            if state.get("version") != 1 or state.get("admin_token_digest") != digest:
                raise PermissionError(
                    "Legacy state requires its original administrator token and supported schema"
                )
        self._put(b"auth-v2", digest.encode(), overwrite=False)
        if self._get(b"auth-v2") != digest.encode():
            raise PermissionError(
                "Existing storage requires the original administrator token"
            )
        # Ray's fixed named actor is the sole writer. GCS KV has no CAS:
        # this redo journal is not a distributed lock or multi-writer transaction.
        pending = self._get(b"pending-v2")
        if pending is not None:
            self._apply(json.loads(pending))
            self._delete(b"pending-v2")
        self._migrate_legacy(digest)

    def _get(self, key):
        return _internal_kv_get(key, namespace=self.namespace)

    def _put(self, key, value, *, overwrite=True):
        _internal_kv_put(key, value, overwrite=overwrite, namespace=self.namespace)
        self.bytes_written += len(key) + len(value)
        self.kv_writes += 1

    def _delete(self, key):
        _internal_kv_del(key, namespace=self.namespace)
        self.kv_writes += 1

    def _apply(self, changes):
        for suffix, value in changes:
            key = self.PREFIX + suffix.encode()
            if value is None:
                self._delete(key)
            else:
                self._put(
                    key,
                    json.dumps(value, separators=(",", ":"), sort_keys=True).encode(),
                )

    def _migrate_legacy(self, digest):
        raw = self._get(b"controller-state-v1")
        if raw is None or self._get(b"migrated-v1"):
            return
        state = json.loads(raw)
        if state["admin_token_digest"] != digest:
            raise PermissionError(
                "Legacy state requires its original administrator token"
            )
        # Migration runs before serving requests. Replaying after an interrupted
        # copy is safe; the old named actor must have stopped before this starts.
        changes = []
        import base64

        for namespace, job, resources in state["quotas"]:
            key = [namespace, job]
            suffix = (
                "quota/" + base64.urlsafe_b64encode(json.dumps(key).encode()).decode()
            )
            changes.append(
                (suffix, {"key": key, "resources": resources, "revision": 1})
            )
        for permit in state.get("permits", []):
            changes.append(
                (
                    "permit/" + permit["ticket"],
                    {
                        "key": [permit["quota_namespace"], permit["job_id"]],
                        "resources": permit["resources"],
                        "refs": permit.get("refs", []),
                        "actor_id": permit.get("actor_id"),
                        "revision": 1,
                        "state": "TRACKED"
                        if permit.get("refs") or permit.get("actor_id")
                        else "RESERVED",
                    },
                )
            )
        for ticket in state.get("released", []):
            changes.append(("permit/" + ticket, {"state": "RELEASED", "revision": 1}))
        for offset in range(0, len(changes), self.MAX_OPS):
            self.commit(changes[offset : offset + self.MAX_OPS])
        self._put(b"migrated-v1", b"1")

    def load(self):
        records = {}
        for key in _internal_kv_list(self.PREFIX, namespace=self.namespace):
            raw = self._get(key)
            if raw is None:
                raise RuntimeError("Record disappeared during single-writer recovery")
            records[key[len(self.PREFIX) :].decode()] = json.loads(raw)
        return records

    def commit(self, changes):
        if len(changes) > self.MAX_OPS:
            raise ValueError("Batch exceeds bounded write size")
        if not changes:
            return
        # Persist intent first, then apply idempotent record replacements.
        # Only this controller reads these keys; it publishes memory after all
        # writes finish. Any error stops admission until replay on reconstruction.
        encoded = json.dumps(changes, separators=(",", ":"), sort_keys=True).encode()
        self._put(b"pending-v2", encoded)
        self._apply(changes)
        self._delete(b"pending-v2")
        self.batches += 1


class BoundedWriter:
    """One blocking I/O worker; bounded requests and redo batches."""

    def __init__(self, store):
        self.store = store
        self.requests = asyncio.Queue(maxsize=32)
        self.task = None
        self.error = None

    async def commit(self, changes):
        if self.error:
            raise RuntimeError("Storage writer failed") from self.error
        if self.task is None:
            self.task = asyncio.create_task(self._run())
        future = asyncio.get_running_loop().create_future()
        await self.requests.put((changes, future))
        return await asyncio.shield(future)

    async def _run(self):
        while True:
            changes, future = await self.requests.get()
            try:
                if self.error:
                    raise RuntimeError("Storage writer failed") from self.error
                await asyncio.to_thread(self.store.commit, changes)
            except Exception as error:
                self.error = error
                if not future.done():
                    future.set_exception(error)
            else:
                if not future.done():
                    future.set_result(None)
            finally:
                self.requests.task_done()
