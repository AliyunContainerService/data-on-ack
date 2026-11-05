"""Submission admission prototype (uses private Ray interfaces).

An administrator first calls ``start(admin_token=...)`` to launch the queue.
Business jobs call ``enable_admission()`` to initialize Ray and take over submissions.
Each submission
waits for the named detached controller's approval, then returns Ray's native result.
The controller configures resource ceilings per job with ``set_job_quota()``.
Each operator charges its declared Ray resources, including ``.options()``
overrides. Requests are FIFO within a job; jobs do not block each other.
Worker startup hooks automatically install interception for nested submissions.
Ray Client and non-Python submissions are not supported by this prototype.

Example::

    import ray
    import secrets
    from ray_admission.queue import (
        disable_admission, enable_admission, policy, start,
    )

    # Administrator process: persist the token privately for future updates.
    token = secrets.token_urlsafe(32)
    controller = start(admin_token=token)
    job_id = ray.get_runtime_context().get_job_id()
    ray.get(controller.set_job_quota.remote(
        job_id, {"CPU": 2, "GPU": 0}, admin_token=token
    ))

    # Business job: connects without the administrator token.
    controller = enable_admission()

    @ray.remote(num_cpus=1)
    @policy(queue="batch", tenant="example")
    def work(value):
        return value + 1

    assert ray.get(work.remote(1)) == 2
    print(ray.get(controller.snapshot.remote()))
    disable_admission()
    ray.kill(controller)  # Explicit cleanup of the detached service, if desired.
    ray.shutdown()

Request kind, job ID, resources, target name/actor ID, and annotation metadata go
to the controller. Arguments stay on the original submission path. Approval is not
execution: original submission failures still propagate normally. Registration
does not cover existing actors, unrelated drivers, or Ray's internal retries.
FIFO means controller arrival order, not execution order. A full quota at the
head blocks later submissions in the same job. Unconfigured jobs and resource
dimensions are unlimited. Task resources are held until all returns are ready;
actor lifetime resources are held until GCS reports DEAD (including restarts).
Actor methods do not charge the actor resources again. Default actors reserve
zero lifetime CPUs, matching Ray. Resource-bearing tasks require positive fixed
returns. Nested submissions can deadlock if the parent holds the job's budget.
Requests larger than the current quota wait until the quota is increased.
Quota configuration and admitted resource reservations survive a controller actor
restart within the same Ray cluster. In-flight admission RPCs are retried by Ray.
Cluster shutdown removes the GCS KV state unless Ray's GCS storage is persistent.
There is no lease expiry for submitters lost before tracking.
Administrator tokens authorize configuration RPCs; they are not a sandbox for
mutually untrusted code with full access to the same Ray cluster. Submission job
IDs and resource claims remain cooperative metadata in this prototype.
"""

import argparse
import asyncio
import base64
import functools
import hashlib
import hmac
import inspect
import json
import math
import os
import threading
import uuid
from collections import Counter, deque
from contextlib import contextmanager

import ray
from ray.actor import ActorClass, ActorHandle
from ray.remote_function import RemoteFunction

_NAME = "ray-submission-queue"
_NAMESPACE = "ray-submission-queue"
_lock = threading.RLock()
_local = threading.local()
_controller = None
_identity = None
_quota_namespace = None
_originals = {}
_runtime_env_cache = {}
_MARKER = "RAY_SUBMISSION_QUEUE_IDENTITY"
_STATE_KEY = "controller-state-v1"
DESCRIPTION = "Per-Job resource quota and FIFO submission queue"


def _load_state():
    from ray.experimental.internal_kv import _internal_kv_get

    raw = _internal_kv_get(_STATE_KEY, namespace=_NAMESPACE)
    if raw is None:
        return None
    state = json.loads(raw)
    if state.get("version") != 1 or not isinstance(state.get("quotas"), list):
        raise RuntimeError("Unsupported or corrupt queue state in Ray GCS KV")
    return state


