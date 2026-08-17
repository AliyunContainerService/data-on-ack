"""Cross-node NIXL transfer over eRDMA, backend selectable (UCX or Mooncake).

Rank 0 (target) allocates a buffer, registers it with NIXL and publishes its
agent metadata over a plain TCP socket. Rank 1 (initiator) registers its own
buffer and issues a NIXL READ, then verifies the contents.

    python nixl_check.py --role target    --backend Mooncake --mem dram
    python nixl_check.py --role initiator --backend Mooncake --mem dram \
        --peer <target-ip>
"""

import argparse
import socket
import time

import torch
from nixl._api import nixl_agent, nixl_agent_config

SIZE = 256 * 1024 * 1024  # 256 MiB


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
    ap.add_argument("--mem", choices=["dram", "vram"], default="vram")
    ap.add_argument("--peer", default="")
    ap.add_argument("--port", type=int, default=19999)
    args = ap.parse_args()

    agent = nixl_agent(args.role, nixl_agent_config(backends=[args.backend]))

    device = "cuda" if args.mem == "vram" else "cpu"
    fill = 7.0 if args.role == "target" else 0.0
    buf = torch.full((SIZE // 4,), fill, dtype=torch.float32, device=device)
    if device == "cuda":
        torch.cuda.synchronize()
    reg = agent.register_memory([buf])
    assert reg is not None, "register_memory failed"

    if args.role == "target":
        srv = socket.socket()
        srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        srv.bind(("0.0.0.0", args.port))
        srv.listen(1)
        print("target: waiting for initiator", flush=True)
        conn, _ = srv.accept()
        send_msg(conn, agent.get_agent_metadata())
        send_msg(conn, agent.get_serialized_descs(reg.trim()))
        recv_msg(conn)
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
    remote_descs = agent.deserialize_descs(recv_msg(sock))
    local_descs = reg.trim()

    for i in range(3):
        handle = agent.initialize_xfer("READ", local_descs, remote_descs, peer)
        start = time.perf_counter()
        state = agent.transfer(handle)
        while state != "DONE":
            state = agent.check_xfer_state(handle)
            if state == "ERR":
                raise SystemExit("transfer failed")
        if device == "cuda":
            torch.cuda.synchronize()
        elapsed = time.perf_counter() - start
        agent.release_xfer_handle(handle)
        print(
            f"iter {i}: READ {SIZE / 2**20:.0f} MiB ({args.mem}) in "
            f"{elapsed * 1000:.1f} ms ({SIZE / elapsed / 2**30:.2f} GiB/s), "
            f"first={float(buf[0])}",
            flush=True,
        )

    assert float(buf[0]) == 7.0 and float(buf[-1]) == 7.0, "data mismatch"
    send_msg(sock, b"done")
    sock.close()
    print(f"NIXL/{args.backend} over eRDMA: OK", flush=True)


if __name__ == "__main__":
    main()
