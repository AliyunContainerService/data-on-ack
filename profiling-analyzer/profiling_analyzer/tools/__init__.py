# -*- coding: utf-8 -*-
"""Tool functions for the profiling analyzer agents."""
from .trace_parser import (
    parse_trace_file,
    get_trace_summary,
    extract_events_by_category,
    extract_long_events,
)
from .statistics_tools import (
    compute_event_duration_stats,
    compute_category_breakdown,
    detect_duration_outliers,
    compute_timeline_gaps,
)
from .report_tools import (
    format_report_section,
    generate_markdown_report,
)

__all__ = [
    "parse_trace_file",
    "get_trace_summary",
    "extract_events_by_category",
    "extract_long_events",
    "compute_event_duration_stats",
    "compute_category_breakdown",
    "detect_duration_outliers",
    "compute_timeline_gaps",
    "format_report_section",
    "generate_markdown_report",
]