def _save_state(state, *, overwrite=True):
    from ray.experimental.internal_kv import _internal_kv_put

    _internal_kv_put(
        _STATE_KEY,
        json.dumps(state, sort_keys=True),
        overwrite=overwrite,
        namespace=_NAMESPACE,
    )


def _current_quota_namespace():
    from ray._private.ray_constants import RAY_NAMESPACE_ENVIRONMENT_VARIABLE

    return os.environ.get(RAY_NAMESPACE_ENVIRONMENT_VARIABLE, _NAMESPACE)


def _setup_worker(quota_namespace, previous_key):
    if previous_key:
        from ray._private.runtime_env.setup_hook import load_and_execute_setup_hook

        error = load_and_execute_setup_hook(previous_key)
        if error:
            raise RuntimeError(error)
    # Never create a controller from a worker startup hook.
    controller = ray.get_actor(_NAME, namespace=_NAMESPACE)
    with _lock:
        _install(controller, quota_namespace)


def _propagated_runtime_env(runtime_env):
    from ray._private import ray_constants
    from ray._private.runtime_env.setup_hook import (
        upload_worker_process_setup_hook_if_needed,
    )
    from ray._private.worker import global_worker
    from ray.runtime_env import RuntimeEnv

    hook_key = ray_constants.WORKER_PROCESS_SETUP_HOOK_ENV_VAR
    # Preserve inherited job hooks as well as per-task/actor hooks. Serialize
    # first so callable hooks are represented by their existing GCS keys.
    env = RuntimeEnv(**dict(runtime_env or {}))
    env = upload_worker_process_setup_hook_if_needed(env, global_worker)
    inherited = json.loads(global_worker.runtime_env or "{}")
    inherited_vars = inherited.get("env_vars", {})
    env_vars = dict(env.get("env_vars", {}))
    identity = json.dumps(_identity)
    effective_vars = {**inherited_vars, **env_vars}
    previous_key = effective_vars.get(hook_key)
    if effective_vars.get(_MARKER) == identity and (
        hook_key not in env_vars or env_vars[hook_key] == inherited_vars.get(hook_key)
    ):
        # Reuse the installed hook instead of chaining it at every nesting level.
        env_vars[hook_key] = previous_key
        env_vars[_MARKER] = identity
        env["env_vars"] = env_vars
        return env
    cache_key = (env.serialize(), previous_key, identity)
    if cache_key not in _runtime_env_cache:
        quota_namespace = _quota_namespace

        def setup():
            _setup_worker(quota_namespace, previous_key)

        env_vars.pop(hook_key, None)
        env_vars[_MARKER] = identity
        env["env_vars"] = env_vars
        env["worker_process_setup_hook"] = setup
        env = upload_worker_process_setup_hook_if_needed(env, global_worker)
        _runtime_env_cache[cache_key] = env
    return _runtime_env_cache[cache_key]


