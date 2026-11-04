"""Command-line administration of Ray admission implementations."""

import argparse
import json
import os

import ray
from ray_admission import queue as submission_queue

_ADMISSIONS = {"queue": "Per-Job resource quota and FIFO submission queue"}


def _resources(value):
    try:
        resources = json.loads(value)
        submission_queue._validate_resources(resources)
        return resources
    except (ValueError, TypeError) as error:
        raise argparse.ArgumentTypeError(str(error)) from error


def _token(args):
    token = args.admin_token or os.environ.get("RAY_ADMISSION_ADMIN_TOKEN")
    if not token:
        raise ValueError("Set RAY_ADMISSION_ADMIN_TOKEN or pass --admin-token")
    return token


def _parser():
    parser = argparse.ArgumentParser(prog="ray-admission")
    admissions = parser.add_subparsers(dest="admission", required=True)
    admissions.add_parser("list", help="List available admission implementations")
    queue = admissions.add_parser("queue", help=_ADMISSIONS["queue"])
    queue.add_argument("--address", default=None, help="Ray cluster address")
    queue.add_argument(
        "--admin-token", help="Administrator token (environment variable is preferred)"
    )
    commands = queue.add_subparsers(dest="command", required=True)
    commands.add_parser("start", help="Start or authenticate to the detached queue")
    commands.add_parser("status", help="Show quotas, usage, and waiting requests")
    quota = commands.add_parser("quota", help="Manage per-Job quotas")
    quota_commands = quota.add_subparsers(dest="quota_command", required=True)
    set_quota = quota_commands.add_parser("set")
    set_quota.add_argument("job_id")
    set_quota.add_argument("resources", type=_resources, help='JSON, e.g. {"CPU": 4}')
    delete_quota = quota_commands.add_parser("delete")
    delete_quota.add_argument("job_id")
    return parser


def main(argv=None):
    parser = _parser()
    args = parser.parse_args(argv)
    if args.admission == "list":
        for name, description in _ADMISSIONS.items():
            print(f"{name}\t{description}")
        return
    try:
        if args.command == "start":
            if args.address is not None:
                ray.init(address=args.address, namespace=submission_queue._NAMESPACE)
            submission_queue.start(admin_token=_token(args))
            print(
                json.dumps(
                    {
                        "name": submission_queue._NAME,
                        "namespace": submission_queue._NAMESPACE,
                    }
                )
            )
        else:
            ray.init(address=args.address, namespace=submission_queue._NAMESPACE)
            controller = ray.get_actor(
                submission_queue._NAME, namespace=submission_queue._NAMESPACE
            )
            if args.command == "status":
                print(json.dumps(ray.get(controller.status.remote()), indent=2))
            else:
                resources = args.resources if args.quota_command == "set" else None
                ray.get(
                    controller.set_job_quota.remote(
                        args.job_id,
                        resources,
                        quota_namespace=submission_queue._current_quota_namespace(),
                        admin_token=_token(args),
                    )
                )
                print(
                    json.dumps(
                        {
                            "quota_namespace": (
                                submission_queue._current_quota_namespace()
                            ),
                            "job_id": args.job_id,
                            "resources": resources,
                        }
                    )
                )
    except (ValueError, RuntimeError) as error:
        parser.exit(1, f"ray-admission: {error}\n")
    finally:
        if ray.is_initialized():
            ray.shutdown()
