# -*- coding: utf-8 -*-
"""Report agent – generates the final profiling analysis report.

This is the *last* agent in the multi-agent pipeline.  It receives the
intermediate analysis results collected by the planning agent and produces
a complete Markdown report following its own internal plan.
"""
from __future__ import annotations

from typing import Any

from agentscope.agent import AgentBase
from agentscope.message import Msg

from ..tools.report_tools import generate_markdown_report


class ReportAgent(AgentBase):
    """Report generation agent.

    Expects ``msg.metadata`` to carry a dict with the following keys
    (all produced by the upstream analysis agents):

    * ``file_info``       – from ``get_trace_summary``
    * ``summary_stats``   – from ``compute_event_duration_stats``
    * ``category_breakdown`` – from ``compute_category_breakdown``
    * ``outliers``        – from ``detect_duration_outliers``
    * ``timeline_gaps``   – from ``compute_timeline_gaps``
    * ``file_path``       – original file path (for the title)

    The agent creates its own plan (reflected in its output), executes each
    step sequentially, and returns the completed Markdown report as the text
    content of the reply message.
    """

    def __init__(self, name: str = "ReportAgent") -> None:
        super().__init__()
        self.name = name

    async def reply(self, msg: Msg | None = None, **kwargs: Any) -> Msg:
        meta = (msg.metadata if msg else None) or {}

        file_info: dict[str, Any] = meta.get("file_info", {})
        summary_stats: dict[str, Any] = meta.get("summary_stats", {})
        category_breakdown: dict[str, Any] = meta.get("category_breakdown", {})
        outliers: dict[str, Any] = meta.get("outliers", {})
        timeline_gaps: dict[str, Any] = meta.get("timeline_gaps", {})
        file_path: str = meta.get("file_path", "unknown")

        # ---- Report plan (the agent reasons about what to include) ----
        plan_steps = [
            "1. Analyse potential issues from outliers and gaps",
            "2. Derive investigation directions",
            "3. Propose optimisation suggestions",
            "4. Compose overall summary",
            "5. Assemble Markdown report",
        ]
        plan_msg = Msg(
            self.name,
            "Report plan:\n" + "\n".join(plan_steps),
            "assistant",
        )
        await self.print(plan_msg)

        # ---- Step 1: Potential issues ----
        potential_issues = _derive_potential_issues(
            outliers, timeline_gaps, category_breakdown, summary_stats,
        )

        # ---- Step 2: Investigation directions ----
        investigation_directions = _derive_investigation_directions(
            outliers, timeline_gaps, category_breakdown,
        )

        # ---- Step 3: Optimisation suggestions ----
        optimization_suggestions = _derive_optimizations(
            outliers, timeline_gaps, category_breakdown, summary_stats,
        )

        # ---- Step 4: Overall summary ----
        overall_summary = _compose_overall_summary(
            file_info, summary_stats, category_breakdown, outliers,
            timeline_gaps, potential_issues,
        )

        # ---- Step 5: Assemble the report ----
        report = generate_markdown_report(
            title=f"Profiling Analysis Report – {file_path}",
            file_info=file_info,
            summary_stats=summary_stats,
            category_breakdown=category_breakdown,
            outliers=outliers,
            timeline_gaps=timeline_gaps,
            potential_issues=potential_issues,
            investigation_directions=investigation_directions,
            optimization_suggestions=optimization_suggestions,
            overall_summary=overall_summary,
        )

        reply_msg = Msg(
            self.name,
            report,
            "assistant",
            metadata={"report": report},
        )
        await self.print(reply_msg)
        return reply_msg

    async def observe(self, msg: Msg | list[Msg] | None = None) -> None:
        """No-op: this agent is stateless and does not observe messages."""

    async def handle_interrupt(self, *args: Any, **kwargs: Any) -> Msg:
        return Msg(self.name, "Interrupted.", "assistant")


# ---------------------------------------------------------------------------
# Heuristic reasoning helpers (deterministic – no LLM needed)
# ---------------------------------------------------------------------------