class _QueueController:
    """Per-job resource budgets and FIFO admission on one async event loop."""

    def __init__(self, admin_token_digest):
        state = _load_state()
        if state is None:
            state = {
                "version": 1,
                "admin_token_digest": admin_token_digest,
                "quotas": [],
                "permits": [],
            }
            _save_state(state, overwrite=False)
            state = _load_state()
        if state["admin_token_digest"] != admin_token_digest:
            raise PermissionError(
                "Existing queue state requires its original administrator token"
            )
        self._admin_token_digest = state["admin_token_digest"]
        self._counts = Counter()
        self._quotas = {}
        for namespace, job_id, resources in state["quotas"]:
            self._quotas[(namespace, job_id)] = dict(resources)
        self._usage = {}
        self._waiting = {}
        self._permits = {}
        self._tracked = {}
        self._actors = {}
        self._monitor = None
        self._storage_error = None
        self._monitor_error = None
        self._released = set(state.get("released", []))
        self._pending = {}
        for saved in state.get("permits", []):
            ticket = saved["ticket"]
            key = (saved["quota_namespace"], saved["job_id"])
            resources = _validate_resources(saved["resources"])
            self._permits[ticket] = (key, resources)
            self._usage.setdefault(key, Counter()).update(resources)
            if saved.get("refs"):
                self._tracked[ticket] = {
                    ray.cloudpickle.loads(base64.b64decode(value))
                    for value in saved["refs"]
                }
            if saved.get("actor_id"):
                self._actors[ticket] = saved["actor_id"]
        if self._tracked or self._actors:
            self._start_monitor()

    def _persist(self):
        self._ensure_writable()
        try:
            self._write_snapshot()
        except Exception as error:
            # A failed response does not prove the KV write was rejected. Stop
            # all mutations until reconstruction reads the authoritative state.
            self._storage_error = str(error)
            for future in self._pending.values():
                if not future.done():
                    future.set_exception(RuntimeError("Queue storage unavailable; restart required"))
            raise RuntimeError("Queue storage unavailable; restart required") from error

    def _ensure_writable(self):
        if self._storage_error is not None:
            raise RuntimeError("Queue storage unavailable; restart required")

    def _write_snapshot(self):
        _save_state(
            {
                "version": 1,
                "admin_token_digest": self._admin_token_digest,
                "released": sorted(self._released),
                "quotas": [
                    [namespace, job_id, quota]
                    for (namespace, job_id), quota in sorted(self._quotas.items())
                ],
                "permits": [
                    {
                        "ticket": ticket,
                        "quota_namespace": key[0],
                        "job_id": key[1],
                        "resources": resources,
                        "refs": [
                            base64.b64encode(ray.cloudpickle.dumps(ref)).decode()
                            for ref in self._tracked.get(ticket, ())
                        ],
                        "actor_id": self._actors.get(ticket),
                    }
                    for ticket, (key, resources) in sorted(self._permits.items())
                ],
            }
        )

    def check_admin(self, admin_token):
        if not isinstance(admin_token, str) or not admin_token:
            raise PermissionError("Queue administrator credentials required")
        digest = hashlib.sha256(admin_token.encode("utf-8")).hexdigest()
        if not hmac.compare_digest(digest, self._admin_token_digest):
            raise PermissionError("Invalid queue administrator credentials")

    def set_job_quota(
        self, job_id, resources, *, quota_namespace=_NAMESPACE, admin_token=None
    ):
        """Set resource ceilings; unspecified dimensions/jobs are unlimited.

        Requires the queue administrator token, even when changing one's own job.
        Pass None to remove a job's ceilings. Lowering a ceiling does not evict
        admitted work. Existing usage is retained and new requests wait.
        """
        self.check_admin(admin_token)
        self._ensure_writable()
        if not isinstance(job_id, str) or not job_id:
            raise ValueError("job_id must be a nonempty string")
        if not isinstance(quota_namespace, str) or not quota_namespace:
            raise ValueError("quota_namespace must be a nonempty string")
        key = (quota_namespace, job_id)
        quotas = dict(self._quotas)
        if resources is None:
            quotas.pop(key, None)
        else:
            _validate_resources(resources)
            quotas[key] = dict(resources)
        previous = self._quotas
        self._quotas = quotas
        try:
            self._persist()
        except Exception:
            self._quotas = previous
            raise
        self._drain()

    async def acquire(self, request, ticket):
        self._ensure_writable()
        if ticket in self._released:
            raise ValueError("Ticket has already been released")
        if ticket in self._permits:
            return True  # Ray retried an approved RPC after actor restart.
        if ticket in self._pending:
            return await asyncio.shield(self._pending[ticket])
        job_id = request["job_id"]
        quota_namespace = request.get("quota_namespace", _NAMESPACE)
        key = (quota_namespace, job_id)
        resources = _validate_resources(request["resources"])
        if request["kind"] == "actor_method":
            # Actor lifetime resources are charged once at creation.
            self._counts[request["kind"]] += 1
            return True
        if not request.get("trackable", True) and resources:
            raise ValueError("Resource accounting requires positive fixed returns")
        future = asyncio.get_running_loop().create_future()
        self._pending[ticket] = future
        self._waiting.setdefault(key, deque()).append(
            (ticket, request, resources, future)
        )
        try:
            self._drain()
            return await asyncio.shield(future)
        except asyncio.CancelledError:
            self.release(ticket)
            raise
        finally:
            if future.done():
                if not future.cancelled():
                    future.exception()  # Retrieve a storage failure set before awaiting.
                self._pending.pop(ticket, None)

    def _drain(self):
        self._ensure_writable()
        # FIFO within each job: a blocked job does not block another job.
        for key, waiting in list(self._waiting.items()):
            usage = self._usage.setdefault(key, Counter())
            limits = self._quotas.get(key, {})
            while waiting:
                ticket, request, resources, future = waiting[0]
                if future.cancelled():
                    waiting.popleft()
                    continue
                if any(
                    usage[r] + resources.get(r, 0) > limit + 1e-9
                    for r, limit in limits.items()
                ):
                    break
                waiting.popleft()
                if resources:
                    usage.update(resources)
                    self._permits[ticket] = (key, resources)
                    self._persist()
                self._counts[request["kind"]] += 1
                future.set_result(True)
            if not waiting:
                del self._waiting[key]

    def release(self, ticket):
        self._ensure_writable()
        if ticket in self._released:
            return
        permit = self._permits.pop(ticket, None)
        if permit is not None:
            key, resources = permit
            usage = self._usage[key]
            for resource, amount in resources.items():
                usage[resource] = max(0, usage[resource] - amount)
        self._tracked.pop(ticket, None)
        self._actors.pop(ticket, None)
        self._released.add(ticket)
        self._persist()
        for waiting in self._waiting.values():
            for queued_ticket, _, _, future in waiting:
                if queued_ticket == ticket:
                    future.cancel()
                    break
        self._drain()

    async def track(self, ticket, refs):
        self._ensure_writable()
        if ticket not in self._permits:
            return  # A request with no resource demand needs no permit.
        refs = set(refs)
        if ticket in self._actors:
            raise ValueError("Ticket already tracks an actor")
        if ticket in self._tracked:
            if self._tracked[ticket] != refs:
                raise ValueError("Ticket already tracks different task returns")
            self._start_monitor()
            return
        self._tracked[ticket] = refs
        self._persist()
        self._start_monitor()

    async def track_actor(self, ticket, actor_id):
        self._ensure_writable()
        if ticket not in self._permits:
            return
        if ticket in self._tracked:
            raise ValueError("Ticket already tracks task returns")
        if ticket in self._actors:
            if self._actors[ticket] != actor_id:
                raise ValueError("Ticket already tracks a different actor")
            self._start_monitor()
            return
        if actor_id in self._actors.values():
            self.release(ticket)
            return
        self._actors[ticket] = actor_id
        self._persist()
        self._start_monitor()

    def _start_monitor(self):
        if self._monitor is None or self._monitor.done():
            self._monitor = asyncio.create_task(self._watch_completions())

    async def _watch_completions(self):
        delay = 0.02
        while self._tracked or self._actors:
            if self._storage_error is not None:
                return
            try:
                await self._poll_completions()
                self._monitor_error = None
                delay = 0.02
            except asyncio.CancelledError:
                raise
            except Exception as error:
                self._monitor_error = str(error)
                delay = min(max(delay * 2, 0.1), 5)
            await asyncio.sleep(delay)

    async def _poll_completions(self):
        from ray._private.state import actors

        if self._tracked or self._actors:
            refs = list({ref for refs in self._tracked.values() for ref in refs})
            if refs:
                ready, _ = ray.wait(
                    refs, num_returns=len(refs), timeout=0, fetch_local=False
                )
                ready = set(ready)
                for ticket, pending in list(self._tracked.items()):
                    if pending.issubset(ready):
                        self.release(ticket)
            for ticket, actor_id in list(self._actors.items()):
                state = await asyncio.to_thread(actors, actor_id)
                if state and state.get("State") == "DEAD":
                    self.release(ticket)

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
            "permits": [
                {
                    "ticket": ticket,
                    "state": "TRACKED" if ticket in self._tracked or ticket in self._actors else "RESERVED",
                    "quota_namespace": key[0],
                    "job_id": key[1],
                    "resources": dict(resources),
                }
                for ticket, (key, resources) in self._permits.items()
            ],
            "quotas": nested(self._quotas),
            "usage": nested(self._usage),
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
                if not future.cancelled()
            ],
        }


