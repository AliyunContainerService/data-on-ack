"""Single-writer admission state machine backed by fenced etcd transactions."""

import asyncio
import base64
import hashlib
import hmac
import json
import os
from collections import Counter, deque

import ray

from ray_admission._storage import BoundedWriter, EtcdStore

NAMESPACE = "ray-submission-queue"


def quota_key(key):
    return "quota/" + base64.urlsafe_b64encode(json.dumps(key).encode()).decode()


def ticket_parts(ticket):
    if ":" not in ticket:
        return None
    session, sequence = ticket.split(":", 1)
    if not session or not sequence.isdigit() or int(sequence) < 1:
        raise ValueError("Invalid session ticket")
    return session, int(sequence)


class QueueController:
    def __init__(self, admin_token_digest, endpoint=None, prefix=None, ca_file=None):
        from ray_admission.queue import _validate_resources

        self._validate = _validate_resources
        self._store = EtcdStore(
            endpoint or os.environ.get("RAY_ADMISSION_ETCD_ENDPOINT"),
            prefix or os.environ.get("RAY_ADMISSION_ETCD_PREFIX"),
            admin_token_digest,
            ca_file=ca_file or os.environ.get("RAY_ADMISSION_ETCD_CA_FILE"),
        )
        self._writer = BoundedWriter(self._store)
        self._admin_token_digest = admin_token_digest
        self._lock = asyncio.Lock()
        self._records = self._store.load()
        self._quotas = {}
        self._sessions = {}
        self._counts = Counter()
        self._usage = {}
        self._permits = {}
        self._tracked = {}
        self._actors = {}
        self._released = set()
        self._waiting = {}
        self._pending = {}
        self._requests = {}
        self._storage_error = None
        self._monitor_error = None
        self._monitor = None
        self._drainer = None
        for name, record in self._records.items():
            if name.startswith("quota/"):
                self._validate(record["resources"])
                self._quotas[tuple(record["key"])] = record["resources"]
            elif name.startswith("session/"):
                self._sessions[name[8:]] = record["watermark"]
            elif name.startswith("permit/"):
                self._apply_permit(name[7:], record)
            else:
                raise RuntimeError("Unknown storage record: " + name)
        self._start_monitor()

    def _apply_permit(self, ticket, record):
        old = self._permits.pop(ticket, None)
        if old:
            key, resources = old
            self._usage[key].subtract(resources)
        self._tracked.pop(ticket, None)
        self._actors.pop(ticket, None)
        self._released.discard(ticket)
        if record is None:
            return
        if record["state"] == "RELEASED":
            self._released.add(ticket)
            return
        if record["state"] not in ("RESERVED", "TRACKED"):
            raise RuntimeError("Invalid permit state")
        key = tuple(record["key"])
        resources = self._validate(record["resources"])
        self._permits[ticket] = (key, resources)
        self._usage.setdefault(key, Counter()).update(resources)
        if record.get("refs"):
            self._tracked[ticket] = {
                ray.cloudpickle.loads(base64.b64decode(value))
                for value in record["refs"]
            }
        if record.get("actor_id"):
            self._actors[ticket] = record["actor_id"]

    def _ensure_writable(self):
        if self._storage_error is not None:
            raise RuntimeError("Queue storage unavailable; restart required")

    async def _commit(self, changes):
        """Called under the mutation lock; publish memory only after commit."""
        self._ensure_writable()
        changes = [
            (
                key,
                None
                if value is None
                else {
                    **value,
                    "revision": self._records.get(key, {}).get("revision", 0) + 1,
                },
            )
            for key, value in changes
        ]
        try:
            await self._writer.commit(changes)
        except BaseException as error:
            # Cancellation can race a committed write just like a lost reply.
            self._storage_error = str(error)
            for future in self._pending.values():
                if not future.done():
                    future.set_exception(
                        RuntimeError("Queue storage unavailable; restart required")
                    )
            raise RuntimeError("Queue storage unavailable; restart required") from error
        for key, record in changes:
            if record is None:
                self._records.pop(key, None)
            else:
                self._records[key] = record
            if key.startswith("permit/"):
                self._apply_permit(key[7:], record)

    def check_admin(self, token):
        if not isinstance(token, str) or not token:
            raise PermissionError("Queue administrator credentials required")
        if not hmac.compare_digest(
            hashlib.sha256(token.encode()).hexdigest(), self._admin_token_digest
        ):
            raise PermissionError("Invalid queue administrator credentials")

    async def open_session(self, session):
        if not session or any(c not in "0123456789abcdef" for c in session):
            raise ValueError("Invalid session ID")
        async with self._lock:
            self._ensure_writable()
            if session not in self._sessions:
                await self._commit([("session/" + session, {"watermark": 0})])
                self._sessions[session] = 0

    def _acknowledged(self, ticket):
        parts = ticket_parts(ticket)
        return parts is not None and parts[1] <= self._sessions.get(parts[0], 0)

    async def acknowledge(self, session, watermark):
        if (
            isinstance(watermark, bool)
            or not isinstance(watermark, int)
            or watermark < 0
        ):
            raise ValueError("Invalid acknowledgment watermark")
        async with self._lock:
            self._ensure_writable()
            if session not in self._sessions:
                raise ValueError("Unknown session")
            if watermark > self._sessions[session]:
                await self._commit([("session/" + session, {"watermark": watermark})])
                self._sessions[session] = watermark
            await self._compact_locked()

    async def _compact_locked(self):
        obsolete = [ticket for ticket in self._released if self._acknowledged(ticket)]
        for offset in range(0, len(obsolete), 32):
            await self._commit(
                [
                    ("permit/" + ticket, None)
                    for ticket in obsolete[offset : offset + 32]
                ]
            )

    async def set_job_quota(
        self, job_id, resources, *, quota_namespace=NAMESPACE, admin_token=None
    ):
        self.check_admin(admin_token)
        if (
            not isinstance(job_id, str)
            or not job_id
            or not isinstance(quota_namespace, str)
            or not quota_namespace
        ):
            raise ValueError("job_id and quota_namespace must be nonempty strings")
        if resources is not None:
            self._validate(resources)
        key = (quota_namespace, job_id)
        async with self._lock:
            await self._commit(
                [
                    (
                        quota_key(key),
                        None
                        if resources is None
                        else {"key": list(key), "resources": dict(resources)},
                    )
                ]
            )
            if resources is None:
                self._quotas.pop(key, None)
            else:
                self._quotas[key] = dict(resources)
        self._schedule_drain()

    async def acquire(self, request, ticket):
        resources = self._validate(request["resources"])
        key = (request.get("quota_namespace", NAMESPACE), request["job_id"])
        if not request.get("trackable", True) and resources:
            raise ValueError("Resource accounting requires positive fixed returns")
        async with self._lock:
            self._ensure_writable()
            parts = ticket_parts(ticket)
            if parts and parts[0] not in self._sessions:
                raise ValueError("Unknown session")
            if ticket in self._released or self._acknowledged(ticket):
                raise ValueError("Ticket has already been released or acknowledged")
            if ticket in self._permits:
                if self._permits[ticket] != (key, resources):
                    raise ValueError("Ticket reused for a different request")
                await self._commit([])  # Fence stale read-only approval retries too.
                return True
            if request["kind"] == "actor_method":
                self._counts[request["kind"]] += 1
                return True
            if ticket in self._pending:
                if self._requests[ticket] != (key, resources):
                    raise ValueError("Ticket reused for a different request")
                future = self._pending[ticket]
            else:
                future = asyncio.get_running_loop().create_future()
                self._pending[ticket] = future
                self._requests[ticket] = (key, resources)
                self._waiting.setdefault(key, deque()).append(
                    (ticket, request, resources, future)
                )
        self._schedule_drain()
        try:
            return await asyncio.shield(future)
        except asyncio.CancelledError:
            await self.release(ticket)
            raise
        finally:
            if future.done():
                if not future.cancelled():
                    future.exception()
                self._pending.pop(ticket, None)
                self._requests.pop(ticket, None)

    def _schedule_drain(self):
        if self._drainer is None or self._drainer.done():
            self._drainer = asyncio.create_task(self._drain())

    async def _drain(self):
        try:
            async with self._lock:
                self._ensure_writable()
                for key, waiting in list(self._waiting.items()):
                    while waiting:
                        selected = []
                        projected = self._usage.get(key, Counter()).copy()
                        for item in waiting:
                            ticket, request, resources, future = item
                            if future.done():
                                selected.append(item)
                            else:
                                if any(
                                    projected[r] + resources.get(r, 0) > limit + 1e-9
                                    for r, limit in self._quotas.get(key, {}).items()
                                ):
                                    break
                                projected.update(resources)
                                selected.append(item)
                            if len(selected) == 32:
                                break
                        if not selected:
                            break
                        changes = [
                            (
                                "permit/" + ticket,
                                {
                                    "state": "RESERVED",
                                    "key": list(key),
                                    "resources": resources,
                                },
                            )
                            for ticket, _, resources, future in selected
                            if resources and not future.done()
                        ]
                        if changes:
                            await self._commit(changes)
                        for ticket, request, _, future in selected:
                            waiting.popleft()
                            if not future.done():
                                self._counts[request["kind"]] += 1
                                future.set_result(True)
                    if not waiting:
                        del self._waiting[key]
        except Exception as error:
            # _commit already poisons the controller on ambiguous writes.
            for future in self._pending.values():
                if not future.done():
                    future.set_exception(error)

    async def _release_locked(self, tickets):
        changes = []
        for ticket in tickets:
            if ticket in self._released:
                continue
            if self._acknowledged(ticket):
                if ticket not in self._permits:
                    continue
                value = None
            else:
                value = {"state": "RELEASED"}
            changes.append(("permit/" + ticket, value))
        for offset in range(0, len(changes), 32):
            await self._commit(changes[offset : offset + 32])
        for ticket in tickets:
            future = self._pending.get(ticket)
            if future is not None and not future.done():
                future.cancel()

    async def release(self, ticket):
        async with self._lock:
            self._ensure_writable()
            await self._release_locked([ticket])
        self._schedule_drain()

    async def track(self, ticket, refs):
        refs = set(refs)
        async with self._lock:
            self._ensure_writable()
            if ticket not in self._permits:
                return
            if ticket in self._actors or (
                ticket in self._tracked and self._tracked[ticket] != refs
            ):
                raise ValueError("Ticket already tracks a different execution")
            if ticket not in self._tracked:
                await self._commit(
                    [
                        (
                            "permit/" + ticket,
                            {
                                **self._records["permit/" + ticket],
                                "state": "TRACKED",
                                "refs": [
                                    base64.b64encode(
                                        ray.cloudpickle.dumps(ref)
                                    ).decode()
                                    for ref in refs
                                ],
                            },
                        )
                    ]
                )
        self._start_monitor()

    async def track_actor(self, ticket, actor_id):
        async with self._lock:
            self._ensure_writable()
            if ticket not in self._permits:
                return
            if ticket in self._tracked or (
                ticket in self._actors and self._actors[ticket] != actor_id
            ):
                raise ValueError("Ticket already tracks a different execution")
            if ticket in self._actors:
                return
            if actor_id in self._actors.values():
                await self._release_locked([ticket])
            else:
                await self._commit(
                    [
                        (
                            "permit/" + ticket,
                            {
                                **self._records["permit/" + ticket],
                                "state": "TRACKED",
                                "actor_id": actor_id,
                            },
                        )
                    ]
                )
        self._start_monitor()
        self._schedule_drain()

    def _start_monitor(self):
        if self._monitor is None or self._monitor.done():
            self._monitor = asyncio.create_task(self._watch_completions())

    async def _watch_completions(self):
        delay = 0.1
        while self._tracked or self._actors or self._released:
            if self._storage_error is not None:
                return
            try:
                await self._poll_completions()
                self._monitor_error = None
                delay = 0.1
            except asyncio.CancelledError:
                raise
            except Exception as error:
                self._monitor_error = str(error)
                delay = min(delay * 2, 5)
            await asyncio.sleep(delay)

    async def _poll_completions(self):
        from ray._private.state import actors

        tracked = dict(self._tracked)
        actor_ids = dict(self._actors)
        refs = list({ref for pending in tracked.values() for ref in pending})
        ready = set()
        if refs:
            ready, _ = await asyncio.to_thread(
                ray.wait, refs, num_returns=len(refs), timeout=0, fetch_local=False
            )
            ready = set(ready)
        states = await asyncio.to_thread(actors) if actor_ids else {}
        completed = [
            ticket for ticket, pending in tracked.items() if pending.issubset(ready)
        ]
        completed += [
            ticket
            for ticket, actor_id in actor_ids.items()
            if states.get(actor_id, {}).get("State") == "DEAD"
        ]
        async with self._lock:
            self._ensure_writable()
            await self._release_locked(completed)
            await self._compact_locked()
        if completed:
            self._schedule_drain()

    def snapshot(self):
        return dict(self._counts)

    def status(self):
        def nested(values):
            result = {}
            for (namespace, job_id), value in values.items():
                result.setdefault(namespace, {})[job_id] = dict(value)
            return result

        return {
            "health": {
                "storage_error": self._storage_error,
                "monitor_error": self._monitor_error,
                "accepting": self._storage_error is None,
            },
            "storage": {
                "backend": "etcd",
                "transactions": self._store.transactions,
                "bytes_written": self._store.bytes_written,
                "released_records": len(self._released),
                "sessions": len(self._sessions),
            },
            "quotas": nested(self._quotas),
            "usage": nested(self._usage),
            "permits": [
                {
                    "ticket": ticket,
                    "state": self._records["permit/" + ticket]["state"],
                    "quota_namespace": key[0],
                    "job_id": key[1],
                    "resources": dict(resources),
                }
                for ticket, (key, resources) in self._permits.items()
            ],
            "waiting": [
                {
                    "ticket": ticket,
                    "name": request["name"],
                    "job_id": key[1],
                    "quota_namespace": key[0],
                    "resources": dict(resources),
                }
                for key, waiting in self._waiting.items()
                for ticket, request, resources, future in waiting
                if not future.done()
            ],
        }
