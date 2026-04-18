# pygofastproxy

[![CI](https://github.com/26zl/pygofastproxy/actions/workflows/ci.yml/badge.svg)](https://github.com/26zl/pygofastproxy/actions/workflows/ci.yml)
[![Security](https://github.com/26zl/pygofastproxy/actions/workflows/security.yml/badge.svg)](https://github.com/26zl/pygofastproxy/actions/workflows/security.yml)
[![PyPI](https://img.shields.io/pypi/v/pygofastproxy)](https://pypi.org/project/pygofastproxy/)
[![Downloads](https://img.shields.io/pypi/dm/pygofastproxy)](https://pypi.org/project/pygofastproxy/)
[![Python](https://img.shields.io/pypi/pyversions/pygofastproxy)](https://pypi.org/project/pygofastproxy/)
[![License](https://img.shields.io/github/license/26zl/pygofastproxy)](LICENSE)

A high-performance HTTP reverse proxy for Python, powered by Go's [fasthttp](https://github.com/valyala/fasthttp). Three lines of Python to put a Go-speed proxy with built-in security in front of your app.

```python
from pygofastproxy import run_proxy

run_proxy(target="http://localhost:4000", port=8080)
```

```
┌────────┐       ┌──────────────────────┐       ┌──────────┐
│        │       │    pygofastproxy     │       │          │
│ Client │──────▶│    (Go binary)       │──────▶│ Backend  │
│        │◀──────│                      │◀──────│ (Flask,  │
└────────┘       │  + Security headers  │       │  Django, │
                 │  + Per-IP rate limit  │       │  FastAPI)│
                 │  + CORS              │       └──────────┘
                 │  + TLS termination   │
                 └──────────────────────┘
```


## Why pygofastproxy?

Python web frameworks are great for building apps, but they weren't designed to be internet-facing reverse proxies. The typical solution is deploying Nginx or Caddy alongside your app — but that means extra infrastructure, config files, and moving parts.

pygofastproxy gives you Go-level proxy performance from a `pip install`:

| | pygofastproxy | proxy.py | Nginx | mitmproxy |
|---|:---:|:---:|:---:|:---:|
| `pip install` + 3 lines of code | **Yes** | No (plugin system) | No (separate install) | No (inspection tool) |
| Performance | Go (fasthttp) | Python | C | Python |
| Security headers by default | **Yes** | No | No (manual config) | No |
| Per-IP rate limiting built-in | **Yes** | No | Module | No |
| CORS handling built-in | **Yes** | No | No | No |
| TLS termination | **Yes** | Yes | Yes | Yes |
| Zero Python dependencies | **Yes** | Yes | N/A | No |

**Best for:** Python developers who want a fast, secure reverse proxy without deploying separate infrastructure.

**Not designed for:** Traffic inspection/debugging (use mitmproxy), load balancing across multiple backends (use Nginx/Traefik), or HTTP/2/WebSocket proxying (fasthttp limitation).


## Quick Start

1. **Install:**

   ```bash
   pip install pygofastproxy
   ```

2. **Start your backend** (e.g., Flask on port 4000)

3. **Run the proxy:**

   ```python
   from pygofastproxy import run_proxy

   run_proxy(target="http://localhost:4000", port=8080)
   ```

4. **Send requests** to `http://localhost:8080` — they'll be proxied to your backend with security headers, rate limiting, and CORS handling added automatically.

**Requirements:** Python 3.10+ and [Go](https://golang.org/dl/) (for building the proxy binary on first run). The binary is cached and only rebuilt when source changes.


## Configuration

### Python API

```python
run_proxy(
    target="http://localhost:4000",
    port=8080,
    rate_limit_rps=5000,
    allowed_origins="https://yourdomain.com,https://app.yourdomain.com",
    tls_cert_file="/path/to/cert.pem",
    tls_key_file="/path/to/key.pem",
)
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `target` | str | `"http://localhost:4000"` | Backend URL to proxy to |
| `port` | int | `8080` | Proxy listen port |
| `rate_limit_rps` | int | `1000` | Per-IP requests/sec (0 = unlimited) |
| `max_conns_per_host` | int | `1000` | Max concurrent backend connections |
| `read_timeout` | str | `"10s"` | Read timeout (e.g., `"30s"`, `"1m"`) |
| `write_timeout` | str | `"10s"` | Write timeout |
| `max_idle_conn_duration` | str | `"60s"` | Idle connection TTL |
| `max_request_body_size` | int | `10485760` | Max request body in bytes (10MB) |
| `allowed_origins` | str | `None` | Comma-separated CORS origins |
| `cors_allow_credentials` | bool | `False` | Enable `Access-Control-Allow-Credentials` |
| `tls_cert_file` | str | `None` | TLS certificate path (enables HTTPS) |
| `tls_key_file` | str | `None` | TLS private key path |

### Environment Variables

All settings can also be configured via environment variables (useful for Docker and CLI usage):

```bash
PY_BACKEND_TARGET=http://localhost:4000
PY_BACKEND_PORT=8080
PROXY_RATE_LIMIT_RPS=5000
PROXY_MAX_CONNS_PER_HOST=2000
PROXY_MAX_CONN_WAIT_TIMEOUT=5s
PROXY_READ_TIMEOUT=30s
PROXY_WRITE_TIMEOUT=30s
PROXY_MAX_IDLE_CONN_DURATION=60s
PROXY_READ_BUFFER_SIZE=16384
PROXY_WRITE_BUFFER_SIZE=16384
PROXY_MAX_REQUEST_BODY_SIZE=20971520
ALLOWED_ORIGINS=https://yourdomain.com
PROXY_CORS_ALLOW_CREDENTIALS=false
PROXY_TLS_CERT_FILE=/path/to/cert.pem
PROXY_TLS_KEY_FILE=/path/to/key.pem
```

Run via CLI: `python -m pygofastproxy`


## Security Features

All security features are enabled by default with zero configuration:

- **Per-IP rate limiting** — Token bucket algorithm, 1000 req/s per IP by default. Stale buckets are cleaned up automatically.
- **Security headers** — Every response gets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `X-XSS-Protection: 1; mode=block`. `Cache-Control: no-store` is added only if the backend didn't set its own.
- **Request size limits** — Rejects bodies over 10MB by default (configurable).
- **Hop-by-hop header stripping** — Removes `Connection`, `Proxy-Authorization`, `Transfer-Encoding`, and any additional headers listed in the `Connection` header value (RFC 7230).
- **X-Forwarded headers** — Always overwrites `X-Forwarded-For`, `X-Forwarded-Proto`, and `X-Forwarded-Host` to prevent client spoofing.
- **CORS protection** — When `allowed_origins` is set, only listed origins get CORS headers. The `null` origin is always rejected. `Access-Control-Allow-Credentials` is off by default.
- **No information leakage** — Internal backend URLs and error details are never exposed to clients.
- **TLS termination** — Optional HTTPS support via cert/key configuration.
- **Private target warnings** — Logs a warning if the backend target is a private/loopback address.
- **Connection backpressure** — `MaxConnWaitTimeout` prevents goroutine accumulation when the backend is slow.


## Benchmarks

Run the included benchmark to measure throughput on your hardware:

```bash
pip install pygofastproxy
python benchmarks/benchmark.py
```

You can also use external tools for more rigorous testing:

```bash
# Using hey (https://github.com/rakyll/hey)
hey -n 10000 -c 50 http://localhost:8080/

# Using wrk (https://github.com/wg/wrk)
wrk -t4 -c50 -d10s http://localhost:8080/
```

The proxy uses Go's fasthttp library, which is designed for [high throughput with minimal allocations](https://github.com/valyala/fasthttp#http-server-performance-comparison-with-nethttp). All header byte slices are pre-allocated, the rate limiter uses per-bucket locks instead of a global mutex, and CORS origins are refreshed on a timer rather than per-request.


## Docker

```bash
docker compose up --build
```

The Dockerfile uses a multi-stage build (Go builder + slim Python runtime) and runs as a non-root user. Configure via environment variables in `.env` or `docker-compose.yml`.


## Testing

```bash
pip install -e ".[test]"
pytest tests/ -v
```

The test suite covers proxy forwarding, HTTP methods, security headers, CORS, rate limiting, health endpoint, and configuration validation.


## Architecture

pygofastproxy is a hybrid Python+Go project:

- **Python layer** (`pygofastproxy/`) — Package API, CLI, Go binary lifecycle management. Python never touches HTTP traffic.
- **Go layer** (`pygofastproxy/go/`) — The actual proxy, built on [fasthttp](https://github.com/valyala/fasthttp). Handles all request forwarding, security, rate limiting, and CORS.

The Go binary is compiled automatically on first use via `runner.py` and cached. It's only rebuilt when `.go`, `go.mod`, or `go.sum` files change. Go is only required at build time — if the binary already exists (e.g., in Docker), Go is not needed.


## Roadmap

- **Pre-compiled wheels** — Ship platform-specific wheels with the Go binary already built, removing the Go requirement entirely. Pure `pip install` with no build step. ([Track progress](https://github.com/26zl/pygofastproxy/issues))


## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on reporting bugs, suggesting features, and submitting pull requests.


## License

[MIT](LICENSE) — Copyright 2026 Laurent Zogaj

Powered by [fasthttp](https://github.com/valyala/fasthttp) by [valyala](https://github.com/valyala).
