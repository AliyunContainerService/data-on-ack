import atexit
import os
import shutil
import socket
import subprocess
import tempfile
import time
import unittest
import uuid
from urllib.request import urlopen

_process = None
_endpoint = None


def configure_storage():
    global _process, _endpoint
    if _process is None:
        executable = shutil.which("etcd")
        if not executable:
            raise unittest.SkipTest("etcd executable required for integration tests")
        ports = []
        for _ in range(2):
            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                ports.append(sock.getsockname()[1])
        _endpoint = f"http://127.0.0.1:{ports[0]}"
        peer = f"http://127.0.0.1:{ports[1]}"
        directory = tempfile.mkdtemp(prefix="admission-etcd-test-")
        _process = subprocess.Popen(
            [
                executable,
                "--data-dir",
                directory,
                "--listen-client-urls",
                _endpoint,
                "--advertise-client-urls",
                _endpoint,
                "--listen-peer-urls",
                peer,
                "--initial-advertise-peer-urls",
                peer,
                "--initial-cluster",
                "default=" + peer,
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

        def cleanup():
            _process.terminate()
            _process.wait(timeout=10)
            shutil.rmtree(directory)

        atexit.register(cleanup)
        deadline = time.monotonic() + 10
        while True:
            try:
                with urlopen(_endpoint + "/health", timeout=1):
                    break
            except OSError:
                if time.monotonic() > deadline or _process.poll() is not None:
                    raise RuntimeError("Test etcd failed to start")
                time.sleep(0.05)
    os.environ["RAY_ADMISSION_ETCD_ENDPOINT"] = _endpoint
    os.environ["RAY_ADMISSION_ETCD_PREFIX"] = "/test/" + uuid.uuid4().hex
