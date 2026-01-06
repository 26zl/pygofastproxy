# pygofastproxy

A simple, fast, and secure HTTP reverse proxy for Python, powered by Go's [fasthttp](https://github.com/valyala/fasthttp) library.


## Quick Start

1. **Install the package:**

   ```bash
   pip install pygofastproxy
   ```

2. **Start your backend server** (e.g., Flask) on port 4000.

3. **Run the proxy:**

   ```python
   from pygofastproxy import run_proxy

   run_proxy(target="http://localhost:4000", port=8080)
   ```

4. **Send requests** to `http://localhost:8080`.


## What is pygofastproxy?

pygofastproxy is a **reverse proxy** that sits in front of your Python web application to provide:

- Fast performance using Go's fasthttp library
- Built-in security with automatic security headers
- CORS handling for frontend applications
- Rate limiting to protect your backend
- Simple setup with zero configuration required

```
Client → pygofastproxy:8080 → Your Backend:4000
```

The proxy receives all client requests, adds security protections, and forwards them to your backend server.


## Installation

Install from PyPI:

```bash
pip install pygofastproxy
```

**Requirements:**
- Python 3.8+
- Go (for building the proxy binary)


## Usage

### Basic Example

```python
from pygofastproxy import run_proxy

# Start the proxy (forwards :8080 to your backend at :4000)
run_proxy(target="http://localhost:4000", port=8080)
```

### With Configuration

```python
from pygofastproxy import run_proxy

run_proxy(
    target="http://localhost:4000",
    port=8080,
    rate_limit_rps=5000,
    allowed_origins="https://yourdomain.com"
)
```

### Configuration Options

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `target` | str | `"http://localhost:4000"` | Backend server URL to proxy to |
| `port` | int | `8080` | Port for proxy to listen on |
| `max_conns_per_host` | int | `1000` | Maximum concurrent connections per host |
| `read_timeout` | str | `"10s"` | Read timeout (e.g., "10s", "1m") |
| `write_timeout` | str | `"10s"` | Write timeout (e.g., "10s", "1m") |
| `rate_limit_rps` | int | `1000` | Requests per second limit (0 = unlimited) |
| `max_request_body_size` | int | `10485760` | Max request body size in bytes (10MB default) |
| `allowed_origins` | str | `None` | Comma-separated CORS origins |


## Environment Variables

You can also configure the proxy using environment variables:

```bash
PY_BACKEND_TARGET=http://localhost:4000
PY_BACKEND_PORT=8080
PROXY_MAX_CONNS_PER_HOST=2000
PROXY_READ_TIMEOUT=30s
PROXY_WRITE_TIMEOUT=30s
PROXY_RATE_LIMIT_RPS=5000
PROXY_MAX_REQUEST_BODY_SIZE=20971520
ALLOWED_ORIGINS=https://yourdomain.com
```


## Security Features

The proxy automatically adds security headers and protections:

- **Request size limits** - Prevents memory exhaustion attacks (default: 10MB)
- **Security headers** - Adds X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Cache-Control
- **Input validation** - Validates all URLs and ports
- **Rate limiting** - Token bucket rate limiting to prevent backend overload
- **CORS protection** - When `allowed_origins` is set, only requests from those origins are permitted


## Docker

Use the included `Dockerfile` and `docker-compose.yml`:

```bash
docker compose up --build
```

The compose file builds the proxy. Make sure your backend server is accessible at the URL specified in `PY_BACKEND_TARGET` (default: `http://host.docker.internal:4000` for accessing host services from Docker).


## Testing

Run the included test:

```bash
python test_functionality.py
```

Or test manually:

1. Start a backend server: `python3 -m http.server 4000`
2. Start the proxy: `python -m pygofastproxy`
3. Test it: `curl http://localhost:8080`


## Troubleshooting

### "Go is not installed or not found in PATH"
Install Go from [golang.org/dl](https://golang.org/dl/)

### Rate limiting triggered
Increase `rate_limit_rps` or set to `0` for unlimited

### CORS errors
Set `allowed_origins` to include your frontend domain


## License

This project is licensed under the [MIT License](LICENSE).


## Credits

Powered by [fasthttp](https://github.com/valyala/fasthttp) by [valyala](https://github.com/valyala).
