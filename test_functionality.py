#!/usr/bin/env python3
"""
Basic functionality tests for the proxy.
Requires `requests` installed.
"""

import http.server
import socket
import threading
import time
from contextlib import contextmanager

import requests

from pygofastproxy import run_proxy


def _find_free_port() -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class _BackendHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"message": "hello"}')

    def log_message(self, *args, **kwargs):
        return


@contextmanager
def start_backend():
    port = _find_free_port()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), _BackendHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield port
    finally:
        server.shutdown()
        thread.join(timeout=2)


@contextmanager
def start_proxy(target: str, port: int, **kwargs):
    proc = run_proxy(target=target, port=port, **kwargs)
    try:
        time.sleep(1)  # Wait for proxy to start
        yield proc
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except Exception:
            pass


def test_basic_proxy():
    with start_backend() as backend_port:
        proxy_port = _find_free_port()
        with start_proxy(
            target=f"http://127.0.0.1:{backend_port}",
            port=proxy_port,
        ):
            base = f"http://127.0.0.1:{proxy_port}"

            # Test basic GET
            res = requests.get(f"{base}/test", timeout=2)
            assert res.status_code == 200
            assert res.json().get("message") == "hello"

            # Test health endpoint
            health = requests.get(f"{base}/health", timeout=2)
            assert health.status_code == 200
            
            # Test security headers
            assert "X-Content-Type-Options" in res.headers
            assert "X-Frame-Options" in res.headers

    print("✓ Basic proxy test passed")


if __name__ == "__main__":
    test_basic_proxy()
    print("All tests completed successfully.")
