# -*- coding: utf-8 -*-
"""File parser agent – responsible for parsing and summarising trace files.

This agent follows the agentscope ``AgentBase`` pattern.  It wraps the
low-level trace-parser tools and exposes them as ``reply`` / ``observe``
methods that other agents can call in a multi-agent workflow.
"""
from __future__ import annotations

from typing import Any

from agentscope.agent import AgentBase
from agentscope.message import Msg

from ..tools.trace_parser import (
    get_trace_summary,
    extract_events_by_category,
    extract_long_events,
    parse_trace_file,
)


class FileParserAgent(AgentBase):
    """Agent that parses Chrome Tracing files and produces structured summaries.

    The agent accepts a message whose ``metadata`` carries:

    * ``file_path`` – path to the trace file (required)
    * ``action``    – one of ``"summary"``, ``"by_category"``,
      ``"long_events"``, ``"parse"`` (default ``"summary"``)
    * ``params``    – optional dict of extra parameters forwarded to the
      underlying tool function.
    """

    def __init__(self, name: str = "FileParserAgent") -> None:
        super().__init__()
        self.name = name

    async def reply(self, msg: Msg | None = None, **kwargs: Any) -> Msg:
        """Parse the trace file according to the requested action."""
        meta = (msg.metadata if msg else None) or {}
        file_path: str = meta.get("file_path", "")
        action: str = meta.get("action", "summary")
        params: dict[str, Any] = meta.get("params", {})

        if not file_path:
            result = {"error": "No file_path provided in message metadata."}
        elif action == "summary":
            result = get_trace_summary(file_path, **params)
        elif action == "by_category":
            category = params.get("category", "")
            result = extract_events_by_category(
                file_path,
                category=category,
                max_events=params.get("max_events", 5000),
            )
        elif action == "long_events":
            result = extract_long_events(
                file_path,
                threshold_us=params.get("threshold_us", 1_000_000),
                max_events=params.get("max_events", 200),
            )
        elif action == "parse":
            result = parse_trace_file(
                file_path,
                max_events=params.get("max_events", 0),
            )
        else:
            result = {"error": f"Unknown action: {action}"}

        reply_msg = Msg(
            self.name,
            f"File parsing complete (action={action}).",
            "assistant",
            metadata=result,
        )
        await self.print(reply_msg)
        return reply_msg

    async def observe(self, msg: Msg | list[Msg] | None = None) -> None:
        """No-op: this agent is stateless and does not observe messages."""

    async def handle_interrupt(self, *args: Any, **kwargs: Any) -> Msg:
        return Msg(self.name, "Interrupted.", "assistant")
