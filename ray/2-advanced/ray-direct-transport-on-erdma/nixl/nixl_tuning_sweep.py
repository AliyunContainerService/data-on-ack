"""Why is NIXL/UCX slower than ucx_perftest on the same eRDMA link?

Sweeps the two things that plausibly explain the gap, so the answer is
measured rather than guessed:

  --descs N     split the transfer into N descriptors in one NIXL request
                (a single 256 MiB RDMA READ leaves little for UCX to
                pipeline; several descriptors give it independent work)
  --busy-poll   drive progress in a tight loop instead of sleeping between
                check_xfer_state() calls

Usage mirrors nixl_check.py:
    python nixl_tuning_sweep.py --role target    [--mem dram|vram]
    python nixl_tuning_sweep.py --role initiator --peer <ip> [--mem dram|vram]
"""

import argparse
import socket
import time

import torch
from nixl._api import nixl_agent, nixl_agent_config

TOTAL = 256 * 1024 * 1024  # 256 MiB
DESC_COUNTS = (1, 4, 16, 64)  # default sweep, override with --descs


def send_msg(sock, data: bytes):
    sock.sendall(len(data).to_bytes(8, "big") + data)


def recv_msg(sock) -> bytes:
    return _recv_exact(sock, int.from_bytes(_recv_exact(sock, 8), "big"))


def _recv_exact(sock, n: int) -> bytes:
    buf = b""
    while len(buf) < n:
        chunk = sock.recv(n - len(buf))
        if not chunk:
            raise ConnectionError("peer closed")
        buf += chunk
    return buf


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--role", choices=["target", "initiator"], required=True)
    ap.add_argument("--backend", default="UCX")
    ap.add_argument("--mem", choices=["dram", "vram"], default="dram")
    ap.add_argument("--peer", default="")
    ap.add_argument("--port", type=int, default=19999)
    ap.add_argument("--iters", type=int, default=5)
    ap.add_argument("--op", choices=["READ", "WRITE"], default="READ")
    ap.add_argument(
        "--descs",
        default=",".join(str(n) for n in DESC_COUNTS),
        help="comma-separated descriptor counts to sweep",
    )
    args = ap.parse_args()

    desc_counts = tuple(int(n) for n in args.descs.split(","))
    agent = nixl_agent(args.role, nixl_agent_config(backends=[args.backend]))

    device = "cuda" if args.mem == "vram" else "cpu"
    payload_side = "target" if args.op == "READ" else "initiator"
    fill = 7.0 if args.role == payload_side else 0.0
    buf = torch.full((TOTAL // 4,), fill, dtype=torch.float32, device=device)
    if device == "cuda":
        torch.cuda.synchronize()

    # Register the whole buffer once, then describe it at different
    # granularities. reg_descs are (addr, len, dev_id) tuples.
    base = buf.data_ptr()
    mem_type = "VRAM" if device == "cuda" else "DRAM"
    dev_id = buf.device.index or 0 if device == "cuda" else 0
    reg = agent.register_memory([buf])
    assert reg is not None, "register_memory failed"

    def descs(n):
        chunk = TOTAL // n
        return agent.get_xfer_descs(
            [(base + i * chunk, chunk, dev_id) for i in range(n)], mem_type
        )

    if args.role == "target":
        srv = socket.socket()
        srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        srv.bind(("0.0.0.0", args.port))
        srv.listen(1)
        print("target: waiting for initiator", flush=True)
        conn, _ = srv.accept()
        send_msg(conn, agent.get_agent_metadata())
        for n in desc_counts:
            send_msg(conn, agent.get_serialized_descs(descs(n)))
        for _ in desc_counts:
            seen = 0
            while seen < args.iters:
                for msgs in agent.get_new_notifs().values():
                    seen += len(msgs)
            send_msg(conn, b"ok")
        recv_msg(conn)
        if args.op == "WRITE":
            assert float(buf[0]) == 7.0 and float(buf[-1]) == 7.0, "data mismatch"
        print("target: done", flush=True)
        conn.close()
        return

    sock = socket.socket()
    for _ in range(30):
        try:
            sock.connect((args.peer, args.port))
            break
        except OSError:
            time.sleep(1)
    else:
        raise SystemExit("cannot reach target")

    peer = agent.add_remote_agent(recv_msg(sock)).decode()
    remote = {n: agent.deserialize_descs(recv_msg(sock)) for n in desc_counts}

    print(
        f"{'descs':>6}  {'GiB/s':>7}  {'GiB/s(bar)':>11}  "
        f"({args.op}, {args.mem}, {args.iters} iters)",
        flush=True,
    )
    for n in desc_counts:
        local = descs(n)
        best = 0.0
        loop_start = time.perf_counter()
        for _ in range(args.iters):
            handle = agent.initialize_xfer(
                args.op, local, remote[n], peer, b"done"
            )
            start = time.perf_counter()
            state = agent.transfer(handle)
            while state != "DONE":
                state = agent.check_xfer_state(handle)
                if state == "ERR":
                    raise SystemExit(f"transfer failed with {n} descs")
            if device == "cuda":
                torch.cuda.synchronize()
            elapsed = time.perf_counter() - start
            agent.release_xfer_handle(handle)
            best = max(best, TOTAL / elapsed / 2**30)
        # A WRITE is reported DONE once the local side is drained, so per-op
        # numbers can exceed the wire rate. NIXL delivers a notification to
        # the target only after the data has landed, so wait for the target to
        # confirm it saw one notification per iteration. (A plain TCP round
        # trip would prove nothing: the socket is not ordered against RDMA.)
        recv_msg(sock)
        agg = TOTAL * args.iters / (time.perf_counter() - loop_start) / 2**30
        if args.op == "READ":
            assert float(buf[0]) == 7.0 and float(buf[-1]) == 7.0, "data mismatch"
        print(f"{n:>6}  {best:>7.2f}  {agg:>11.2f}", flush=True)

    send_msg(sock, b"done")
    sock.close()


if __name__ == "__main__":
    main()
