"""Integration checks for the experimental admission prototype."""

import asyncio
import json
import secrets
import subprocess
import sys
import time
import unittest
import uuid
from concurrent.futures import ThreadPoolExecutor
from unittest.mock import patch

import ray
from ray_admission import queue


@ray.remote(num_cpus=0)
class _Gate:
    def __init__(self):
        self.events = {}
        self.entered = []
        self.active = 0
        self.peak = 0

    async def block(self, token):
        event = self.events.setdefault(token, asyncio.Event())
        self.entered.append(token)
        self.active += 1
        self.peak = max(self.peak, self.active)
        await event.wait()
        self.active -= 1
        return token

    def open(self, token):
        self.events.setdefault(token, asyncio.Event()).set()

    def state(self):
        return self.entered, self.active, self.peak


def wait_until(predicate):
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.02)
    raise AssertionError("Timed out waiting for queue state")


def controller_is_absent(namespace):
    try:
        ray.get_actor(queue._NAME, namespace=namespace)
    except ValueError:
        return True
    return False


def status(controller):
    state = ray.get(controller.status.remote())
    state["quotas"] = state["quotas"].get(queue._NAMESPACE, {})
    state["usage"] = state["usage"].get(queue._NAMESPACE, {})
    return state


class AutoStartTest(unittest.TestCase):
    def test_enable_admission_initializes_ray(self):
        self.assertFalse(ray.is_initialized())
        initialize = ray.init
        controller = None
        try:
            # Limit local test resources without bypassing real initialization.
            with patch.object(
                ray,
                "init",
                side_effect=lambda **kwargs: initialize(
                    num_cpus=2, include_dashboard=False, **kwargs
                ),
            ) as start:
                controller = queue.start(admin_token=secrets.token_urlsafe(32))
                queue.enable_admission()
                start.assert_called_once()
            self.assertTrue(ray.is_initialized())
            self.assertEqual(ray.get(controller.snapshot.remote()), {})
        finally:
            queue.disable_admission()
            if controller is not None:
                ray.kill(controller)
                wait_until(lambda: controller_is_absent(queue._NAMESPACE))
            ray.shutdown()


class SubmissionQueueTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        ray.init(
            num_cpus=2, num_gpus=2, resources={"custom": 2}, include_dashboard=False
        )

    @classmethod
    def tearDownClass(cls):
        ray.shutdown()

    def setUp(self):
        from ray.experimental.internal_kv import _internal_kv_del

        _internal_kv_del(queue._STATE_KEY, namespace=queue._NAMESPACE)
        self.controller = None
        self.admin_token = secrets.token_urlsafe(32)

    def tearDown(self):
        from ray.experimental.internal_kv import _internal_kv_del

        queue.disable_admission()
        if self.controller is not None:
            ray.kill(self.controller)
            wait_until(lambda: controller_is_absent(queue._NAMESPACE))
        _internal_kv_del(queue._STATE_KEY, namespace=queue._NAMESPACE)

    def test_quota_survives_controller_recreation(self):
        controller = queue.start(admin_token=self.admin_token)
        self.controller = controller
        ray.get(
            controller.set_job_quota.remote(
                "saved-job", {"CPU": 2}, admin_token=self.admin_token
            )
        )
        ray.kill(controller)
        wait_until(lambda: controller_is_absent(queue._NAMESPACE))
        self.controller = queue.start(admin_token=self.admin_token)
        self.assertEqual(status(self.controller)["quotas"], {"saved-job": {"CPU": 2}})

    def test_running_task_permit_survives_automatic_restart(self):
        gate = _Gate.remote()

        @ray.remote(num_cpus=1)
        def work():
            return ray.get(gate.block.remote("task"))

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 1}, admin_token=self.admin_token
            )
        )
        task = work.remote()
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 1})
        ray.kill(controller, no_restart=False)
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 1})
        gate.open.remote("task")
        self.assertEqual(ray.get(task), "task")
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 0})

    def test_running_actor_permit_survives_automatic_restart(self):
        @ray.remote(num_cpus=0.5)
        class Service:
            def ready(self):
                return True

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 0.5}, admin_token=self.admin_token
            )
        )
        actor = Service.remote()
        self.assertTrue(ray.get(actor.ready.remote()))
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 0.5})
        ray.kill(controller, no_restart=False)
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 0.5})
        ray.kill(actor)
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 0})

    def test_waiting_submission_retries_after_automatic_restart(self):
        gate = _Gate.remote()

        @ray.remote(num_cpus=1)
        def work(token):
            return ray.get(gate.block.remote(token))

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 1}, admin_token=self.admin_token
            )
        )
        first = work.remote("first")
        wait_until(lambda: status(controller)["usage"].get(job_id) == {"CPU": 1})
        with ThreadPoolExecutor(max_workers=1) as pool:
            second = pool.submit(work.remote, "second")
            wait_until(lambda: len(status(controller)["waiting"]) == 1)
            ray.kill(controller, no_restart=False)
            wait_until(lambda: len(status(controller)["waiting"]) == 1)
            self.assertFalse(second.done())
            ray.get(gate.open.remote("first"))
            self.assertEqual(ray.get(first), "first")
            second_ref = second.result(timeout=30)
            ray.get(gate.open.remote("second"))
            self.assertEqual(ray.get(second_ref), "second")

    def enable_admission(self):
        queue.start(admin_token=self.admin_token)
        self.controller = queue.enable_admission()
        return self.controller

    def test_native_results_options_annotations_and_existing_definitions(self):
        @ray.remote
        @queue.policy(queue="batch")
        def work(value):
            return value + 1

        @ray.remote
        @queue.policy(queue="service")
        class Service:
            def run(self, value):
                return value * 2

        controller = self.enable_admission()
        self.assertEqual(
            queue._request("task", work, (), {})["metadata"], {"queue": "batch"}
        )
        self.assertEqual(
            queue._request("actor_creation", Service, (), {})["metadata"],
            {"queue": "service"},
        )
        ref = work.options(num_cpus=0).remote(3)
        self.assertIsInstance(ref, ray.ObjectRef)
        self.assertEqual(ray.get(ref), 4)
        actor = Service.options(num_cpus=0).remote()
        try:
            self.assertIsInstance(actor, ray.actor.ActorHandle)
            self.assertEqual(ray.get(actor.run.options(num_returns=1).remote(5)), 10)
            self.assertEqual(
                ray.get(controller.snapshot.remote()),
                {"task": 1, "actor_creation": 1, "actor_method": 1},
            )
            self.assertIs(queue.enable_admission(), controller)
            queue.disable_admission()
            self.assertEqual(ray.get(work.remote(0)), 1)
            self.assertEqual(ray.get(controller.snapshot.remote())["task"], 1)
        finally:
            ray.kill(actor)

    def test_only_administrator_can_change_or_remove_quotas(self):
        controller = self.enable_admission()
        ray.get(
            controller.set_job_quota.remote(
                "protected-job", {"CPU": 1}, admin_token=self.admin_token
            )
        )
        for target in ("protected-job", ray.get_runtime_context().get_job_id()):
            for resources in ({"CPU": 100}, None):
                for credentials in (None, "wrong-token"):
                    with self.assertRaises(PermissionError):
                        ray.get(
                            controller.set_job_quota.remote(
                                target, resources, admin_token=credentials
                            )
                        )
        self.assertEqual(
            status(controller)["quotas"], {"protected-job": {"CPU": 1}}
        )
        ray.get(
            controller.set_job_quota.remote(
                "protected-job", None, admin_token=self.admin_token
            )
        )
        self.assertEqual(status(controller)["quotas"], {})

    def test_start_authenticates_existing_queue(self):
        controller = self.enable_admission()
        same = queue.start(admin_token=self.admin_token)
        self.assertEqual(controller._ray_actor_id, same._ray_actor_id)
        with self.assertRaises(PermissionError):
            queue.start(admin_token=secrets.token_urlsafe(32))
        # A failed bootstrap attempt cannot replace the original credentials.
        ray.get(
            controller.set_job_quota.remote(
                "job", {"CPU": 1}, admin_token=self.admin_token
            )
        )

    def test_business_registration_cannot_create_queue(self):
        with self.assertRaisesRegex(RuntimeError, "administrator"):
            queue.enable_admission()
        with self.assertRaises(ValueError):
            ray.get_actor(queue._NAME, namespace=queue._NAMESPACE)

    def test_enable_admission_uses_environment_namespace(self):
        namespace = "admission-env-" + uuid.uuid4().hex
        with patch.dict("os.environ", {"RAY_NAMESPACE": namespace}):
            self.controller = queue.start(admin_token=self.admin_token)
            attached = queue.enable_admission()
            self.assertEqual(attached._ray_actor_id, self.controller._ray_actor_id)
            self.assertIs(queue.enable_admission(), attached)

    def test_controller_identity_cannot_be_overridden(self):
        with self.assertRaises(TypeError):
            queue.start(admin_token=self.admin_token, name="other")
        with self.assertRaises(TypeError):
            queue.start(admin_token=self.admin_token, namespace="other")
        with self.assertRaises(TypeError):
            queue.enable_admission(name="other")
        with self.assertRaises(TypeError):
            queue.enable_admission(namespace="other")

    def test_quota_namespace_is_independent_of_controller_namespace(self):
        @ray.remote
        def work():
            return None

        controller = queue.start(admin_token=self.admin_token)
        self.controller = controller
        with patch.dict("os.environ", {"RAY_NAMESPACE": "team-a"}):
            queue.enable_admission()
            self.assertEqual(queue._quota_namespace, "team-a")
            self.assertEqual(
                queue._request("task", work, (), {})["quota_namespace"], "team-a"
            )
        queue.disable_admission()
        ray.get(
            controller.set_job_quota.remote(
                "same-job",
                {"CPU": 0},
                quota_namespace="team-a",
                admin_token=self.admin_token,
            )
        )
        request = {
            "kind": "task",
            "name": "example",
            "job_id": "same-job",
            "resources": {"CPU": 1},
        }
        blocked = controller.acquire.remote(
            {**request, "quota_namespace": "team-a"}, "blocked"
        )
        self.assertTrue(
            ray.get(
                controller.acquire.remote(
                    {**request, "quota_namespace": "team-b"}, "allowed"
                )
            )
        )
        state = ray.get(controller.status.remote())
        self.assertEqual(state["quotas"]["team-a"]["same-job"], {"CPU": 0})
        self.assertEqual(state["usage"]["team-b"]["same-job"], {"CPU": 1})
        self.assertEqual(state["waiting"][0]["quota_namespace"], "team-a")
        ray.get(controller.release.remote("allowed"))
        ray.get(
            controller.set_job_quota.remote(
                "same-job",
                None,
                quota_namespace="team-a",
                admin_token=self.admin_token,
            )
        )
        self.assertTrue(ray.get(blocked))
        ray.get(controller.release.remote("blocked"))

    def test_environment_namespace_reaches_actor_workers(self):
        namespace = "admission-worker-" + uuid.uuid4().hex

        @ray.remote(num_cpus=0)
        class Parent:
            def submit_child(self):
                @ray.remote(num_cpus=0)
                def child():
                    from ray_admission import queue as submission_queue

                    return submission_queue._quota_namespace

                from ray_admission import queue as submission_queue

                return submission_queue._quota_namespace, ray.get(child.remote())

        with patch.dict("os.environ", {"RAY_NAMESPACE": namespace}):
            self.controller = queue.start(admin_token=self.admin_token)
            queue.enable_admission()
            actor = Parent.remote()
            try:
                self.assertEqual(
                    ray.get(actor.submit_child.remote()), (namespace, namespace)
                )
                self.assertEqual(
                    ray.get(self.controller.snapshot.remote()),
                    {"actor_creation": 1, "actor_method": 1, "task": 1},
                )
            finally:
                ray.kill(actor)

    def test_controller_failure_prevents_submission(self):
        @ray.remote
        def work():
            return "must not be submitted"

        controller = self.enable_admission()
        ray.kill(controller)
        with self.assertRaises(ray.exceptions.RayActorError):
            work.remote()

    def test_detached_controller_reused_after_disable_admission(self):
        controller = self.enable_admission()
        queue.disable_admission()
        replacement = queue.enable_admission()
        self.assertEqual(controller._ray_actor_id, replacement._ray_actor_id)

    def test_nested_submissions_are_automatically_intercepted(self):
        @ray.remote(num_cpus=0)
        def parent():
            @ray.remote(num_cpus=0)
            def child():
                return 42

            return ray.get(child.remote())

        controller = self.enable_admission()
        self.assertEqual(ray.get(parent.remote()), 42)
        self.assertEqual(ray.get(controller.snapshot.remote()), {"task": 2})

    def test_actor_nested_submission_and_user_setup_hook(self):
        def setup():
            import os

            os.environ["SUBMISSION_QUEUE_TEST_SETUP"] = "preserved"

        @ray.remote(num_cpus=0)
        class Parent:
            def run(self):
                import os

                @ray.remote(num_cpus=0)
                def child():
                    return os.environ["SUBMISSION_QUEUE_TEST_SETUP"]

                return ray.get(child.remote())

        controller = self.enable_admission()
        actor = Parent.options(
            runtime_env={"worker_process_setup_hook": setup}
        ).remote()
        try:
            self.assertEqual(ray.get(actor.run.remote()), "preserved")
            self.assertEqual(
                ray.get(controller.snapshot.remote()),
                {"actor_creation": 1, "actor_method": 1, "task": 1},
            )
        finally:
            ray.kill(actor)

    def test_multiple_returns_and_remote_errors(self):
        @ray.remote(num_returns=2)
        def pair():
            return 1, 2

        @ray.remote
        def fail():
            raise ValueError("business failure")

        controller = self.enable_admission()
        refs = pair.remote()
        self.assertEqual(ray.get(refs), [1, 2])
        with self.assertRaises(ray.exceptions.RayTaskError):
            ray.get(fail.remote())
        self.assertEqual(ray.get(controller.snapshot.remote()), {"task": 2})

    def check_fifo(self, quota):
        @ray.remote(num_cpus=0.5)
        def work(gate, token):
            return ray.get(gate.block.remote(token))

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 0.5 * quota}, admin_token=self.admin_token
            )
        )
        gate = _Gate.remote()
        pool = ThreadPoolExecutor(max_workers=2)
        refs = []
        try:
            refs = [work.remote(gate, index) for index in range(quota)]
            wait_until(lambda: ray.get(gate.state.remote())[1] == quota)
            first = pool.submit(work.remote, gate, quota)
            wait_until(lambda: len(status(controller)["waiting"]) == 1)
            second = pool.submit(work.remote, gate, quota + 1)
            wait_until(lambda: len(status(controller)["waiting"]) == 2)
            self.assertFalse(first.done())
            self.assertFalse(second.done())
            self.assertEqual(ray.get(gate.state.remote())[2], quota)
            ray.get(gate.open.remote(0))
            refs.append(first.result(timeout=30))
            wait_until(lambda: quota in ray.get(gate.state.remote())[0])
            self.assertFalse(second.done())
            ray.get(gate.open.remote(quota))
            refs.append(second.result(timeout=30))
            wait_until(lambda: quota + 1 in ray.get(gate.state.remote())[0])
            for index in range(quota + 2):
                ray.get(gate.open.remote(index))
            self.assertEqual(ray.get(refs), list(range(quota + 2)))
            entered, _, peak = ray.get(gate.state.remote())
            self.assertEqual(sorted(entered[:quota]), list(range(quota)))
            self.assertEqual(entered[quota:], [quota, quota + 1])
            self.assertEqual(peak, quota)
            wait_until(
                lambda: not any(
                    status(controller)["usage"][job_id].values()
                )
            )
        finally:
            for index in range(quota + 2):
                ray.get(gate.open.remote(index))
            pool.shutdown(wait=False, cancel_futures=True)
            ray.kill(gate)

    def test_fifo_quota_one(self):
        self.check_fifo(1)

    def test_fifo_quota_two(self):
        self.check_fifo(2)

    def test_quota_released_after_task_and_submission_errors(self):
        @ray.remote(num_cpus=0.5, num_returns=2)
        def work(value):
            if value < 0:
                raise ValueError("expected failure")
            return value, value + 1

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 0.5}, admin_token=self.admin_token
            )
        )
        with self.assertRaises(ray.exceptions.RayTaskError):
            ray.get(work.remote(-1))
        self.assertEqual(ray.get(work.remote(3)), [3, 4])

        class Unserializable:
            def __reduce__(self):
                raise ValueError("cannot serialize")

        with self.assertRaises((TypeError, ValueError)):
            work.remote(Unserializable())
        self.assertEqual(ray.get(work.remote(5)), [5, 6])
        wait_until(
            lambda: not any(
                status(controller)["usage"][job_id].values()
            )
        )

    def test_invalid_quota(self):
        controller = self.enable_admission()
        for resources in (
            0,
            {"CPU": -1},
            {"CPU": True},
            {"CPU": float("nan")},
            {"GPU": float("inf")},
            {"": 1},
        ):
            with self.subTest(resources=resources), self.assertRaises(ValueError):
                ray.get(
                    controller.set_job_quota.remote(
                        "job-a", resources, admin_token=self.admin_token
                    )
                )
        with self.assertRaisesRegex(ValueError, "controller"):
            queue.policy(quota=2)

    def test_per_job_resource_quotas_and_fifo(self):
        controller = self.enable_admission()
        ray.get(
            controller.set_job_quota.remote(
                "job-a", {"CPU": 1, "GPU": 0.5}, admin_token=self.admin_token
            )
        )
        ray.get(
            controller.set_job_quota.remote(
                "job-b", {"CPU": 2}, admin_token=self.admin_token
            )
        )

        def acquire(job, name, resources, ticket):
            return controller.acquire.remote(
                {
                    "kind": "task",
                    "name": name,
                    "job_id": job,
                    "resources": resources,
                    "metadata": {},
                },
                ticket,
            )

        self.assertTrue(
            ray.get(acquire("job-a", "op1", {"CPU": 0.5, "GPU": 0.5}, "a0"))
        )
        a1 = acquire("job-a", "op2", {"CPU": 0.5, "GPU": 0.5}, "a1")
        wait_until(lambda: len(status(controller)["waiting"]) == 1)
        # A CPU-only request would fit but must not overtake the same job's head.
        a2 = acquire("job-a", "op3", {"CPU": 0.5}, "a2")
        wait_until(lambda: len(status(controller)["waiting"]) == 2)
        self.assertTrue(ray.get(acquire("job-b", "op1", {"CPU": 2}, "b0")))
        self.assertEqual(ray.wait([a1, a2], timeout=0)[0], [])
        state = status(controller)
        self.assertEqual(state["usage"]["job-a"], {"CPU": 0.5, "GPU": 0.5})
        self.assertEqual(state["usage"]["job-b"], {"CPU": 2})
        self.assertEqual([item["ticket"] for item in state["waiting"]], ["a1", "a2"])
        ray.get(controller.release.remote("a0"))
        self.assertEqual(ray.get([a1, a2]), [True, True])
        self.assertEqual(
            status(controller)["usage"]["job-a"],
            {"CPU": 1, "GPU": 0.5},
        )
        for ticket in ("a1", "a2", "b0"):
            ray.get(controller.release.remote(ticket))

    def test_live_quota_updates_preserve_usage(self):
        controller = self.enable_admission()
        request = {
            "kind": "task",
            "name": "op",
            "job_id": "job",
            "resources": {"CPU": 1},
            "metadata": {},
        }
        self.assertTrue(ray.get(controller.acquire.remote(request, "first")))
        ray.get(
            controller.set_job_quota.remote(
                "job", {"CPU": 0}, admin_token=self.admin_token
            )
        )
        pending = controller.acquire.remote(request, "second")
        wait_until(lambda: len(status(controller)["waiting"]) == 1)
        self.assertEqual(
            status(controller)["usage"]["job"], {"CPU": 1}
        )
        ray.get(
            controller.set_job_quota.remote(
                "job", {"CPU": 2}, admin_token=self.admin_token
            )
        )
        self.assertTrue(ray.get(pending))
        ray.get(
            controller.set_job_quota.remote(
                "job", {"CPU": 0}, admin_token=self.admin_token
            )
        )
        third = controller.acquire.remote(request, "third")
        wait_until(lambda: len(status(controller)["waiting"]) == 1)
        ray.get(
            controller.set_job_quota.remote("job", None, admin_token=self.admin_token)
        )
        self.assertTrue(ray.get(third))
        for ticket in ("first", "second", "third"):
            ray.get(controller.release.remote(ticket))

    def test_quota_rejects_untrackable_return_modes(self):
        @ray.remote(num_returns=0)
        def no_returns():
            pass

        @ray.remote
        def stream():
            yield 1

        controller = self.enable_admission()
        for task in (no_returns, stream):
            with self.assertRaisesRegex(ValueError, "positive fixed returns"):
                task.remote()
        self.assertEqual(ray.get(controller.snapshot.remote()), {})

    def test_options_resource_vector_is_charged(self):
        @ray.remote(num_cpus=1)
        def work(gate, token):
            return ray.get(gate.block.remote(token))

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        demand = {"CPU": 0.5, "GPU": 0.5, "custom": 0.25}
        ray.get(
            controller.set_job_quota.remote(
                job_id, demand, admin_token=self.admin_token
            )
        )
        configured = work.options(
            num_cpus=0.5, num_gpus=0.5, resources={"custom": 0.25}
        )
        gate = _Gate.remote()
        pool = ThreadPoolExecutor(max_workers=1)
        try:
            first = configured.remote(gate, 0)
            wait_until(lambda: ray.get(gate.state.remote())[1] == 1)
            second = pool.submit(configured.remote, gate, 1)
            wait_until(lambda: len(status(controller)["waiting"]) == 1)
            state = status(controller)
            self.assertEqual(state["usage"][job_id], demand)
            self.assertEqual(state["waiting"][0]["resources"], demand)
            self.assertEqual(state["waiting"][0]["job_id"], job_id)
            ray.get(gate.open.remote(0))
            second_ref = second.result(timeout=30)
            ray.get(gate.open.remote(1))
            self.assertEqual(ray.get([first, second_ref]), [0, 1])
        finally:
            ray.get([gate.open.remote(0), gate.open.remote(1)])
            pool.shutdown(wait=False, cancel_futures=True)
            ray.kill(gate)

    def test_actor_lifetime_resources_and_named_reuse(self):
        @ray.remote(num_cpus=1)
        class Service:
            def ready(self):
                return True

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 0.5}, admin_token=self.admin_token
            )
        )
        name = "service-" + uuid.uuid4().hex
        first = Service.options(num_cpus=0.5, name=name).remote()
        second = None
        pool = ThreadPoolExecutor(max_workers=1)
        try:
            self.assertTrue(ray.get(first.ready.remote()))
            existing = Service.options(
                num_cpus=0.5, name=name, get_if_exists=True
            ).remote()
            self.assertEqual(first._ray_actor_id, existing._ray_actor_id)
            self.assertEqual(
                status(controller)["usage"][job_id], {"CPU": 0.5}
            )
            pending = pool.submit(Service.options(num_cpus=0.5).remote)
            wait_until(lambda: len(status(controller)["waiting"]) == 1)
            ray.kill(first)
            second = pending.result(timeout=30)
            self.assertTrue(ray.get(second.ready.remote()))
            ray.kill(second)
            wait_until(
                lambda: status(controller)["usage"][job_id]["CPU"] == 0
            )
        finally:
            ray.kill(first)
            if second is not None:
                ray.kill(second)
            pool.shutdown(wait=False, cancel_futures=True)

    def test_real_jobs_have_independent_budgets(self):
        @ray.remote(num_cpus=0.5)
        def work():
            return ray.get_runtime_context().get_job_id()

        controller = self.enable_admission()
        job_id = ray.get_runtime_context().get_job_id()
        ray.get(
            controller.set_job_quota.remote(
                job_id, {"CPU": 0}, admin_token=self.admin_token
            )
        )
        pool = ThreadPoolExecutor(max_workers=1)
        pending = pool.submit(work.remote)
        code = """
import json
import sys
import ray
from ray_admission.queue import enable_admission
ray.init(address=sys.argv[1])
controller = enable_admission()
job_id = ray.get_runtime_context().get_job_id()
denied = 0
for target in (job_id, sys.argv[2]):
    for quota in ({"CPU": 100}, None):
        try:
            ray.get(controller.set_job_quota.remote(target, quota))
        except PermissionError:
            denied += 1
        else:
            raise AssertionError("Business job changed quota")
try:
    ray.get(controller.set_job_quota.remote(sys.argv[2], {"CPU": 100}, admin_token="forged"))
except PermissionError:
    denied += 1
else:
    raise AssertionError("Forged credentials accepted")
@ray.remote(num_cpus=0.5)
def work():
    return ray.get_runtime_context().get_job_id()
print(json.dumps({"job_id": job_id, "worker_job_id": ray.get(work.remote()), "denied": denied}))
ray.shutdown()
"""
        try:
            wait_until(lambda: len(status(controller)["waiting"]) == 1)
            address = ray._private.worker.global_worker.gcs_client.address
            result = subprocess.run(
                [sys.executable, "-c", code, address, job_id],
                capture_output=True,
                text=True,
                timeout=60,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            other = json.loads(result.stdout.strip().splitlines()[-1])
            self.assertNotEqual(job_id, other["job_id"])
            self.assertEqual(other["job_id"], other["worker_job_id"])
            self.assertEqual(other["denied"], 5)
            self.assertEqual(
                status(controller)["quotas"], {job_id: {"CPU": 0}}
            )
            self.assertFalse(pending.done())
        finally:
            ray.get(
                controller.set_job_quota.remote(
                    job_id, {"CPU": 0.5}, admin_token=self.admin_token
                )
            )
            self.assertEqual(ray.get(pending.result(timeout=30)), job_id)
            pool.shutdown(wait=False, cancel_futures=True)


if __name__ == "__main__":
    unittest.main()