def _validate_resources(resources):
    if not isinstance(resources, dict):
        raise ValueError("resources must be a resource-name to amount dictionary")
    for name, amount in resources.items():
        if (
            not isinstance(name, str)
            or not name
            or isinstance(amount, bool)
            or not isinstance(amount, (int, float))
            or not math.isfinite(amount)
            or amount < 0
        ):
            raise ValueError("resource amounts must be finite nonnegative numbers")
    return {name: amount for name, amount in resources.items() if amount != 0}


@contextmanager
def _bypass():
    previous = getattr(_local, "bypass", False)
    _local.bypass = True
    try:
        yield
    finally:
        _local.bypass = previous


def policy(**metadata):
    """Annotate a function or class *below* ``@ray.remote``.

    Metadata is forwarded unchanged. Resource ceilings are configured only via
    the controller's set_job_quota(), not on individual operators.
    Actor methods currently carry only their method name.
    """

    if "quota" in metadata:
        raise ValueError(
            "Set quotas on the controller with set_job_quota(job_id, resources)"
        )

    def decorate(target):
        target.__ray_admission_policy__ = dict(metadata)
        return target

    return decorate


def _request(kind, target, args, kwargs):
    if kind == "task":
        definition = target._function
        name = target._function_name
    elif kind == "actor_creation":
        definition = target.__ray_metadata__.modified_class
        name = definition.__name__
    else:
        return {
            "kind": kind,
            "name": kwargs.get("method_name", args[0] if args else ""),
            "actor_id": target._ray_actor_id.hex(),
            "metadata": {},
            "job_id": ray.get_runtime_context().get_job_id(),
            "quota_namespace": _quota_namespace,
            "resources": {},
        }
    return {
        "job_id": ray.get_runtime_context().get_job_id(),
        "quota_namespace": _quota_namespace,
        "resources": {},
        "kind": kind,
        "name": name,
        "metadata": dict(getattr(definition, "__ray_admission_policy__", {})),
    }


