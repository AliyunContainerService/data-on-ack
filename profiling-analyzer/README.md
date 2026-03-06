# AI Profiling Analyzer

基于 [AgentScope](https://github.com/agentscope-ai/agentscope) 框架的多 Agent 协同 Profiling 分析工具。

## 概述

本工具通过多个 Agent 协同运作，对 Chrome Tracing 格式的 Profiling 文件进行自动化分析并生成完整的分析报告。

### Agent 架构

```
ProfilingPlanningAgent  (规划/编排 Agent)
    ├── FileParserAgent         – 解析 & 摘要 Trace 文件
    ├── StatisticsAgent         – 持续时间统计 / 分类分析
    ├── AnomalyDetectionAgent   – 异常检测（离群点 & 时间线空隙）
    └── ReportAgent             – 最终 Markdown 报告生成
```

### 工作流程

1. **ProfilingPlanningAgent** 接收一个或多个 Trace 文件路径，自主设计分析 Plan。
2. 按照 Plan 调用子 Agent：
   - **FileParserAgent** 解析 Chrome Tracing 文件，支持流式解析大文件（GB 级）。
   - **StatisticsAgent** 计算事件持续时间统计和分类占比分析。
   - **AnomalyDetectionAgent** 检测时间异常值（Z-Score）和时间线空隙。
3. **ReportAgent** 收集所有中间结果，制定报告生成 Plan 并逐步执行，生成最终分析报告，包含：
   - **潜在问题点**：自动检测的异常和瓶颈
   - **进一步排查方向**：基于分析结果的排查建议
   - **可能的优化点**：性能优化建议
   - **整体分析概括**：全局分析总结

## 安装

```bash
cd profiling-analyzer
pip install -r requirements.txt
```

## 使用方式

### 命令行

```bash
# 分析单个文件
python -m profiling_analyzer.main /path/to/trace.json

# 分析多个文件
python -m profiling_analyzer.main trace1.json trace2.json

# 输出到文件
python -m profiling_analyzer.main trace.json -o report.md
```

### Python API

```python
import asyncio
from agentscope.message import Msg
from profiling_analyzer.agents import ProfilingPlanningAgent

async def main():
    agent = ProfilingPlanningAgent()
    result = await agent(
        Msg(
            "user",
            "Please analyse the profiling trace files.",
            "user",
            metadata={"file_paths": ["/path/to/trace.json"]},
        ),
    )
    # result.metadata["reports"] contains the list of report strings
    print(result.get_text_content())

asyncio.run(main())
```

### 与 ReActAgent 配合使用

工具函数也可以注册到 agentscope 的 `ReActAgent` 中，由 LLM 自主决定调用：

```python
import asyncio, os
from agentscope.agent import ReActAgent
from agentscope.model import DashScopeChatModel
from agentscope.formatter import DashScopeChatFormatter
from agentscope.memory import InMemoryMemory
from agentscope.tool import Toolkit
from agentscope.plan import PlanNotebook

from profiling_analyzer.tools import (
    parse_trace_file,
    get_trace_summary,
    extract_events_by_category,
    extract_long_events,
    compute_event_duration_stats,
    compute_category_breakdown,
    detect_duration_outliers,
    compute_timeline_gaps,
    generate_markdown_report,
)

async def main():
    toolkit = Toolkit()
    toolkit.register_tool_function(parse_trace_file)
    toolkit.register_tool_function(get_trace_summary)
    toolkit.register_tool_function(extract_events_by_category)
    toolkit.register_tool_function(extract_long_events)
    toolkit.register_tool_function(compute_event_duration_stats)
    toolkit.register_tool_function(compute_category_breakdown)
    toolkit.register_tool_function(detect_duration_outliers)
    toolkit.register_tool_function(compute_timeline_gaps)
    toolkit.register_tool_function(generate_markdown_report)

    agent = ReActAgent(
        name="ProfilingAnalyst",
        sys_prompt=(
            "You are an AI profiling analyst. Analyse the given Chrome "
            "Tracing profiling files and produce a comprehensive report "
            "covering potential issues, investigation directions, "
            "optimisation suggestions, and an overall summary."
        ),
        model=DashScopeChatModel(
            model_name="qwen-max",
            api_key=os.environ["DASHSCOPE_API_KEY"],
            stream=True,
        ),
        formatter=DashScopeChatFormatter(),
        memory=InMemoryMemory(),
        toolkit=toolkit,
        plan_notebook=PlanNotebook(),
        max_iters=20,
    )
    msg = await agent(Msg("user", "Analyse /data/trace.json", "user"))
    print(msg.get_text_content())

asyncio.run(main())
```

## 测试

```bash
cd profiling-analyzer
python -m pytest tests/ -v
```

## Chrome Tracing 格式

本工具处理的 Chrome Tracing JSON 格式示例：

```json
[
  {"name": "op_A", "cat": "gpu", "ph": "X", "ts": 0, "dur": 1500, "pid": 1, "tid": 1},
  {"name": "op_B", "cat": "cpu", "ph": "X", "ts": 2000, "dur": 500, "pid": 1, "tid": 2}
]
```

或者带有 `traceEvents` 包装：

```json
{"traceEvents": [...]}
```

文件大小可从几 MB 到 GB 级不等；大文件通过 ijson 流式解析以控制内存使用。
