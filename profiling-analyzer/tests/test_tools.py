# -*- coding: utf-8 -*-
"""Tests for the profiling analyser tool modules."""
from __future__ import annotations

import json
import os
import tempfile
import unittest

# Tools under test
from profiling_analyzer.tools.trace_parser import (
    parse_trace_file,
    get_trace_summary,
    extract_events_by_category,
    extract_long_events,
)
from profiling_analyzer.tools.statistics_tools import (
    compute_event_duration_stats,
    compute_category_breakdown,
    detect_duration_outliers,
    compute_timeline_gaps,
)
from profiling_analyzer.tools.report_tools import (
    format_report_section,
    generate_markdown_report,
)


# ---------------------------------------------------------------------------
# Sample data helpers
# ---------------------------------------------------------------------------

SAMPLE_EVENTS = [
    {"name": "A", "cat": "gpu", "ph": "X", "ts": 0, "dur": 100, "pid": 1, "tid": 1},
    {"name": "B", "cat": "gpu", "ph": "X", "ts": 200, "dur": 300, "pid": 1, "tid": 1},
    {"name": "C", "cat": "cpu", "ph": "X", "ts": 600, "dur": 50, "pid": 1, "tid": 2},
    {"name": "D", "cat": "cpu", "ph": "X", "ts": 700, "dur": 2_000_000, "pid": 2, "tid": 3},
    {"name": "E", "cat": "io",  "ph": "X", "ts": 3_000_000, "dur": 500, "pid": 2, "tid": 3},
]


def _write_trace_file(events, *, wrapper: str = "list") -> str:
    """Write events to a temporary JSON file and return the path."""
    if wrapper == "list":
        data = events
    else:
        data = {"traceEvents": events}
    fd, path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w") as fh:
        json.dump(data, fh)
    return path


# ---------------------------------------------------------------------------
# trace_parser tests
# ---------------------------------------------------------------------------

class TestTraceParser(unittest.TestCase):
    def setUp(self):
        self.path_list = _write_trace_file(SAMPLE_EVENTS, wrapper="list")
        self.path_dict = _write_trace_file(SAMPLE_EVENTS, wrapper="dict")

    def tearDown(self):
        os.unlink(self.path_list)
        os.unlink(self.path_dict)

    def test_parse_trace_file_list_format(self):
        res = parse_trace_file(self.path_list)
        self.assertEqual(len(res["events"]), 5)
        self.assertGreater(res["file_size_bytes"], 0)

    def test_parse_trace_file_dict_format(self):
        res = parse_trace_file(self.path_dict)
        self.assertEqual(len(res["events"]), 5)

    def test_parse_trace_file_max_events(self):
        res = parse_trace_file(self.path_list, max_events=2)
        self.assertEqual(len(res["events"]), 2)

    def test_parse_missing_file(self):
        res = parse_trace_file("/nonexistent/path.json")
        self.assertIn("error", res)
        self.assertEqual(len(res["events"]), 0)

    def test_get_trace_summary(self):
        res = get_trace_summary(self.path_list)
        self.assertIn("categories", res)
        self.assertIn("gpu", res["categories"])
        self.assertIn("cpu", res["categories"])
        self.assertEqual(len(res["process_ids"]), 2)

    def test_extract_events_by_category(self):
        res = extract_events_by_category(self.path_list, "gpu")
        self.assertEqual(res["total_matched"], 2)

    def test_extract_long_events(self):
        res = extract_long_events(self.path_list, threshold_us=1_000)
        self.assertEqual(res["total_matched"], 1)
        self.assertEqual(res["events"][0]["name"], "D")


# ---------------------------------------------------------------------------
# statistics_tools tests
# ---------------------------------------------------------------------------

class TestStatisticsTools(unittest.TestCase):
    def test_duration_stats(self):
        stats = compute_event_duration_stats(SAMPLE_EVENTS)
        self.assertEqual(stats["count"], 5)
        self.assertEqual(stats["max_us"], 2_000_000)
        self.assertEqual(stats["min_us"], 50)

    def test_duration_stats_empty(self):
        stats = compute_event_duration_stats([])
        self.assertEqual(stats["count"], 0)

    def test_category_breakdown(self):
        bd = compute_category_breakdown(SAMPLE_EVENTS)
        self.assertIn("gpu", bd)
        self.assertIn("cpu", bd)
        self.assertEqual(bd["gpu"]["count"], 2)

    def test_detect_duration_outliers(self):
        out = detect_duration_outliers(SAMPLE_EVENTS, z_threshold=1.5)
        # Event D with dur=2_000_000 should be an outlier
        self.assertGreater(len(out["outliers"]), 0)
        self.assertEqual(out["outliers"][0]["name"], "D")

    def test_detect_duration_outliers_empty(self):
        out = detect_duration_outliers([], z_threshold=3.0)
        self.assertEqual(len(out["outliers"]), 0)

    def test_compute_timeline_gaps(self):
        gaps = compute_timeline_gaps(SAMPLE_EVENTS, min_gap_us=50)
        # There should be gaps between event groups
        self.assertGreater(len(gaps["gaps"]), 0)


# ---------------------------------------------------------------------------
# report_tools tests
# ---------------------------------------------------------------------------

class TestReportTools(unittest.TestCase):
    def test_format_report_section(self):
        s = format_report_section("Test", "body text", level=3)
        self.assertTrue(s.startswith("### Test"))
        self.assertIn("body text", s)

    def test_generate_markdown_report(self):
        report = generate_markdown_report(
            title="Test Report",
            file_info={
                "file_size_bytes": 1024 * 1024,
                "sampled_events": 100,
                "categories": ["gpu", "cpu"],
                "phases": ["X"],
                "process_ids": [1],
                "thread_ids": [1, 2],
                "time_range_us": {"min_us": 0, "max_us": 1_000_000},
            },
            summary_stats=compute_event_duration_stats(SAMPLE_EVENTS),
            category_breakdown=compute_category_breakdown(SAMPLE_EVENTS),
            outliers=detect_duration_outliers(SAMPLE_EVENTS),
            timeline_gaps=compute_timeline_gaps(SAMPLE_EVENTS),
            potential_issues=["Issue A", "Issue B"],
            investigation_directions=["Dir 1"],
            optimization_suggestions=["Opt 1"],
            overall_summary="All good.",
        )
        self.assertIn("# Test Report", report)
        self.assertIn("Issue A", report)
        self.assertIn("Dir 1", report)
        self.assertIn("Opt 1", report)
        self.assertIn("All good.", report)


if __name__ == "__main__":
    unittest.main()
