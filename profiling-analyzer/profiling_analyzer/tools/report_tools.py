# -*- coding: utf-8 -*-
"""Report generation tools for profiling analysis."""
from __future__ import annotations

import json
from datetime import datetime
from typing import Any


def format_report_section(
    title: str,
    content: str,
    level: int = 2,
) -> str:
    """Format a single section of a Markdown report.

    Args:
        title: Section heading.
        content: Body text (Markdown).
        level: Heading level (1-4).

    Returns:
        Formatted Markdown string for the section.
    """
    prefix = "#" * max(1, min(level, 4))
    return f"{prefix} {title}\n\n{content}\n"


def generate_markdown_report(
    title: str,
    file_info: dict[str, Any],
    summary_stats: dict[str, Any],
    category_breakdown: dict[str, Any],
    outliers: dict[str, Any],
    timeline_gaps: dict[str, Any],
    potential_issues: list[str],
    investigation_directions: list[str],
    optimization_suggestions: list[str],
    overall_summary: str,
) -> str:
    """Generate a complete Markdown profiling analysis report.

    Args:
        title: Report title.
        file_info: Output of ``get_trace_summary``.
        summary_stats: Output of ``compute_event_duration_stats``.
        category_breakdown: Output of ``compute_category_breakdown``.
        outliers: Output of ``detect_duration_outliers``.
        timeline_gaps: Output of ``compute_timeline_gaps``.
        potential_issues: Bullet list of potential issues found.
        investigation_directions: Bullet list of further investigation items.
        optimization_suggestions: Bullet list of optimization suggestions.
        overall_summary: Free-form overall analysis text.

    Returns:
        A complete Markdown report string.
    """
    parts: list[str] = []

    # Title
    parts.append(f"# {title}\n")
    parts.append(f"*Generated at: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}*\n")

    # 1. File overview
    parts.append(format_report_section("File Overview", _render_file_info(file_info)))

    # 2. Duration statistics
    parts.append(format_report_section("Duration Statistics", _render_stats(summary_stats)))

    # 3. Category breakdown
    parts.append(format_report_section("Category Breakdown", _render_categories(category_breakdown)))

    # 4. Outlier events
    parts.append(format_report_section("Outlier Events", _render_outliers(outliers)))

    # 5. Timeline gaps
    parts.append(format_report_section("Timeline Gaps", _render_gaps(timeline_gaps)))

    # 6. Potential issues
    parts.append(format_report_section(
        "Potential Issues",
        _render_bullet_list(potential_issues) or "_No issues identified._",
    ))

    # 7. Further investigation
    parts.append(format_report_section(
        "Further Investigation Directions",
        _render_bullet_list(investigation_directions) or "_None._",
    ))

    # 8. Optimization suggestions
    parts.append(format_report_section(
        "Optimization Suggestions",
        _render_bullet_list(optimization_suggestions) or "_None._",
    ))

    # 9. Overall summary
    parts.append(format_report_section("Overall Analysis Summary", overall_summary))

    return "\n".join(parts)


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------

def _render_file_info(info: dict[str, Any]) -> str:
    size_mb = info.get("file_size_bytes", 0) / (1024 * 1024)
    lines = [
        f"- **File size**: {size_mb:.2f} MB",
        f"- **Sampled events**: {info.get('sampled_events', 'N/A')}",
        f"- **Categories**: {', '.join(info.get('categories', []))}",
        f"- **Phases**: {', '.join(info.get('phases', []))}",
        f"- **Processes**: {len(info.get('process_ids', []))}",
        f"- **Threads**: {len(info.get('thread_ids', []))}",
    ]
    tr = info.get("time_range_us")
    if tr:
        span_s = (tr["max_us"] - tr["min_us"]) / 1_000_000
        lines.append(f"- **Time span**: {span_s:.3f} s")
    return "\n".join(lines)


def _render_stats(stats: dict[str, Any]) -> str:
    if stats.get("count", 0) == 0:
        return "_No duration data available._"

    def _fmt(us: float) -> str:
        if us >= 1_000_000:
            return f"{us / 1_000_000:.3f} s"
        if us >= 1_000:
            return f"{us / 1_000:.2f} ms"
        return f"{us:.1f} μs"

    return "\n".join([
        f"| Metric | Value |",
        f"|--------|-------|",
        f"| Count  | {stats['count']} |",
        f"| Total  | {_fmt(stats['total_us'])} |",
        f"| Mean   | {_fmt(stats['mean_us'])} |",
        f"| Median | {_fmt(stats['median_us'])} |",
        f"| Stdev  | {_fmt(stats['stdev_us'])} |",
        f"| Min    | {_fmt(stats['min_us'])} |",
        f"| Max    | {_fmt(stats['max_us'])} |",
        f"| P95    | {_fmt(stats['p95_us'])} |",
    ])


def _render_categories(breakdown: dict[str, Any]) -> str:
    if not breakdown:
        return "_No category data._"

    lines = ["| Category | Count | Total Duration | Mean Duration |", "|----------|-------|----------------|---------------|"]
    for cat, info in list(breakdown.items())[:20]:
        total = info["total_dur_us"]
        mean = info["mean_dur_us"]
        lines.append(
            f"| {cat} | {info['count']} | {_fmt_us(total)} | {_fmt_us(mean)} |",
        )
    return "\n".join(lines)


def _render_outliers(outliers: dict[str, Any]) -> str:
    items = outliers.get("outliers", [])
    if not items:
        return "_No outlier events detected._"

    total = outliers.get("total_outliers", len(items))
    lines = [f"Detected **{total}** outlier event(s). Top entries:\n"]
    lines.append("| Name | Category | Duration | Z-Score |")
    lines.append("|------|----------|----------|---------|")
    for ev in items[:15]:
        lines.append(
            f"| {ev.get('name', 'N/A')} "
            f"| {ev.get('cat', 'N/A')} "
            f"| {_fmt_us(ev.get('dur', 0))} "
            f"| {ev.get('_z_score', '-')} |",
        )
    return "\n".join(lines)


def _render_gaps(gaps_data: dict[str, Any]) -> str:
    gaps = gaps_data.get("gaps", [])
    if not gaps:
        return "_No significant timeline gaps found._"

    lines = [f"Found **{len(gaps)}** gap(s) in the timeline:\n"]
    lines.append("| Start | End | Duration |")
    lines.append("|-------|-----|----------|")
    for g in gaps[:15]:
        lines.append(
            f"| {_fmt_us(g['start_us'])} | {_fmt_us(g['end_us'])} | {_fmt_us(g['duration_us'])} |",
        )
    return "\n".join(lines)


def _render_bullet_list(items: list[str]) -> str:
    if not items:
        return ""
    return "\n".join(f"- {item}" for item in items)


def _fmt_us(us: float) -> str:
    if us >= 1_000_000:
        return f"{us / 1_000_000:.3f} s"
    if us >= 1_000:
        return f"{us / 1_000:.2f} ms"
    return f"{us:.1f} μs"
