#!/usr/bin/env bash
# Generate prompts.jsonl for slime remote-agent RL from a harbor task dataset directory.
# One line per task; metadata.instance_id = the task directory name.
#
# Usage:
#   bash gen-prompts.sh <dataset-dir> [-o prompts.jsonl] [-n 3] [-i] [--prompt "..."]
#     <dataset-dir>   directory containing one subdir per task
#                     (e.g. ./data/swe-bench-verified or /var/model-dataset/swe-bench-verified)
#     -o              output file (default ./prompts.jsonl)
#     -n              max tasks to include, 0 = all (default 3)
#     -i              write into the head pod instead of locally (uses $POD env)
#     --prompt        instruction text (default: the SWE fix instruction)
#
# Examples:
#   bash gen-prompts.sh /var/model-dataset/swe-bench-verified -n 5
#   POD=$POD bash gen-prompts.sh /var/model-dataset/swe-bench-verified -i
set -euo pipefail

DATASET_DIR=""
OUT="prompts.jsonl"
MAX=3
IN_POD=0
PROMPT="Fix the bug described in the task instruction in the mounted repository. Read the instruction, locate the root cause, and edit the source so the failing tests pass."

while [ $# -gt 0 ]; do
  case "$1" in
    -o) OUT="$2"; shift 2 ;;
    -n) MAX="$2"; shift 2 ;;
    -i) IN_POD=1; shift ;;
    --prompt) PROMPT="$2"; shift 2 ;;
    -*) echo "unknown flag: $1" >&2; exit 1 ;;
    *) DATASET_DIR="$1"; shift ;;
  esac
done
[ -n "$DATASET_DIR" ] || { echo "usage: $0 <dataset-dir> [-o out.jsonl] [-n max] [-i] [--prompt '...']" >&2; exit 1; }

gen() {  # gen <dataset-dir> <out> — runs locally or in the pod
  local dir="$1" out="$2"
  python3 - "$dir" "$out" "$MAX" "$PROMPT" <<'PY'
import json, sys
from pathlib import Path

dataset, out, max_tasks, prompt = sys.argv[1], Path(sys.argv[2]), int(sys.argv[3]), sys.argv[4]
tasks = sorted(p for p in Path(dataset).iterdir() if p.is_dir() and (p / "task.toml").exists())
if not tasks:
    sys.exit(f"no task directories with task.toml under {dataset}")
if max_tasks > 0:
    tasks = tasks[:max_tasks]
with open(out, "w") as fh:
    for t in tasks:
        fh.write(json.dumps({
            "prompt": prompt,
            "task_name": t.name,
            "metadata": {"instance_id": t.name},
        }, ensure_ascii=False) + "\n")
print(f"[ok] wrote {len(tasks)} prompts -> {out}")
for t in tasks:
    print("  ", t.name)
PY
}

if [ "$IN_POD" = 1 ]; then
  : "${POD:?set POD=<head-pod> for in-pod generation}"
  DST="/root/slime/examples/remote_agent/prompts.jsonl"
  kubectl exec -n default "$POD" -- mkdir -p "$(dirname "$DST")"
  kubectl exec -n default "$POD" -- bash -s -- "$DATASET_DIR" "$DST" "$MAX" "$PROMPT" <<'SH'
python3 - "$@" <<'PY'
import json, sys
from pathlib import Path
dataset, out, max_tasks, prompt = sys.argv[1], sys.argv[2], int(sys.argv[3]), sys.argv[4]
tasks = sorted(p for p in Path(dataset).iterdir() if p.is_dir() and (p / "task.toml").exists())
if not tasks:
    sys.exit(f"no task directories with task.toml under {dataset}")
if max_tasks > 0:
    tasks = tasks[:max_tasks]
with open(out, "w") as fh:
    for t in tasks:
        fh.write(json.dumps({"prompt": prompt, "task_name": t.name,
                             "metadata": {"instance_id": t.name}}, ensure_ascii=False) + "\n")
print(f"[ok] wrote {len(tasks)} prompts -> {out}")
for t in tasks:
    print("  ", t.name)
PY
SH
  kubectl exec -n default "$POD" -- head -2 "$DST"
else
  gen "$DATASET_DIR" "$OUT"
fi
