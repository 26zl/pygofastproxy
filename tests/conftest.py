import http.server
import socket
import threading
import time

import pytest

from pygofastproxy import run_proxy


def _find_free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class _BackendHandler(http.server.BaseHTTPRequestHandler):
    """Test backend that echoes request info back as JSON."""

    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"method":"GET","path":"' + self.path.encode() + b'"}')

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(
            b'{"method":"POST","body_size":' + str(len(body)).encode() + b"}"
        )

    def do_PUT(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(
            b'{"method":"PUT","body_size":' + str(len(body)).encode() + b"}"
        )

    def do_DELETE(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"method":"DELETE"}')

    def log_message(self, *args, **kwargs):
        return


class _CachingBackendHandler(_BackendHandler):
    """Backend that sets its own Cache-Control header."""

    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Cache-Control", "max-age=3600")
        self.end_headers()
        self.wfile.write(b'{"cached":true}')


def _start_server(handler_class):
    port = _find_free_port()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", port), handler_class)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    return server, port


def _start_proxy(target, **kwargs):
    port = _find_free_port()
    proc = run_proxy(target=target, port=port, **kwargs)
    time.sleep(1.5)
    return proc, port


@pytest.fixture(scope="session")
def backend():
    """Session-scoped test backend."""
    server, port = _start_server(_BackendHandler)
    yield f"http://127.0.0.1:{port}"
    server.shutdown()


@pytest.fixture(scope="session")
def caching_backend():
    """Session-scoped backend that sets Cache-Control."""
    server, port = _start_server(_CachingBackendHandler)
    yield f"http://127.0.0.1:{port}"
    server.shutdown()


@pytest.fixture(scope="session")
def proxy_url(backend):
    """Session-scoped default proxy."""
    proc, port = _start_proxy(backend)
    yield f"http://127.0.0.1:{port}"
    proc.terminate()
    proc.wait(timeout=5)


@pytest.fixture(scope="class")
def cors_proxy(backend):
    """Class-scoped proxy with CORS configured."""
    proc, port = _start_proxy(
        backend, allowed_origins="http://allowed.example.com,http://other.example.com"
    )
    yield f"http://127.0.0.1:{port}"
    proc.terminate()
    proc.wait(timeout=5)


@pytest.fixture(scope="class")
def rate_limited_proxy(backend):
    """Class-scoped proxy with very low rate limit for testing."""
    proc, port = _start_proxy(backend, rate_limit_rps=3)
    yield f"http://127.0.0.1:{port}"
    proc.terminate()
    proc.wait(timeout=5)


@pytest.fixture(scope="class")
def caching_proxy(caching_backend):
    """Class-scoped proxy pointing to a backend that sets Cache-Control."""
    proc, port = _start_proxy(caching_backend)
    yield f"http://127.0.0.1:{port}"
    proc.terminate()
    proc.wait(timeout=5)
