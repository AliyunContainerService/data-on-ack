import asyncio
import threading
import unittest
from unittest.mock import patch

from etcd_test_support import configure_storage
from ray_admission import queue
from ray_admission._storage import FencedError


class ControllerFaultTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        configure_storage()
        self.controllers = []
        self.controller = self.restore()
        self.request = {
            "kind": "actor_creation",
            "name": "Service",
            "job_id": "job",
            "resources": {"CPU": 1},
        }

    def restore(self):
        c = queue._QueueController("digest")
        self.controllers.append(c)
        return c

    async def asyncTearDown(self):
        tasks = []
        for c in self.controllers:
            for task in (c._monitor, c._drainer, c._writer.task):
                if task:
                    task.cancel()
                    tasks.append(task)
        await asyncio.gather(*tasks, return_exceptions=True)

    async def test_duplicate_actor_tracking_keeps_permit(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        with patch.object(c, "_start_monitor"):
            await c.track_actor("ticket", "actor-id")
            await c.track_actor("ticket", "actor-id")
            with self.assertRaises(ValueError):
                await c.track_actor("ticket", "different")
        self.assertEqual(c._usage[(queue._NAMESPACE, "job")]["CPU"], 1)

    async def test_released_ticket_cannot_be_granted_again_after_restart(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        await c.release("ticket")
        restored = self.restore()
        with self.assertRaises(ValueError):
            await restored.acquire(self.request, "ticket")
        await restored.release("ticket")
        self.assertFalse(restored._permits)

    async def test_ambiguous_grant_write_stops_admission_until_restore(self):
        c = self.controller
        original = c._store.commit

        def write_then_fail(changes):
            original(changes)
            raise OSError("response lost after commit")

        with patch.object(c._store, "commit", side_effect=write_then_fail):
            with self.assertRaisesRegex(RuntimeError, "restart required"):
                await c.acquire(self.request, "ticket")
            with self.assertRaises(RuntimeError):
                await c.acquire(self.request, "second")
        restored = self.restore()
        self.assertTrue(await restored.acquire(self.request, "ticket"))
        self.assertEqual(restored._usage[(queue._NAMESPACE, "job")]["CPU"], 1)

    async def test_failed_release_retains_durable_reservation(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        with patch.object(c._store, "commit", side_effect=OSError("write rejected")):
            with self.assertRaises(RuntimeError):
                await c.release("ticket")
        self.assertIn("ticket", self.restore()._permits)

    async def test_duplicate_waiters_share_one_reservation(self):
        c = self.controller
        c._quotas[(queue._NAMESPACE, "job")] = {"CPU": 0}
        first = asyncio.create_task(c.acquire(self.request, "ticket"))
        second = asyncio.create_task(c.acquire(self.request, "ticket"))
        await asyncio.sleep(0.02)
        self.assertEqual(len(c._waiting[(queue._NAMESPACE, "job")]), 1)
        c._quotas.clear()
        c._schedule_drain()
        self.assertEqual(await asyncio.gather(first, second), [True, True])
        self.assertEqual(c._usage[(queue._NAMESPACE, "job")]["CPU"], 1)

    async def test_monitor_retries_transient_query_failure(self):
        c = self.controller
        c._actors["ticket"] = "actor-id"
        calls = 0

        async def poll():
            nonlocal calls
            calls += 1
            if calls == 1:
                raise OSError("temporary GCS failure")
            c._actors.clear()

        with patch.object(c, "_poll_completions", side_effect=poll):
            await asyncio.wait_for(c._watch_completions(), timeout=2)
        self.assertEqual(calls, 2)
        self.assertIsNone(c._monitor_error)

    async def test_old_controller_is_fenced(self):
        old = self.controller
        await old.acquire(self.request, "cached")
        new = self.restore()
        with self.assertRaises(FencedError):
            old._store.commit([("permit/stale", {"state": "RELEASED"})])
        self.assertNotIn("permit/stale", new._store.load())
        with self.assertRaises(RuntimeError):
            await old.acquire(self.request, "cached")

    async def test_cancelled_write_requires_recovery_and_does_not_block_loop(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        entered = threading.Event()
        resume = threading.Event()
        original = c._store.commit

        def slow_write(changes):
            entered.set()
            if not resume.wait(timeout=5):
                raise TimeoutError("test did not unblock writer")
            original(changes)

        with patch.object(c._store, "commit", side_effect=slow_write):
            task = asyncio.create_task(c.track_actor("ticket", "actor-id"))
            while not entered.is_set():
                await asyncio.sleep(0.001)
            # The event loop is responsive while storage I/O is blocked.
            self.assertFalse(task.done())
            task.cancel()
            try:
                with self.assertRaises(RuntimeError):
                    await task
            finally:
                resume.set()
            await c._writer.requests.join()
        self.assertIsNotNone(c._storage_error)
        with patch.object(queue._QueueController, "_start_monitor"):
            restored = self.restore()
        self.assertEqual(restored._actors["ticket"], "actor-id")

    async def test_compaction_resumes_after_watermark_commit(self):
        c = self.controller
        await c.open_session("abc")
        await c.acquire(self.request, "abc:1")
        await c.release("abc:1")
        original = c._store.commit

        def fail_deletion(changes):
            if any(value is None for _, value in changes):
                raise OSError("cleanup unavailable")
            original(changes)

        with patch.object(c._store, "commit", side_effect=fail_deletion):
            with self.assertRaises(RuntimeError):
                await c.acknowledge("abc", 1)
        restored = self.restore()
        with self.assertRaises(ValueError):
            await restored.acquire(self.request, "abc:1")
        async with restored._lock:
            await restored._compact_locked()
        self.assertNotIn("permit/abc:1", restored._store.load())

    async def test_ack_compacts_tombstones_without_resurrecting_tickets(self):
        c = self.controller
        await c.open_session("abc")
        await c.acquire(self.request, "abc:1")
        await c.acquire(self.request, "abc:2")
        await c.release("abc:2")
        # An out-of-order completion cannot acknowledge the missing first one.
        await c.acknowledge("abc", 0)
        self.assertIn("abc:2", c._released)
        await c.acknowledge("abc", 2)
        self.assertNotIn("abc:2", c._released)
        self.assertIn("abc:1", c._permits)
        await c.release("abc:1")
        restored = self.restore()
        self.assertFalse(restored._permits)
        self.assertFalse(restored._released)
        with self.assertRaises(ValueError):
            await restored.acquire(self.request, "abc:2")

    async def test_batched_grants_and_constant_transition_write_size(self):
        c = self.controller
        c._quotas[(queue._NAMESPACE, "job")] = {"CPU": 0}
        waiters = [
            asyncio.create_task(c.acquire(self.request, f"batch-{i:04d}"))
            for i in range(96)
        ]
        await asyncio.sleep(0.02)
        before = c._store.transactions
        c._quotas.clear()
        c._schedule_drain()
        await asyncio.gather(*waiters)
        self.assertEqual(c._store.transactions - before, 3)
        before = c._store.bytes_written
        await c.acquire(self.request, "one-record")
        size = c._store.bytes_written - before
        self.assertLess(size, 1500)
        # Active-set size does not appear in the transaction payload.
        self.assertEqual(len(c._permits), 97)
