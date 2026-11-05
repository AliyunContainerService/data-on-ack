"""Fault injection at persistence and RPC retry boundaries."""

import asyncio
import copy
import unittest
from unittest.mock import patch

from ray_admission import queue


class ControllerFaultTest(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.saved = None

        def save(state, **kwargs):
            self.saved = copy.deepcopy(state)

        self.save = save
        self.load_patch = patch.object(queue, "_load_state", lambda: copy.deepcopy(self.saved))
        self.save_patch = patch.object(queue, "_save_state", side_effect=save)
        self.load_patch.start()
        self.writer = self.save_patch.start()
        self.addCleanup(self.load_patch.stop)
        self.addCleanup(self.save_patch.stop)
        self.controller = queue._QueueController("digest")
        self.request = {
            "kind": "actor_creation", "name": "Service", "job_id": "job",
            "resources": {"CPU": 1},
        }

    async def test_duplicate_actor_tracking_keeps_permit(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        with patch.object(c, "_start_monitor"):
            await c.track_actor("ticket", "actor-id")
            await c.track_actor("ticket", "actor-id")
            with self.assertRaises(ValueError):
                await c.track_actor("ticket", "different-actor")
        self.assertEqual(c._usage[(queue._NAMESPACE, "job")]["CPU"], 1)
        self.assertIn("ticket", c._permits)

    async def test_released_ticket_cannot_be_granted_again_after_restart(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        c.release("ticket")
        restored = queue._QueueController("digest")
        with self.assertRaisesRegex(ValueError, "released"):
            await restored.acquire(self.request, "ticket")
        restored.release("ticket")
        self.assertFalse(restored._permits)

    async def test_ambiguous_grant_write_stops_admission_until_restore(self):
        def write_then_fail(state, **kwargs):
            self.save(state)
            raise OSError("response lost after commit")

        self.writer.side_effect = write_then_fail
        c = self.controller
        with self.assertRaisesRegex(RuntimeError, "restart required"):
            await c.acquire(self.request, "ticket")
        with self.assertRaises(RuntimeError):
            await c.acquire(self.request, "second")
        self.writer.side_effect = self.save
        restored = queue._QueueController("digest")
        self.assertTrue(await restored.acquire(self.request, "ticket"))
        self.assertEqual(restored._usage[(queue._NAMESPACE, "job")]["CPU"], 1)

    async def test_failed_release_retains_durable_reservation(self):
        c = self.controller
        await c.acquire(self.request, "ticket")
        self.writer.side_effect = OSError("write rejected")
        with self.assertRaises(RuntimeError):
            c.release("ticket")
        with self.assertRaises(RuntimeError):
            await c.acquire(self.request, "second")
        self.writer.side_effect = self.save
        restored = queue._QueueController("digest")
        self.assertIn("ticket", restored._permits)

    async def test_duplicate_waiters_share_one_reservation(self):
        c = self.controller
        c._quotas[(queue._NAMESPACE, "job")] = {"CPU": 0}
        first = asyncio.create_task(c.acquire(self.request, "ticket"))
        second = asyncio.create_task(c.acquire(self.request, "ticket"))
        await asyncio.sleep(0)
        self.assertEqual(len(c._waiting[(queue._NAMESPACE, "job")]), 1)
        c._quotas.clear()
        c._drain()
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
