#!/usr/bin/env bash
# Submit one harbor task to the Rollout Server, poll to completion, print the reward.
# Adapted from the RolloutServer project's tutorial 00 (submit a task).
#
# Usage:
#   SERVER=http://localhost:8080 TASK_DIR=./data/swe-bench-verified/astropy__astropy-14309 \
#     AGENT=oracle bash submit-task.sh
# Env:
#   SERVER    Rollout Server URL           (default http://localhost:8080)
#   VIEWER    Viewer URL                   (default http://localhost:8081, optional)
#   TASK_DIR  task directory to submit     (required)
#   AGENT     agent name                   (default oracle; use nop for a no-op)
#   JOB_ID    job id                       (default demo)
#   POOL      sandbox set / pool name      (default empty; set for sandbox-pool backends)
set -euo pipefail

SERVER="${SERVER:-http://localhost:8080}"
VIEWER="${VIEWER:-http://localhost:8081}"
AGENT="${AGENT:-oracle}"
JOB_ID="${JOB_ID:-demo}"
POOL="${POOL:-}"

TASK_DIR="${TASK_DIR:?set TASK_DIR to the task directory (must contain task.toml)}"
TASK_NAME="$(basename "$TASK_DIR")"
ARCHIVE="/tmp/${TASK_NAME}.tar.gz"

# --- health check ---------------------------------------------------------
curl -sf "$SERVER/health" -o /dev/null || {
  echo "ERROR: cannot reach $SERVER — port-forward first:"
  echo "  kubectl port-forward svc/kube-rl 8080:8080 -n kube-rl"; exit 1; }

# --- package the task ------------------------------------------------------
# task_path MUST equal the tar's top-level directory name
COPYFILE_DISABLE=1 tar czf "$ARCHIVE" -C "$(dirname "$TASK_DIR")" "$TASK_NAME"

# --- submit asynchronously --------------------------------------------------
META="{\"job_id\":\"$JOB_ID\",\"task_id\":\"${TASK_NAME}-${AGENT}\",\"task_path\":\"$TASK_NAME\",\"agent\":{\"name\":\"$AGENT\"}"
[ "$AGENT" = nop ] && META+=',"verifier":{"disable":true}'
[ -n "$POOL" ] && META+=",\"environment_kwargs\":{\"sandbox_set_name\":\"$POOL\",\"override_claim_image\":true}"
META+='}'

RUN_ID=$(curl -s -X POST "$SERVER/api/v1/runs/async" \
  -F "metadata=$META" -F "task_archive=@$ARCHIVE" \
  | python3 -c 'import sys,json;print(json.load(sys.stdin)["run_id"])')
echo "run_id=$RUN_ID  (task=$TASK_NAME agent=$AGENT)"

# --- poll to a terminal state ----------------------------------------------
FINAL=unknown
for i in $(seq 1 240); do
  read -r STATUS PHASE <<<"$(curl -s "$SERVER/api/v1/runs/async/$RUN_ID/status" \
    | python3 -c 'import sys,json;d=json.load(sys.stdin);print(d.get("status","?"),d.get("phase","-"))')"
  [ "$PHASE" != "${LAST:-}" ] && { printf '  [%4ds] status=%-10s phase=%s\n' "$((i*5))" "$STATUS" "$PHASE"; LAST=$PHASE; }
  if [ "$STATUS" = completed ] || [ "$STATUS" = failed ]; then FINAL=$STATUS; break; fi
  sleep 5
done
echo "final status: $FINAL"

# --- fetch the result --------------------------------------------------------
curl -s "$SERVER/api/v1/runs/async/$RUN_ID/result" | python3 -m json.tool

# --- viewer pointer -----------------------------------------------------------
echo "trajectory: $VIEWER/ (job '$JOB_ID', trial '$RUN_ID')"