def _wrap(original, kind):
    signature = inspect.signature(original)

    @functools.wraps(original)
    def submit(target, *args, **kwargs):
        controller = _controller
        if controller is not None and not getattr(_local, "bypass", False):
            # Never route controller RPCs back through the controller itself.
            is_controller = (
                kind == "actor_method"
                and target._ray_actor_id == controller._ray_actor_id
            )
            if not is_controller:
                request = _request(kind, target, args, kwargs)
                bound = None
                if kind != "actor_method":
                    from ray._common import ray_option_utils
                    from ray._common.utils import resources_from_ray_options
                    from ray._private.utils import get_runtime_env_info

                    bound = signature.bind(target, *args, **kwargs)
                    options_key = "task_options" if kind == "task" else "actor_options"
                    options = bound.arguments.setdefault(options_key, {})
                    if kind == "actor_creation" and options.get("get_if_exists"):
                        # Existing handles consume no new resources. Ray recursively
                        # calls _remote(get_if_exists=False) only when creating.
                        return original(*bound.args, **bound.kwargs)
                    defaults = (
                        ray_option_utils.task_options
                        if kind == "task"
                        else ray_option_utils.actor_options
                    )
                    resource_options = {
                        key: value.default_value for key, value in defaults.items()
                    }
                    resource_options.update(options)
                    resources = resources_from_ray_options(resource_options)
                    if kind == "actor_creation":
                        default_cpu = (
                            1
                            if set(resources).difference(
                                {"memory", "object_store_memory"}
                            )
                            else 0
                        )
                        resources.setdefault("CPU", default_cpu)
                    request["resources"] = _validate_resources(resources)
                    if kind == "task":
                        returns = options.get("num_returns", target._num_returns)
                        request["trackable"] = not target._is_generator and (
                            returns is None or isinstance(returns, int) and returns > 0
                        )
                    env = _propagated_runtime_env(options.get("runtime_env"))
                    options["runtime_env"] = env
                    if kind == "task":
                        bound.arguments[
                            "serialized_runtime_env_info"
                        ] = get_runtime_env_info(env, serialize=True)
                ticket = uuid.uuid4().hex
                try:
                    with _bypass():
                        allowed = ray.get(controller.acquire.remote(request, ticket))
                    if allowed is not True:
                        raise PermissionError("Submission rejected by queue controller")
                    if bound is not None:
                        result = original(*bound.args, **bound.kwargs)
                    else:
                        result = original(target, *args, **kwargs)
                except BaseException:
                    # Includes interrupted admission and local serialization errors.
                    # A failed controller must not obscure the original exception.
                    try:
                        with _bypass():
                            ray.get(controller.release.remote(ticket), timeout=5)
                    except Exception:
                        pass
                    raise
                if request["resources"]:
                    with _bypass():
                        if kind == "task":
                            refs = (
                                [result]
                                if isinstance(result, ray.ObjectRef)
                                else result
                            )
                            ray.get(controller.track.remote(ticket, refs))
                        elif kind == "actor_creation":
                            ray.get(
                                controller.track_actor.remote(
                                    ticket, result._ray_actor_id.hex()
                                )
                            )
                return result
        return original(target, *args, **kwargs)

    return submit


