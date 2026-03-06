# -*- coding: utf-8 -*-
"""Entry-point for the multi-agent profiling analyser.

Usage::

    python -m profiling_analyzer.main /path/to/trace1.json [/path/to/trace2.json ...]

The program orchestrates several specialised agents (built on the agentscope
framework) to parse, analyse, and produce a Markdown report for each
Chrome Tracing format profiling file provided on the command line.

Agent architecture::

    ProfilingPlanningAgent  (orchestrator)
        ├── FileParserAgent        – parse & summarise trace files
        ├── StatisticsAgent        – duration stats / category breakdown
        ├── AnomalyDetectionAgent  – outlier & gap detection
        └── ReportAgent            – final Markdown report generation
"""
from __future__ import annotations

import argparse
import asyncio
import os
import sys

from agentscope.message import Msg

from .agents import ProfilingPlanningAgent


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Multi-agent AI profiling analyser (agentscope-based).",
    )
    parser.add_argument(
        "files",
        nargs="+",
        help="One or more Chrome Tracing JSON files to analyse.",
    )
    parser.add_argument(
        "-o", "--output",
        default=None,
        help="Optional output file for the report (Markdown).  "
             "Defaults to stdout.",
    )
    return parser.parse_args()


async def run(file_paths: list[str], output: str | None = None) -> str:
    """Run the full multi-agent analysis pipeline.

    Args:
        file_paths: Trace file paths to analyse.
        output: Optional path to write the Markdown report.

    Returns:
        The combined Markdown report text.
    """
    agent = ProfilingPlanningAgent()

    result_msg = await agent(
        Msg(
            "user",
            "Please analyse the following profiling trace files and "
            "generate a comprehensive report.",
            "user",
            metadata={"file_paths": file_paths},
        ),
    )

    report_text = result_msg.get_text_content()

    if output:
        os.makedirs(os.path.dirname(output) or ".", exist_ok=True)
        with open(output, "w", encoding="utf-8") as fh:
            fh.write(report_text)
        print(f"\nReport written to {output}")

    return report_text


def main() -> None:
    args = parse_args()
    # Validate files exist
    for fp in args.files:
        if not os.path.isfile(fp):
            print(f"Error: file not found – {fp}", file=sys.stderr)
            sys.exit(1)

    report = asyncio.run(run(args.files, args.output))

    if not args.output:
        print(report)


if __name__ == "__main__":
    main()
