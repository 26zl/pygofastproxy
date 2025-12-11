#!/usr/bin/env python3
"""
Extended smoke tests for the proxy. Requires `requests` installed.
Spins up a local backend and exercises:
- Health endpoint
- Basic GET/POST proxying
- Security headers
- CORS allow list
- Body size limit
- Rate limiting enabled/disabled
"""

import http.server
import json
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
    def _send_json(self, payload, status=200):
        data = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):  # noqa: N802
        if self.path == "/hello":
            self._send_json({"message": "hello"})
        else:
            self._send_json({"path": self.path}, status=200)

    def do_POST(self):  # noqa: N802
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length) if length else b""
        try:
            payload = json.loads(body.decode() or "{}")
        except Exception:
            payload = {"raw": body.decode(errors="ignore")}
        self._send_json({"received": payload})

    def log_message(self, *args, **kwargs):  # silence
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
        yield proc
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except Exception:
            pass


def _wait_for_health(port: int, timeout: float = 5.0):
    deadline = time.time() + timeout
    url = f"http://127.0.0.1:{port}/health"
    while time.time() < deadline:
        try:
            res = requests.get(url, timeout=0.5)
            if res.status_code == 200:
                return
        except Exception:
            pass
        time.sleep(0.1)
    raise RuntimeError("Proxy health endpoint did not become ready in time")


def test_proxy_features():
    with start_backend() as backend_port:
        proxy_port = _find_free_port()
        allowed_origin = "http://example.local"
        with start_proxy(
            target=f"http://127.0.0.1:{backend_port}",
            port=proxy_port,
            rate_limit_rps=2,
            max_request_body_size=1024,
            allowed_origins=allowed_origin,
        ):
            _wait_for_health(proxy_port)

            base = f"http://127.0.0.1:{proxy_port}"

            # Basic GET
            res = requests.get(f"{base}/hello", timeout=2)
            assert res.status_code == 200, f"GET /hello failed: {res.status_code}"
            assert res.json().get("message") == "hello"

            # Basic POST echo
            payload = {"test": "data"}
            res = requests.post(f"{base}/echo", json=payload, timeout=2)
            assert res.status_code == 200, f"POST /echo failed: {res.status_code}"
            assert res.json().get("received") == payload

            # Security headers
            for header in (
                "X-Content-Type-Options",
                "X-Frame-Options",
                "X-XSS-Protection",
                "Cache-Control",
                "X-Proxy-Server",
            ):
                assert header in res.headers, f"Missing security header {header}"

            # allow tokens to refill before next checks
            time.sleep(1)

            # CORS allow list
            cors_res = requests.get(f"{base}/hello", headers={"Origin": allowed_origin}, timeout=2)
            assert cors_res.headers.get("Access-Control-Allow-Origin") == allowed_origin

            # Body size limit (set to 1KB above). Send 2KB.
            big_body = "x" * 2048
            too_big = requests.post(f"{base}/echo", data=big_body, timeout=2)
            assert too_big.status_code >= 400, f"Expected rejection for large body, got {too_big.status_code}"

            # Rate limiting should trigger with many rapid requests
            over_limit = 0
            for _ in range(20):
                r = requests.get(f"{base}/hello", timeout=1)
                if r.status_code == 429:
                    over_limit += 1
            assert over_limit > 0, "Rate limiting did not trigger as expected"

    print("✓ Feature test passed")


def test_rate_limit_disabled():
    with start_backend() as backend_port:
        proxy_port = _find_free_port()
        with start_proxy(
            target=f"http://127.0.0.1:{backend_port}",
            port=proxy_port,
            rate_limit_rps=0,  # disabled
        ):
            _wait_for_health(proxy_port)
            base = f"http://127.0.0.1:{proxy_port}"
            codes = []
            for _ in range(20):
                res = requests.get(f"{base}/hello", timeout=1)
                codes.append(res.status_code)
            assert 429 not in codes, "Rate limiting should be disabled but 429s were observed"
    print("✓ Rate limit disabled test passed")


if __name__ == "__main__":
    test_proxy_features()
    test_rate_limit_disabled()
    print("All tests completed successfully.")
