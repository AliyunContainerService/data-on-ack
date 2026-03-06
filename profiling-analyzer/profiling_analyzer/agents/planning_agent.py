# -*- coding: utf-8 -*-
"""Planning agent – the orchestrator of the multi-agent profiling pipeline.

This agent is the entry-point of the analysis workflow.  It:

1. Receives one or more trace file paths.
2. Creates a high-level analysis *Plan*.
3. Delegates sub-tasks to specialised child agents
   (``FileParserAgent``, ``StatisticsAgent``, ``AnomalyDetectionAgent``).
4. Collects intermediate results.
5. Hands the collected results to the ``ReportAgent`` to produce the final
   analysis report.

The agent follows the agentscope ``AgentBase`` pattern and is fully
async-compatible.
"""
from __future__ import annotations

import asyncio
from typing import Any

from agentscope.agent import AgentBase
from agentscope.message import Msg

from .file_parser_agent import FileParserAgent
from .statistics_agent import StatisticsAgent
from .anomaly_detection_agent import AnomalyDetectionAgent
from .report_agent import ReportAgent


class ProfilingPlanningAgent(AgentBase):
    """Orchestrator agent that plans and delegates profiling analysis.

    Usage::

        agent = ProfilingPlanningAgent()
        result = await agent(Msg(
            "user",
            "Please analyse the following trace files.",
            "user",
            metadata={"file_paths": ["/data/trace.json"]},
        ))
        print(result.metadata["reports"])

    ``msg.metadata`` must contain:

    * ``file_paths`` – list of trace file paths to analyse.
    """

    def __init__(self, name: str = "ProfilingPlanningAgent") -> None:
        super().__init__()
        self.name = name

        # Child agents
        self.file_parser = FileParserAgent()
        self.statistics = StatisticsAgent()
        self.anomaly = AnomalyDetectionAgent()
        self.reporter = ReportAgent()

    # ------------------------------------------------------------------
    # Core reply
    # ------------------------------------------------------------------

    async def reply(self, msg: Msg | None = None, **kwargs: Any) -> Msg:
        meta = (msg.metadata if msg else None) or {}
        file_paths: list[str] = meta.get("file_paths", [])

        if not file_paths:
            err = Msg(self.name, "No file_paths provided.", "assistant")
            await self.print(err)
            return err

        # ---- Step 0: Present the plan --------------------------------
        plan = self._create_plan(file_paths)
        plan_msg = Msg(self.name, plan, "assistant")
        await self.print(plan_msg)

        # ---- Analyse each file ----------------------------------------
        all_reports: list[str] = []
        for fp in file_paths:
            report_text = await self._analyse_single_file(fp)
            all_reports.append(report_text)

        combined = "\n\n---\n\n".join(all_reports)
        final_msg = Msg(
            self.name,
            f"All {len(file_paths)} file(s) analysed. Reports follow.\n\n{combined}",
            "assistant",
            metadata={"reports": all_reports},
        )
        await self.print(final_msg)
        return final_msg

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _create_plan(self, file_paths: list[str]) -> str:
        """Build a human-readable plan string."""
        lines = [
            "## Profiling Analysis Plan",
            "",
            f"Files to analyse: {len(file_paths)}",
        ]
        for i, fp in enumerate(file_paths, 1):
            lines.append(f"  {i}. {fp}")

        lines += [
            "",
            "### Steps per file",
            "1. **Parse & Summarise** – use FileParserAgent to get file overview.",
            "2. **Full Parse** – load all events for detailed analysis.",
            "3. **Statistics** – compute duration stats and category breakdown "
            "(StatisticsAgent).",
            "4. **Anomaly Detection** – find outliers and timeline gaps "
            "(AnomalyDetectionAgent).",
            "5. **Report Generation** – hand results to ReportAgent to "
            "produce the final Markdown report.",
        ]
        return "\n".join(lines)

    async def _analyse_single_file(self, file_path: str) -> str:
        """Run the full analysis pipeline for one trace file."""

        # 1. Summary
        summary_msg = await self.file_parser(
            Msg("plan", "Summarise trace file.", "user",
                metadata={"file_path": file_path, "action": "summary"}),
        )
        file_info: dict[str, Any] = summary_msg.metadata or {}

        # 2. Full parse (cap at 50k events to bound memory)
        parse_msg = await self.file_parser(
            Msg("plan", "Parse trace file.", "user",
                metadata={
                    "file_path": file_path,
                    "action": "parse",
                    "params": {"max_events": 50_000},
                }),
        )
        events: list[dict[str, Any]] = (parse_msg.metadata or {}).get("events", [])

        # 3. Statistics (run duration stats & category breakdown in parallel)
        stats_future = self.statistics(
            Msg("plan", "Compute duration stats.", "user",
                metadata={"events": events, "action": "duration_stats"}),
        )
        cat_future = self.statistics(
            Msg("plan", "Compute category breakdown.", "user",
                metadata={"events": events, "action": "category_breakdown"}),
        )
        stats_msg, cat_msg = await asyncio.gather(stats_future, cat_future)
        summary_stats: dict[str, Any] = stats_msg.metadata or {}
        category_breakdown: dict[str, Any] = cat_msg.metadata or {}

        # 4. Anomaly detection (outliers + gaps in parallel)
        outlier_future = self.anomaly(
            Msg("plan", "Detect outliers.", "user",
                metadata={"events": events, "action": "outliers"}),
        )
        gap_future = self.anomaly(
            Msg("plan", "Detect timeline gaps.", "user",
                metadata={"events": events, "action": "gaps"}),
        )
        outlier_msg, gap_msg = await asyncio.gather(outlier_future, gap_future)
        outliers: dict[str, Any] = outlier_msg.metadata or {}
        timeline_gaps: dict[str, Any] = gap_msg.metadata or {}

        # 5. Report
        report_msg = await self.reporter(
            Msg("plan", "Generate analysis report.", "user",
                metadata={
                    "file_path": file_path,
                    "file_info": file_info,
                    "summary_stats": summary_stats,
                    "category_breakdown": category_breakdown,
                    "outliers": outliers,
                    "timeline_gaps": timeline_gaps,
                }),
        )

        return report_msg.metadata.get("report", report_msg.get_text_content())

    async def observe(self, msg: Msg | list[Msg] | None = None) -> None:
        """No-op: the orchestrator does not observe messages directly."""

    async def handle_interrupt(self, *args: Any, **kwargs: Any) -> Msg:
        return Msg(self.name, "Interrupted.", "assistant")
