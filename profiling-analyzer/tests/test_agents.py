# -*- coding: utf-8 -*-
"""Tests for the agent modules (async)."""
from __future__ import annotations

import asyncio
import json
import os
import tempfile
import unittest

from agentscope.message import Msg

from profiling_analyzer.agents.file_parser_agent import FileParserAgent
from profiling_analyzer.agents.statistics_agent import StatisticsAgent
from profiling_analyzer.agents.anomaly_detection_agent import AnomalyDetectionAgent
from profiling_analyzer.agents.report_agent import ReportAgent
from profiling_analyzer.agents.planning_agent import ProfilingPlanningAgent


SAMPLE_EVENTS = [
    {"name": "A", "cat": "gpu", "ph": "X", "ts": 0, "dur": 100, "pid": 1, "tid": 1},
    {"name": "B", "cat": "gpu", "ph": "X", "ts": 200, "dur": 300, "pid": 1, "tid": 1},
    {"name": "C", "cat": "cpu", "ph": "X", "ts": 600, "dur": 50, "pid": 1, "tid": 2},
    {"name": "D", "cat": "cpu", "ph": "X", "ts": 700, "dur": 2_000_000, "pid": 2, "tid": 3},
    {"name": "E", "cat": "io",  "ph": "X", "ts": 3_000_000, "dur": 500, "pid": 2, "tid": 3},
]


def _write_trace(events):
    fd, path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w") as fh:
        json.dump(events, fh)
    return path


class TestFileParserAgent(unittest.TestCase):
    def setUp(self):
        self.path = _write_trace(SAMPLE_EVENTS)
        self.agent = FileParserAgent()

    def tearDown(self):
        os.unlink(self.path)

    def test_summary(self):
        msg = Msg("user", "parse", "user",
                  metadata={"file_path": self.path, "action": "summary"})
        result = asyncio.run(self.agent(msg))
        self.assertIn("categories", result.metadata)

    def test_missing_file(self):
        msg = Msg("user", "parse", "user",
                  metadata={"file_path": "/no/such/file.json", "action": "summary"})
        result = asyncio.run(self.agent(msg))
        self.assertIn("error", result.metadata)


class TestStatisticsAgent(unittest.TestCase):
    def test_duration_stats(self):
        agent = StatisticsAgent()
        msg = Msg("user", "stats", "user",
                  metadata={"events": SAMPLE_EVENTS, "action": "duration_stats"})
        result = asyncio.run(agent(msg))
        self.assertEqual(result.metadata["count"], 5)

    def test_category_breakdown(self):
        agent = StatisticsAgent()
        msg = Msg("user", "cats", "user",
                  metadata={"events": SAMPLE_EVENTS, "action": "category_breakdown"})
        result = asyncio.run(agent(msg))
        self.assertIn("gpu", result.metadata)


class TestAnomalyDetectionAgent(unittest.TestCase):
    def test_outliers(self):
        agent = AnomalyDetectionAgent()
        msg = Msg("user", "outliers", "user",
                  metadata={"events": SAMPLE_EVENTS, "action": "outliers"})
        result = asyncio.run(agent(msg))
        self.assertIn("outliers", result.metadata)

    def test_gaps(self):
        agent = AnomalyDetectionAgent()
        msg = Msg("user", "gaps", "user",
                  metadata={"events": SAMPLE_EVENTS, "action": "gaps"})
        result = asyncio.run(agent(msg))
        self.assertIn("gaps", result.metadata)


class TestReportAgent(unittest.TestCase):
    def test_report_generation(self):
        agent = ReportAgent()
        msg = Msg("user", "report", "user", metadata={
            "file_path": "/tmp/test.json",
            "file_info": {
                "file_size_bytes": 1024,
                "sampled_events": 5,
                "categories": ["gpu"],
                "phases": ["X"],
                "process_ids": [1],
                "thread_ids": [1],
            },
            "summary_stats": {"count": 5, "total_us": 100, "mean_us": 20,
                              "median_us": 15, "stdev_us": 10, "min_us": 5,
                              "max_us": 50, "p95_us": 45},
            "category_breakdown": {"gpu": {"count": 5, "total_dur_us": 100, "mean_dur_us": 20}},
            "outliers": {"outliers": [], "total_outliers": 0, "stats": {}},
            "timeline_gaps": {"gaps": []},
        })
        result = asyncio.run(agent(msg))
        self.assertIn("report", result.metadata)
        self.assertIn("Profiling Analysis Report", result.metadata["report"])


class TestPlanningAgent(unittest.TestCase):
    def setUp(self):
        self.path = _write_trace(SAMPLE_EVENTS)

    def tearDown(self):
        os.unlink(self.path)

    def test_full_pipeline(self):
        agent = ProfilingPlanningAgent()
        msg = Msg("user", "analyse", "user",
                  metadata={"file_paths": [self.path]})
        result = asyncio.run(agent(msg))
        self.assertIn("reports", result.metadata)
        self.assertEqual(len(result.metadata["reports"]), 1)
        self.assertIn("Profiling Analysis Report", result.metadata["reports"][0])


if __name__ == "__main__":
    unittest.main()
