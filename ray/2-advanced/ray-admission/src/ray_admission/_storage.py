"""Transactional per-record storage through the etcd v3 JSON gateway."""

import asyncio
import base64
import json
import ssl
import uuid
from urllib.request import Request, urlopen


def _b64(value):
    return base64.b64encode(value.encode()).decode()


class FencedError(RuntimeError):
    pass


class EtcdStore:
    MAX_OPS = 32

    def __init__(self, endpoint, prefix, digest, *, ca_file=None):
        if not endpoint or not endpoint.startswith(("http://", "https://")):
            raise ValueError("Set RAY_ADMISSION_ETCD_ENDPOINT to an etcd v3 endpoint")
        if not prefix or not prefix.startswith("/"):
            raise ValueError("etcd prefix must be an absolute, cluster-specific path")
        self.endpoint = endpoint.rstrip("/")
        self.prefix = prefix.rstrip("/") + "/"
        self.context = ssl.create_default_context(cafile=ca_file)
        self.generation = uuid.uuid4().hex
        self.transactions = 0
        self.bytes_written = 0
        auth = self._key("auth")
        self._call(
            "txn",
            {
                "compare": [{"key": auth, "target": "VERSION", "version": "0"}],
                "success": [{"request_put": {"key": auth, "value": _b64(digest)}}],
            },
        )
        response = self._call("range", {"key": auth})
        if base64.b64decode(response["kvs"][0]["value"]).decode() != digest:
            raise PermissionError(
                "Existing storage requires the original administrator token"
            )
        # A new incarnation supersedes the previous writer. Every subsequent
        # transaction compares this generation atomically with its mutations.
        self._call(
            "put", {"key": self._key("generation"), "value": _b64(self.generation)}
        )

    def _key(self, suffix):
        return _b64(self.prefix + suffix)

    def _call(self, method, data):
        body = json.dumps(data).encode()
        request = Request(
            self.endpoint + "/v3/kv/" + method,
            body,
            {"Content-Type": "application/json"},
        )
        with urlopen(request, timeout=10, context=self.context) as response:
            result = json.load(response)
        if "error" in result:
            raise RuntimeError(result["error"])
        return result

    def load(self):
        """Read a consistent, revision-pinned paginated snapshot."""
        start = self.prefix
        end = self.prefix[:-1] + "0"  # '/' + 1; all keys below this prefix.
        revision = None
        records = {}
        while True:
            request = {
                "key": _b64(start),
                "range_end": _b64(end),
                "limit": "256",
                "sort_order": "ASCEND",
                "sort_target": "KEY",
            }
            if revision is not None:
                request["revision"] = revision
            response = self._call("range", request)
            if revision is None:
                revision = response["header"]["revision"]
            for entry in response.get("kvs", []):
                key = base64.b64decode(entry["key"]).decode()
                suffix = key[len(self.prefix) :]
                if suffix not in ("auth", "generation"):
                    records[suffix] = json.loads(base64.b64decode(entry["value"]))
            if not response.get("more"):
                break
            start = base64.b64decode(response["kvs"][-1]["key"]).decode() + "\0"
        # Detect takeover during recovery, even if there are no initial writes.
        self.commit([])
        return records

    def commit(self, changes):
        if len(changes) > self.MAX_OPS:
            raise ValueError("Transaction exceeds bounded batch size")
        operations = []
        for key, value in changes:
            if value is None:
                operations.append({"request_delete_range": {"key": self._key(key)}})
            else:
                encoded = json.dumps(value, separators=(",", ":"), sort_keys=True)
                operations.append(
                    {"request_put": {"key": self._key(key), "value": _b64(encoded)}}
                )
        request = {
            "compare": [
                {
                    "key": self._key("generation"),
                    "target": "VALUE",
                    "value": _b64(self.generation),
                }
            ],
            "success": operations,
        }
        response = self._call("txn", request)
        if not response.get("succeeded"):
            raise FencedError("Controller superseded by a newer generation")
        self.transactions += 1
        self.bytes_written += len(json.dumps(request).encode())


class BoundedWriter:
    """One blocking I/O worker; bounded requests and atomic batches."""

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