def _derive_potential_issues(
    outliers: dict[str, Any],
    gaps: dict[str, Any],
    categories: dict[str, Any],
    stats: dict[str, Any],
) -> list[str]:
    issues: list[str] = []
    outlier_list = outliers.get("outliers", [])
    if outlier_list:
        top = outlier_list[0]
        issues.append(
            f"Detected {outliers.get('total_outliers', len(outlier_list))} "
            f"duration outlier(s). The longest is '{top.get('name', 'N/A')}' "
            f"at {top.get('dur', 0) / 1_000_000:.3f}s (z-score "
            f"{top.get('_z_score', '-')})."
        )

    gap_list = gaps.get("gaps", [])
    if gap_list:
        worst = gap_list[0]
        issues.append(
            f"Found {len(gap_list)} timeline gap(s). The largest gap is "
            f"{worst['duration_us'] / 1_000_000:.3f}s, indicating "
            f"possible idle time or missing instrumentation."
        )

    if categories:
        top_cat = next(iter(categories))
        top_info = categories[top_cat]
        total_all = sum(c.get("total_dur_us", 0) for c in categories.values())
        if total_all > 0:
            pct = top_info["total_dur_us"] / total_all * 100
            if pct > 50:
                issues.append(
                    f"Category '{top_cat}' dominates the trace "
                    f"({pct:.1f}% of total duration), which may indicate "
                    f"a bottleneck or unbalanced workload."
                )

    if stats.get("count", 0) > 0:
        p95 = stats.get("p95_us", 0)
        mean = stats.get("mean_us", 0)
        if mean > 0 and p95 / mean > 10:
            issues.append(
                "P95 latency is more than 10× the mean, suggesting heavy "
                "tail latency that may impact user experience."
            )

    return issues


def _derive_investigation_directions(
    outliers: dict[str, Any],
    gaps: dict[str, Any],
    categories: dict[str, Any],
) -> list[str]:
    directions: list[str] = []
    if outliers.get("outliers"):
        directions.append(
            "Investigate the outlier events to determine whether they are "
            "caused by resource contention, GC pauses, or algorithmic "
            "inefficiencies."
        )
    if gaps.get("gaps"):
        directions.append(
            "Examine the timeline gaps – cross-reference with system metrics "
            "(CPU, memory, I/O) to check for resource starvation or "
            "scheduling delays."
        )
    if categories:
        cats = list(categories.keys())
        if len(cats) > 1:
            directions.append(
                f"Compare the top categories ({', '.join(cats[:3])}) to see "
                f"whether they can be parallelised or whether dependent "
                f"chains exist."
            )
    directions.append(
        "Correlate profiling data with application logs and infrastructure "
        "metrics for a holistic root-cause analysis."
    )
    return directions


def _derive_optimizations(
    outliers: dict[str, Any],
    gaps: dict[str, Any],
    categories: dict[str, Any],
    stats: dict[str, Any],
) -> list[str]:
    suggestions: list[str] = []
    if outliers.get("outliers"):
        suggestions.append(
            "Consider adding caching or batching around the outlier "
            "operations to reduce their worst-case latency."
        )
    if gaps.get("gaps"):
        suggestions.append(
            "Reduce idle gaps by pre-fetching data or pipelining "
            "computations to keep the execution pipeline saturated."
        )
    if stats.get("count", 0) > 0 and stats.get("stdev_us", 0) > stats.get("mean_us", 0):
        suggestions.append(
            "High variance in event durations – investigate whether "
            "partitioning work into more uniform chunks can smooth latency."
        )
    suggestions.append(
        "Profile with finer-grained instrumentation on the hot categories "
        "to pinpoint exact bottleneck functions."
    )
    return suggestions


def _compose_overall_summary(
    file_info: dict[str, Any],
    stats: dict[str, Any],
    categories: dict[str, Any],
    outliers: dict[str, Any],
    gaps: dict[str, Any],
    issues: list[str],
) -> str:
    parts: list[str] = []
    size_mb = file_info.get("file_size_bytes", 0) / (1024 * 1024)
    parts.append(
        f"The trace file ({size_mb:.1f} MB) was parsed and analysed. "
    )
    if stats.get("count"):
        parts.append(
            f"A total of {stats['count']} timed events were examined. "
            f"The mean duration is "
            f"{stats['mean_us'] / 1000:.2f} ms with a P95 of "
            f"{stats.get('p95_us', 0) / 1000:.2f} ms. "
        )
    n_outliers = outliers.get("total_outliers", len(outliers.get("outliers", [])))
    n_gaps = len(gaps.get("gaps", []))
    parts.append(
        f"{n_outliers} outlier event(s) and {n_gaps} timeline gap(s) "
        f"were detected. "
    )
    if issues:
        parts.append(
            f"In total {len(issues)} potential issue(s) were flagged "
            f"for further investigation."
        )
    else:
        parts.append("No critical issues were flagged.")
    return "".join(parts)
