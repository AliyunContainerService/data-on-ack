# -*- coding: utf-8 -*-
"""Statistics agent – computes statistical analysis on trace events.

Follows the agentscope ``AgentBase`` async pattern.
"""
from __future__ import annotations

from typing import Any

from agentscope.agent import AgentBase
from agentscope.message import Msg

from ..tools.statistics_tools import (
    compute_event_duration_stats,
    compute_category_breakdown,
)


class StatisticsAgent(AgentBase):
    """Agent that computes duration statistics and category breakdowns.

    Expects ``msg.metadata`` to contain:

    * ``events`` – list of Chrome Tracing event dicts (required)
    * ``action`` – ``"duration_stats"`` or ``"category_breakdown"``
      (default ``"duration_stats"``)
    """

    def __init__(self, name: str = "StatisticsAgent") -> None:
        super().__init__()
        self.name = name

    async def reply(self, msg: Msg | None = None, **kwargs: Any) -> Msg:
        meta = (msg.metadata if msg else None) or {}
        events: list[dict[str, Any]] = meta.get("events", [])
        action: str = meta.get("action", "duration_stats")

        if not events:
            result = {"error": "No events provided in message metadata."}
        elif action == "duration_stats":
            result = compute_event_duration_stats(events)
        elif action == "category_breakdown":
            result = compute_category_breakdown(events)
        else:
            result = {"error": f"Unknown action: {action}"}

        reply_msg = Msg(
            self.name,
            f"Statistical analysis complete (action={action}).",
            "assistant",
            metadata=result,
        )
        await self.print(reply_msg)
        return reply_msg

    async def observe(self, msg: Msg | list[Msg] | None = None) -> None:
        pass

    async def handle_interrupt(self, *args: Any, **kwargs: Any) -> Msg:
        return Msg(self.name, "Interrupted.", "assistant")
