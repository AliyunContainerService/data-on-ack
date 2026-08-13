#!/bin/bash
# Sweep UCX tuning knobs for NIXL transfers over eRDMA.
#
# Runs nixl_tuning_sweep.py once per configuration on two pods and prints a
# table, so the effect of each knob is measured rather than guessed. Usage:
#   ./ucx_knob_sweep.sh <target-pod> <initiator-pod> <target-ip> <READ|WRITE> <dram|vram>
set -uo pipefail

T_POD="${1:?target pod}"; I_POD="${2:?initiator pod}"; PEER="${3:?target ip}"
OP="${4:-READ}"; MEM="${5:-vram}"
PORT=20000

BASE='UCX_NET_DEVICES=erdma_0:1 UCX_TLS=rc_verbs,ud_verbs,self,sm,cuda_copy,cuda_ipc'

run() {
  local label="$1" extra="$2"
  PORT=$((PORT + 1))
  kubectl exec "$T_POD" -- bash -lc \
    "export $BASE $extra; timeout 150 python /tmp/sweep.py --role target \
       --op $OP --mem $MEM --descs 16 --iters 5 --port $PORT" >/dev/null 2>&1 &
  sleep 8
  local out
  out=$(kubectl exec "$I_POD" -- bash -lc \
    "export $BASE $extra; timeout 150 python /tmp/sweep.py --role initiator \
       --op $OP --mem $MEM --descs 16 --iters 5 --peer $PEER --port $PORT" 2>&1 \
    | awk '$1=="16"{print $2}')
  printf '%-42s %s GiB/s\n' "$label" "${out:-FAILED}"
  wait 2>/dev/null
  sleep 2
}

echo "== $OP / $MEM, 256 MiB, 16 descriptors, best of 5"
run "baseline"                       ""
run "RNDV_FRAG_SIZE cuda:16M"        "UCX_RNDV_FRAG_SIZE=host:512K,cuda:16M"
run "RNDV_FRAG_ALLOC_COUNT 512"      "UCX_RNDV_FRAG_ALLOC_COUNT=host:512,cuda:512"
run "RNDV_FRAG_MEM_TYPES=cuda"       "UCX_RNDV_FRAG_MEM_TYPES=cuda"
run "RNDV_SCHEME=put_zcopy"          "UCX_RNDV_SCHEME=put_zcopy"
run "RNDV_SCHEME=get_zcopy"          "UCX_RNDV_SCHEME=get_zcopy"
run "CUDA_COPY_MAX_REG_RATIO=1.0"    "UCX_CUDA_COPY_MAX_REG_RATIO=1.0"
run "both rails"                     "UCX_NET_DEVICES=erdma_0:1,erdma_1:1 UCX_MAX_RNDV_RAILS=2"
run "MAX_RD_ATOMIC=16"               "UCX_RC_VERBS_MAX_RD_ATOMIC=16"
