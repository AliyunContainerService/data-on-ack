# -*- coding: utf-8 -*-
"""Anomaly detection agent – identifies outliers and timeline gaps.

Follows the agentscope ``AgentBase`` async pattern.
"""
from __future__ import annotations

from typing import Any

from agentscope.agent import AgentBase
from agentscope.message import Msg

from ..tools.statistics_tools import (
    detect_duration_outliers,
    compute_timeline_gaps,
)


class AnomalyDetectionAgent(AgentBase):
    """Agent that detects anomalies (outliers and timeline gaps).

    Expects ``msg.metadata`` to contain:

    * ``events`` – list of Chrome Tracing event dicts (required)
    * ``action`` – ``"outliers"`` or ``"gaps"`` (default ``"outliers"``)
    * ``params`` – optional extra parameters for the underlying tool.
    """

    def __init__(self, name: str = "AnomalyDetectionAgent") -> None:
        super().__init__()
        self.name = name

    async def reply(self, msg: Msg | None = None, **kwargs: Any) -> Msg:
        meta = (msg.metadata if msg else None) or {}
        events: list[dict[str, Any]] = meta.get("events", [])
        action: str = meta.get("action", "outliers")
        params: dict[str, Any] = meta.get("params", {})

        if not events:
            result = {"error": "No events provided in message metadata."}
        elif action == "outliers":
            result = detect_duration_outliers(
                events,
                z_threshold=params.get("z_threshold", 3.0),
            )
        elif action == "gaps":
            result = compute_timeline_gaps(
                events,
                min_gap_us=params.get("min_gap_us", 100_000),
            )
        else:
            result = {"error": f"Unknown action: {action}"}

        reply_msg = Msg(
            self.name,
            f"Anomaly detection complete (action={action}).",
            "assistant",
            metadata=result,
        )
        await self.print(reply_msg)
        return reply_msg

    async def observe(self, msg: Msg | list[Msg] | None = None) -> None:
        pass

    async def handle_interrupt(self, *args: Any, **kwargs: Any) -> Msg:
        return Msg(self.name, "Interrupted.", "assistant")
