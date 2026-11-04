"""Discover admission implementations and dispatch their CLI commands."""

import argparse
import importlib
import pkgutil

import ray_admission


def _implementations():
    implementations = {}
    for module_info in pkgutil.iter_modules(ray_admission.__path__):
        if (
            module_info.ispkg
            or module_info.name.startswith("_")
            or module_info.name == "cli"
        ):
            continue
        module = importlib.import_module(f"{ray_admission.__name__}.{module_info.name}")
        if callable(getattr(module, "register_cli", None)):
            implementations[module_info.name] = module
    return implementations


def _parser():
    parser = argparse.ArgumentParser(prog="ray-admission")
    commands = parser.add_subparsers(dest="admission", required=True)
    commands.add_parser("list", help="List available admission implementations")
    for name, module in sorted(_implementations().items()):
        module.register_cli(commands.add_parser(name, help=module.DESCRIPTION))
    return parser


def main(argv=None):
    implementations = _implementations()
    parser = _parser()
    args = parser.parse_args(argv)
    if args.admission == "list":
        for name, module in sorted(implementations.items()):
            print(f"{name}\t{module.DESCRIPTION}")
        return
    try:
        implementations[args.admission].run_cli(args)
    except (ValueError, RuntimeError) as error:
        parser.exit(1, f"ray-admission: {error}\n")
