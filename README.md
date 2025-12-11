# pygofastproxy

A simple, fast, and secure HTTP reverse proxy for Python, powered by Go's [fasthttp](https://github.com/valyala/fasthttp) library.

---

## Quick Start

1. **Install the package:**

   ```bash
   pip install pygofastproxy
   ```

2. **Start your backend server** (e.g., Flask) on port 4000.

3. **Run the proxy:**

   ```python
   from pygofastproxy import run_proxy

   # Basic usage
   run_proxy(target="http://localhost:4000", port=8080)
   ```

4. **Send requests** to `http://localhost:8080`.

---

## Overview

pygofastproxy is a **reverse proxy** that sits in front of your Python web application (Flask, FastAPI, Django, etc.) to provide:

- **Blazing-fast performance** using Go's fasthttp library
- **Built-in security** with automatic security headers and request size limits
- **CORS handling** for frontend applications
- **Rate limiting** to protect your backend from overload
- **Simple setup** with zero configuration required

### What is a Reverse Proxy?

```
Client → pygofastproxy:8080 → Your Backend:4000
```

The proxy receives all client requests, adds security protections, and forwards them to your backend server.

---

## Features

- ✅ **Ultra-fast proxying** with Go's fasthttp library
- ✅ **Simple Python API** - one function to start
- ✅ **Automatic security headers** (X-Frame-Options, X-XSS-Protection, etc.)
- ✅ **Request size limits** to prevent memory exhaustion attacks
- ✅ **CORS support** with configurable allowed origins
- ✅ **Rate limiting** to prevent backend overload
- ✅ **Input validation** to prevent crashes
- ✅ **Connection pooling** for optimal performance
- ✅ **Auto-build** of Go binary if not present

---

## Installation

Install from PyPI:

```bash
pip install pygofastproxy
```

Or for local development:

```bash
pip install /path/to/pygofastproxy
```

**Requirements:**
- Python 3.8+
- Go (for building the proxy binary)

---

## Usage

### Basic Example

```python
from pygofastproxy import run_proxy

# Start the proxy (forwards :8080 to your backend at :4000)
run_proxy(target="http://localhost:4000", port=8080)
```

### Production Configuration

```python
from pygofastproxy import run_proxy

run_proxy(
    target="http://localhost:4000",
    port=8080,
    max_conns_per_host=2000,        # Handle more concurrent connections
    read_timeout="30s",             # Connection read timeout
    write_timeout="30s",            # Connection write timeout
    rate_limit_rps=5000,            # Allow 5000 requests per second
    max_request_body_size=20971520, # 20MB request size limit
    allowed_origins="https://yourdomain.com,https://app.yourdomain.com"
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

---

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

---

## Security Features

### 1. Request Size Limits
Prevents memory exhaustion attacks by limiting request body size (default: 10MB).

### 2. Security Headers
Automatically adds:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Cache-Control: no-store`

### 3. Input Validation
Validates all URLs and ports to prevent crashes and misconfigurations.

### 4. Rate Limiting
Token bucket rate limiting prevents backend overload.

### 5. CORS Protection
When `allowed_origins` is set, only requests from those origins are permitted.

---

## Performance

pygofastproxy is built for speed:

- **Zero-copy operations** where possible
- **Connection pooling** for backend requests
- **Optimized header handling** with byte-slice operations
- **Minimal allocations** using fasthttp's optimizations

**Benchmark results** (compared to direct backend access):
- Latency overhead: ~1-2ms
- Throughput: 50,000+ req/s on modern hardware
- Memory: Minimal overhead with connection pooling

---

## Use Cases

### Development Proxy
```python
# Simple dev setup
run_proxy(target="http://localhost:4000", port=8080)
```
Perfect for running Next.js frontend → Python backend with automatic CORS.

### Production API Gateway
```python
# Production setup with security
run_proxy(
    target="http://localhost:4000",
    port=8080,
    rate_limit_rps=5000,
    allowed_origins="https://yourdomain.com"
)
```
Add a fast, secure layer in front of your Python API.

### Microservices Proxy
```python
# Route to different backends
import os

service = os.getenv("SERVICE_NAME", "api")
target_map = {
    "api": "http://localhost:4000",
    "auth": "http://localhost:4001",
    "data": "http://localhost:4002",
}

run_proxy(target=target_map[service], port=8080)
```

---

## Example: Flask + Next.js

**Backend (Flask):**
```python
# app.py
from flask import Flask

app = Flask(__name__)

@app.route('/api/hello')
def hello():
    return {"message": "Hello from Flask!"}

if __name__ == '__main__':
    app.run(port=4000)
```

**Proxy:**
```python
# proxy.py
from pygofastproxy import run_proxy

run_proxy(
    target="http://localhost:4000",
    port=8080,
    allowed_origins="http://localhost:3000"  # Next.js dev server
)
```

**Frontend (Next.js):**
```javascript
// pages/index.js
export default function Home() {
  const [data, setData] = useState(null);

  useEffect(() => {
    fetch('http://localhost:8080/api/hello')
      .then(res => res.json())
      .then(data => setData(data));
  }, []);

  return <div>{data?.message}</div>;
}
```

---

## Testing

Run the included test:

```bash
python test_functionality.py
```

Or test manually:

1. Start a backend server: `python3 -m http.server 4000`
2. Start the proxy: `python -c "from pygofastproxy import run_proxy; run_proxy()"`
3. Test it: `curl http://localhost:8080`

---

## Docker Example

**Dockerfile:**
```dockerfile
FROM python:3.11-slim

# Install Go
RUN apt-get update && apt-get install -y golang-go

WORKDIR /app

COPY requirements.txt .
RUN pip install -r requirements.txt

COPY . .

CMD ["python", "proxy.py"]
```

**docker-compose.yml:**
```yaml
version: '3.8'
services:
  proxy:
    build: .
    ports:
      - "8080:8080"
    environment:
      - PY_BACKEND_TARGET=http://backend:4000
      - PROXY_RATE_LIMIT_RPS=5000
    depends_on:
      - backend

  backend:
    build: ./backend
    ports:
      - "4000:4000"
```

---

## Troubleshooting

### "Go is not installed or not found in PATH"
Install Go from [golang.org/dl](https://golang.org/dl/)

### High latency
- Increase `max_conns_per_host` for high-traffic scenarios
- Reduce `read_timeout` and `write_timeout` if appropriate

### Rate limiting triggered
- Increase `rate_limit_rps` or set to `0` for unlimited
- Implement client-side backoff/retry logic

### CORS errors
- Set `allowed_origins` to include your frontend domain
- Check browser console for specific origin being blocked

---

## Contributing

Contributions are welcome! Please open issues or submit pull requests for bug fixes, improvements, or new features.

---

## License

This project is licensed under the [MIT License](LICENSE).

---

## Credits

Powered by [fasthttp](https://github.com/valyala/fasthttp) by [valyala](https://github.com/valyala). Thanks to the fasthttp contributors for creating the fastest HTTP library for Go.
