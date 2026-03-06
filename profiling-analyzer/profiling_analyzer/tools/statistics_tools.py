# -*- coding: utf-8 -*-
"""Statistical analysis tools for Chrome Tracing events."""
from __future__ import annotations

import statistics
from typing import Any


def compute_event_duration_stats(
    events: list[dict[str, Any]],
) -> dict[str, Any]:
    """Compute duration statistics across all events that have a ``dur`` field.

    Args:
        events: List of Chrome Tracing event dicts.

    Returns:
        A dict with ``count``, ``total_us``, ``mean_us``, ``median_us``,
        ``stdev_us``, ``min_us``, ``max_us``, and ``p95_us``.
    """
    durations = [ev["dur"] for ev in events if ev.get("dur")]
    if not durations:
        return {"count": 0}

    durations_sorted = sorted(durations)
    p95_idx = int(len(durations_sorted) * 0.95)

    return {
        "count": len(durations),
        "total_us": sum(durations),
        "mean_us": statistics.mean(durations),
        "median_us": statistics.median(durations),
        "stdev_us": statistics.stdev(durations) if len(durations) > 1 else 0.0,
        "min_us": min(durations),
        "max_us": max(durations),
        "p95_us": durations_sorted[p95_idx] if p95_idx < len(durations_sorted) else durations_sorted[-1],
    }


def compute_category_breakdown(
    events: list[dict[str, Any]],
) -> dict[str, Any]:
    """Break down total duration by category.

    Args:
        events: List of Chrome Tracing event dicts.

    Returns:
        A dict mapping each category to ``{"count": int, "total_dur_us": float,
        "mean_dur_us": float}``, sorted by total duration descending.
    """
    cat_data: dict[str, dict[str, Any]] = {}
    for ev in events:
        cats = [c.strip() for c in ev.get("cat", "unknown").split(",")]
        dur = ev.get("dur", 0)
        for cat in cats:
            if not cat:
                cat = "unknown"
            if cat not in cat_data:
                cat_data[cat] = {"count": 0, "total_dur_us": 0.0, "durations": []}
            cat_data[cat]["count"] += 1
            if dur:
                cat_data[cat]["total_dur_us"] += dur
                cat_data[cat]["durations"].append(dur)

    result: dict[str, Any] = {}
    for cat in sorted(cat_data, key=lambda c: cat_data[c]["total_dur_us"], reverse=True):
        info = cat_data[cat]
        mean_dur = (
            statistics.mean(info["durations"]) if info["durations"] else 0.0
        )
        result[cat] = {
            "count": info["count"],
            "total_dur_us": info["total_dur_us"],
            "mean_dur_us": mean_dur,
        }
    return result


def detect_duration_outliers(
    events: list[dict[str, Any]],
    z_threshold: float = 3.0,
) -> dict[str, Any]:
    """Detect duration outliers using a simple z-score method.

    Args:
        events: List of Chrome Tracing event dicts.
        z_threshold: Number of standard deviations above the mean to consider
            an event an outlier.

    Returns:
        A dict with ``outliers`` (list of events) and ``stats`` used for
        detection.
    """
    durations = [(i, ev) for i, ev in enumerate(events) if ev.get("dur")]
    if len(durations) < 2:
        return {"outliers": [], "stats": {}}

    durs = [d for _, d in [(i, ev["dur"]) for i, ev in durations]]
    mean_val = statistics.mean(durs)
    stdev_val = statistics.stdev(durs)

    if stdev_val == 0:
        return {"outliers": [], "stats": {"mean_us": mean_val, "stdev_us": 0}}

    outliers: list[dict[str, Any]] = []
    for _, ev in durations:
        z = (ev["dur"] - mean_val) / stdev_val
        if z > z_threshold:
            outliers.append({**ev, "_z_score": round(z, 2)})

    outliers.sort(key=lambda e: e.get("dur", 0), reverse=True)
    return {
        "outliers": outliers[:100],
        "total_outliers": len(outliers),
        "stats": {"mean_us": mean_val, "stdev_us": stdev_val},
    }


def compute_timeline_gaps(
    events: list[dict[str, Any]],
    min_gap_us: float = 100_000,
) -> dict[str, Any]:
    """Find gaps in the timeline where no events are running.

    Args:
        events: List of Chrome Tracing event dicts.
        min_gap_us: Minimum gap duration in microseconds to report.

    Returns:
        A dict with ``gaps`` (list of ``{start_us, end_us, duration_us}``)
        sorted by duration descending.
    """
    timed = [
        (ev["ts"], ev["ts"] + ev.get("dur", 0))
        for ev in events
        if "ts" in ev
    ]
    if not timed:
        return {"gaps": []}

    timed.sort(key=lambda t: t[0])

    gaps: list[dict[str, float]] = []
    current_end = timed[0][1]
    for start, end in timed[1:]:
        if start > current_end:
            gap = start - current_end
            if gap >= min_gap_us:
                gaps.append({
                    "start_us": current_end,
                    "end_us": start,
                    "duration_us": gap,
                })
        current_end = max(current_end, end)

    gaps.sort(key=lambda g: g["duration_us"], reverse=True)
    return {"gaps": gaps}
