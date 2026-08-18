#!/usr/bin/env python3
"""从 Agent 会话转录文件中提取标记间的 base64 kubeconfig 写入 /tmp/kc.b64。

背景：agentic 流水线不支持 env 注入，secret 只能随 body 展开进提示词；
让 LLM 逐字复述几 KB 的 base64 会随机截断（实测 8480 字符被复述成 5976）。
平台展开后的提示词会随会话转录落盘（~/.claude/projects/*/*.jsonl），
本脚本直接从转录中定位 KCB64BEGIN/KCB64END 标记间的原始值，字节级精确，
不经过模型输出。全程不打印敏感内容，只打印长度与 sha256 供比对。

用法: extract-kc-b64.py [输出路径，默认 /tmp/kc.b64]
退出码: 0 成功；2 secret 为空；3 未找到转录/标记（可回退 Write 工具方式）。
"""
import glob
import hashlib
import json
import os
import re
import sys

PAT = re.compile(r"KCB64BEGIN[\s`]*([A-Za-z0-9+/=]{500,})[\s`]*KCB64END")
EMPTY_PAT = re.compile(r"KCB64BEGIN[\s`]*KCB64END")


def texts(x):
    if isinstance(x, str):
        yield x
    elif isinstance(x, dict):
        for v in x.values():
            yield from texts(v)
    elif isinstance(x, list):
        for v in x:
            yield from texts(v)


def main() -> int:
    out_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/kc.b64"
    home = os.path.expanduser("~")
    files = sorted(
        glob.glob(os.path.join(home, ".claude", "projects", "*", "*.jsonl")),
        key=os.path.getmtime,
        reverse=True,
    )
    if not files:
        print("未找到会话转录文件（~/.claude/projects/*/*.jsonl）", file=sys.stderr)
        return 3
    saw_empty = False
    for path in files:
        try:
            fh = open(path, encoding="utf-8", errors="replace")
        except OSError:
            continue
        with fh:
            for line in fh:
                if "KCB64BEGIN" not in line:
                    continue
                try:
                    obj = json.loads(line)
                except ValueError:
                    continue
                for s in texts(obj):
                    m = PAT.search(s)
                    if m:
                        b64 = m.group(1)
                        with open(out_path, "w", encoding="ascii") as out:
                            out.write(b64)
                        os.chmod(out_path, 0o600)
                        digest = hashlib.sha256(b64.encode("ascii")).hexdigest()
                        print(f"written {out_path}: {len(b64)} chars, sha256={digest}")
                        return 0
                    if EMPTY_PAT.search(s):
                        saw_empty = True
    if saw_empty:
        print("Aone secret TUTORIALS_KUBECONFIG_B64 未配置（标记间为空）", file=sys.stderr)
        return 2
    print("转录文件中未找到标记间的 base64", file=sys.stderr)
    return 3


if __name__ == "__main__":
    sys.exit(main())