def start(*, admin_token):
    """Administrator entry point: create or authenticate to a detached queue.

    Keep the token outside business jobs. Only its hash is stored in the actor;
    no token is included in worker startup hooks or returned by status().
    This starts the service without installing submission interception.
    """
    if not isinstance(admin_token, str) or not admin_token:
        raise ValueError("A nonempty administrator token is required")
    if not ray.is_initialized():
        ray.init(namespace=_NAMESPACE)
    from ray._private.client_mode_hook import client_mode_should_convert

    if client_mode_should_convert():
        raise NotImplementedError("Ray Client is not supported")
    with _bypass():
        controller = (
            ray.remote(_QueueController)
            .options(
                name=_NAME,
                namespace=_NAMESPACE,
                lifetime="detached",
                get_if_exists=True,
                num_cpus=0,
                max_restarts=-1,
                max_task_retries=-1,
            )
            .remote(hashlib.sha256(admin_token.encode("utf-8")).hexdigest())
        )
        # Existing queues retain their credentials; start cannot replace them.
        ray.get(controller.check_admin.remote(admin_token))
    return controller


def enable_admission():
    """Install interception and return the shared detached controller handle.

    Initializes Ray if necessary and connects to an administrator-started queue.
    Does not create the queue or grant configuration privileges.
    Repeated registration with the same
    identity is harmless. Controller failures propagate and prevent submission.
    Admission blocks the calling thread, including when called in an event loop.
    This affects already-defined remote objects and ``.options().remote()`` too.
    New task and actor workers automatically connect to the same controller.
    Existing actors and other drivers are not retroactively instrumented.
    """
    global _controller, _identity
    quota_namespace = _current_quota_namespace()
    if not ray.is_initialized():
        ray.init(namespace=_NAMESPACE)
    from ray._private.client_mode_hook import client_mode_should_convert

    if client_mode_should_convert():
        raise NotImplementedError("Ray Client is not supported")
    with _lock:
        if _controller is not None:
            if _quota_namespace != quota_namespace:
                raise ValueError(
                    "Disable admission before selecting another quota namespace"
                )
            return _controller
        with _bypass():
            try:
                controller = ray.get_actor(_NAME, namespace=_NAMESPACE)
            except ValueError as error:
                raise RuntimeError(
                    "Queue is not running; an administrator must call start() first"
                ) from error
            # Confirm that the controller is ready before enabling interception.
            ray.get(controller.snapshot.remote())
        _install(controller, quota_namespace)
        return controller


