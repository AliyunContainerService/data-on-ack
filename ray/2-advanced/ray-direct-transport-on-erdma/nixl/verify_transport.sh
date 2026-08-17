#!/bin/bash
# Determine which transport a NIXL run used, rather than relying on UCX_TLS.
#
# For each configuration: read the eRDMA byte counters of both devices before
# and after the run, and record the lane UCX selected. A run that used eRDMA
# moves the counters by the payload volume; a TCP run leaves them flat.
#
# Usage: verify_transport.sh <target-pod> <initiator-pod> <target-ip>
set -uo pipefail

T_POD="${1:?target pod}"; I_POD="${2:?initiator pod}"; PEER="${3:?target ip}"
PORT=20500
ITERS=5
MIB=256

counters() {  # total tx+rx bytes across both eRDMA devices, on the initiator
  kubectl exec "$I_POD" -- bash -lc '
    t=0
    for d in erdma_0 erdma_1; do
      for c in hw_tx_bytes_cnt hw_rx_bytes_cnt; do
        t=$((t + $(cat /sys/class/infiniband/$d/ports/1/hw_counters/$c)))
      done
    done
    echo $t'
}

run() {
  local label="$1" op="$2" mem="$3" tls="$4" dev="${5:-erdma_0:1}"
  PORT=$((PORT + 1))
  local env="UCX_NET_DEVICES=$dev UCX_TLS=$tls"

  local before after out lane
  before=$(counters)
  kubectl exec "$T_POD" -- bash -lc \
    "export $env; timeout 150 python /tmp/sweep.py --role target --op $op \
       --mem $mem --descs 16 --iters $ITERS --port $PORT" >/dev/null 2>&1 &
  sleep 8
  out=$(kubectl exec "$I_POD" -- bash -lc \
    "export $env UCX_LOG_LEVEL=info; timeout 150 python /tmp/sweep.py \
       --role initiator --op $op --mem $mem --descs 16 --iters $ITERS \
       --peer $PEER --port $PORT" 2>&1)
  after=$(counters)
  wait 2>/dev/null

  local gibs
  gibs=$(echo "$out" | awk '$1=="16"{print $2}')
  lane=$(echo "$out" | grep -o 'inter-node cfg#[0-9]*.*' | head -1 \
         | grep -oE 'rma\([^)]*\)|am\([^)]*\)' | head -2 | tr '\n' ' ')
  python3 - "$before" "$after" "$ITERS" "$MIB" "${gibs:-0}" "$label" "$lane" <<'PY'
import sys
before, after, iters, mib, gibs, label, lane = sys.argv[1:8]
moved = (int(after) - int(before)) / 2**30
payload = int(iters) * int(mib) / 1024
print(f"{label:<34} {gibs:>6} GiB/s   erdma bytes {moved:6.2f} GiB "
      f"(payload {payload:.2f})   {lane}")
PY
  sleep 2
}

echo "RDMA-only TLS (no tcp in the list):"
run "READ  dram  rc_verbs"  READ  dram "rc_verbs,ud_verbs,self,sm"
run "WRITE dram  rc_verbs"  WRITE dram "rc_verbs,ud_verbs,self,sm"
run "READ  vram  rc_verbs"  READ  vram "rc_verbs,ud_verbs,self,sm,cuda_copy,cuda_ipc"
run "WRITE vram  rc_verbs"  WRITE vram "rc_verbs,ud_verbs,self,sm,cuda_copy,cuda_ipc"

echo
echo "TCP-only TLS, for contrast:"
# TCP needs an ethernet device: with UCX_NET_DEVICES pinned to erdma_0:1 the
# tcp transport has nothing to run on and backend creation simply fails, which
# is itself a useful sanity check on the RDMA rows above.
run "READ  dram  tcp"       READ  dram "tcp,self,sm" eth0
run "WRITE dram  tcp"       WRITE dram "tcp,self,sm" eth0
run "READ  vram  tcp"       READ  vram "tcp,self,sm,cuda_copy,cuda_ipc" eth0
run "WRITE vram  tcp"       WRITE vram "tcp,self,sm,cuda_copy,cuda_ipc" eth0
