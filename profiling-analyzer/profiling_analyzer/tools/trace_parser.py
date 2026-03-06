# -*- coding: utf-8 -*-
"""Chrome Tracing format file parser tools.

Chrome Tracing format (JSON) files contain profiling events with fields such
as ``name``, ``cat`` (category), ``ph`` (phase), ``ts`` (timestamp in μs),
``dur`` (duration in μs), ``pid``, ``tid``, and optional ``args``.

This module provides streaming-capable parsing utilities so that even
multi-GB trace files can be handled without exhausting memory.
"""
from __future__ import annotations

import ijson
import json
import os
from typing import Any


def parse_trace_file(
    file_path: str,
    max_events: int = 0,
) -> dict[str, Any]:
    """Parse a Chrome Tracing JSON file and return its events.

    For large files the parser uses streaming (ijson) to avoid loading the
    entire file into memory.  The caller may set *max_events* to a positive
    number to limit how many events are returned (useful for an initial
    overview).

    Args:
        file_path: Absolute or relative path to the trace JSON file.
        max_events: Maximum number of events to return.  ``0`` means all.

    Returns:
        A dict with keys ``events`` (list of event dicts), ``total_parsed``
        (number of events actually read) and ``file_size_bytes``.
    """
    if not os.path.isfile(file_path):
        return {
            "error": f"File not found: {file_path}",
            "events": [],
            "total_parsed": 0,
            "file_size_bytes": 0,
        }

    file_size = os.path.getsize(file_path)
    use_streaming = file_size > 50 * 1024 * 1024  # >50 MB → streaming

    events: list[dict[str, Any]] = []
    total_parsed = 0

    if use_streaming:
        events, total_parsed = _parse_streaming(file_path, max_events)
    else:
        events, total_parsed = _parse_full(file_path, max_events)

    return {
        "events": events,
        "total_parsed": total_parsed,
        "file_size_bytes": file_size,
    }


def _parse_full(
    file_path: str,
    max_events: int,
) -> tuple[list[dict[str, Any]], int]:
    """Load the entire file into memory (suitable for small files)."""
    with open(file_path, "r", encoding="utf-8") as fh:
        data = json.load(fh)

    raw_events: list[dict[str, Any]]
    if isinstance(data, list):
        raw_events = data
    elif isinstance(data, dict):
        raw_events = data.get("traceEvents", data.get("events", []))
    else:
        raw_events = []

    if max_events > 0:
        raw_events = raw_events[:max_events]

    return raw_events, len(raw_events)


def _parse_streaming(
    file_path: str,
    max_events: int,
) -> tuple[list[dict[str, Any]], int]:
    """Stream-parse the file using ijson (for large files)."""
    events: list[dict[str, Any]] = []
    total = 0
    with open(file_path, "rb") as fh:
        # Try common JSON structures: top-level array or "traceEvents" key
        try:
            parser = ijson.items(fh, "traceEvents.item")
        except Exception:
            fh.seek(0)
            parser = ijson.items(fh, "item")

        for event in parser:
            total += 1
            if max_events > 0 and total > max_events:
                break
            events.append(dict(event))

    return events, total


def get_trace_summary(file_path: str, sample_size: int = 1000) -> dict[str, Any]:
    """Return a high-level summary of a trace file without loading all events.

    Args:
        file_path: Path to the trace JSON file.
        sample_size: Number of events to sample for the summary.

    Returns:
        A dict with ``file_size_bytes``, ``total_events``, ``categories``,
        ``phases``, ``process_ids``, ``thread_ids``, and ``time_range_us``.
    """
    result = parse_trace_file(file_path, max_events=sample_size)
    if result.get("error"):
        return result

    events = result["events"]
    categories: set[str] = set()
    phases: set[str] = set()
    pids: set[int] = set()
    tids: set[int] = set()
    timestamps: list[float] = []

    for ev in events:
        cat = ev.get("cat", "")
        if cat:
            for c in cat.split(","):
                categories.add(c.strip())
        phases.add(ev.get("ph", ""))
        if "pid" in ev:
            pids.add(ev["pid"])
        if "tid" in ev:
            tids.add(ev["tid"])
        if "ts" in ev:
            ts = ev["ts"]
            timestamps.append(ts)
            dur = ev.get("dur", 0)
            if dur:
                timestamps.append(ts + dur)

    time_range = None
    if timestamps:
        time_range = {"min_us": min(timestamps), "max_us": max(timestamps)}

    return {
        "file_size_bytes": result["file_size_bytes"],
        "sampled_events": len(events),
        "categories": sorted(categories),
        "phases": sorted(phases),
        "process_ids": sorted(pids),
        "thread_ids": sorted(tids)[:20],
        "time_range_us": time_range,
    }


def extract_events_by_category(
    file_path: str,
    category: str,
    max_events: int = 5000,
) -> dict[str, Any]:
    """Extract events belonging to a specific category.

    Args:
        file_path: Path to the trace JSON file.
        category: Category name to filter on (matched against the comma-
            separated ``cat`` field of each event).
        max_events: Cap on number of events returned.

    Returns:
        A dict with ``events`` and ``total_matched``.
    """
    result = parse_trace_file(file_path, max_events=0)
    if result.get("error"):
        return result

    matched: list[dict[str, Any]] = []
    for ev in result["events"]:
        cats = [c.strip() for c in ev.get("cat", "").split(",")]
        if category in cats:
            matched.append(ev)
            if len(matched) >= max_events:
                break

    return {"events": matched, "total_matched": len(matched)}


def extract_long_events(
    file_path: str,
    threshold_us: float = 1_000_000,
    max_events: int = 200,
) -> dict[str, Any]:
    """Return events whose duration exceeds a threshold.

    Args:
        file_path: Path to the trace JSON file.
        threshold_us: Minimum duration in microseconds.
        max_events: Maximum events to return.

    Returns:
        A dict with ``events`` and ``total_matched``.
    """
    result = parse_trace_file(file_path, max_events=0)
    if result.get("error"):
        return result

    long: list[dict[str, Any]] = []
    for ev in result["events"]:
        dur = ev.get("dur", 0)
        if dur and dur >= threshold_us:
            long.append(ev)
            if len(long) >= max_events:
                break

    long.sort(key=lambda e: e.get("dur", 0), reverse=True)
    return {"events": long, "total_matched": len(long)}