def _install(controller, quota_namespace):
    global _controller, _identity, _quota_namespace
    if _controller is not None:
        if _quota_namespace != quota_namespace:
            raise ValueError("Process already attached to another quota namespace")
        return
    for cls, method, kind in (
        (RemoteFunction, "_remote", "task"),
        (ActorClass, "_remote", "actor_creation"),
        (ActorHandle, "_actor_method_call", "actor_method"),
    ):
        original = getattr(cls, method)
        wrapper = _wrap(original, kind)
        _originals[cls, method] = (original, wrapper)
        setattr(cls, method, wrapper)
    _identity = (quota_namespace, uuid.uuid4().hex)
    _quota_namespace = quota_namespace
    _controller = controller


def disable_admission():
    """Restore local submission methods; leave the detached controller alive.

    Does not disable interception in workers that have already started.
    Call before ``ray.shutdown()``. Enabling/disabling admission during concurrent
    submissions is not supported.
    """
    global _controller, _identity, _quota_namespace
    with _lock:
        _controller = None
        _identity = None
        _quota_namespace = None
        for (cls, method), (original, wrapper) in _originals.items():
            if getattr(cls, method) is wrapper:
                setattr(cls, method, original)
        _originals.clear()
        _runtime_env_cache.clear()


def _cli_resources(value):
    try:
        resources = json.loads(value)
        _validate_resources(resources)
        return resources
    except (ValueError, TypeError) as error:
        raise argparse.ArgumentTypeError(str(error)) from error


def _cli_token(args):
    token = args.admin_token or os.environ.get("RAY_ADMISSION_ADMIN_TOKEN")
    if not token:
        raise ValueError("Set RAY_ADMISSION_ADMIN_TOKEN or pass --admin-token")
    return token


def register_cli(parser):
    """Add this implementation's commands to the shared CLI parser."""
    parser.add_argument("--address", default=None, help="Ray cluster address")
    parser.add_argument(
        "--admin-token", help="Administrator token (environment variable is preferred)"
    )
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("start", help="Start or authenticate to the detached queue")
    commands.add_parser("status", help="Show quotas, usage, and waiting requests")
    quota = commands.add_parser("quota", help="Manage per-Job quotas")
    quota_commands = quota.add_subparsers(dest="quota_command", required=True)
    set_quota = quota_commands.add_parser("set")
    set_quota.add_argument("job_id")
    set_quota.add_argument(
        "resources", type=_cli_resources, help='JSON, e.g. {"CPU": 4}'
    )
    delete_quota = quota_commands.add_parser("delete")
    delete_quota.add_argument("job_id")


def run_cli(args):
    """Execute a parsed queue command."""
    try:
        if args.command == "start":
            if args.address is not None:
                ray.init(address=args.address, namespace=_NAMESPACE)
            start(admin_token=_cli_token(args))
            print(json.dumps({"name": _NAME, "namespace": _NAMESPACE}))
        else:
            ray.init(address=args.address, namespace=_NAMESPACE)
            controller = ray.get_actor(_NAME, namespace=_NAMESPACE)
            if args.command == "status":
                print(json.dumps(ray.get(controller.status.remote()), indent=2))
            else:
                resources = args.resources if args.quota_command == "set" else None
                quota_namespace = _current_quota_namespace()
                ray.get(
                    controller.set_job_quota.remote(
                        args.job_id,
                        resources,
                        quota_namespace=quota_namespace,
                        admin_token=_cli_token(args),
                    )
                )
                print(
                    json.dumps(
                        {
                            "quota_namespace": quota_namespace,
                            "job_id": args.job_id,
                            "resources": resources,
                        }
                    )
                )
    finally:
        if ray.is_initialized():
            ray.shutdown()
