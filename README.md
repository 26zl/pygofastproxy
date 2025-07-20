# pygofastproxy

A blazing-fast HTTP proxy for Python, powered by Go’s [fasthttp](https://github.com/valyala/fasthttp) library.

---

> **Powered by [fasthttp](https://github.com/valyala/fasthttp)**

## Quick Start

1. **Install the package:**
   ```bash
   pip install pygofastproxy
   ```
2. **Start your backend server** (e.g., Flask) on port 4000.
3. **Run the proxy:**
   ```python
   from pygofastproxy.runner import run_proxy
   run_proxy(target="http://localhost:4000", port=8080)
   ```
4. **Send requests** to `http://localhost:8080`.

---

## How it Works

pygofastproxy launches a Go-based HTTP proxy as a subprocess from Python. The Go proxy listens on the specified port and forwards all HTTP requests to your backend server. Configuration is handled via Python arguments or environment variables.

---

## Overview

pygofastproxy is a Python package that provides a super-fast HTTP proxy, powered by Go, for use with Python web backends. It is ideal for scenarios where you want to:
- Add a high-performance reverse proxy in front of your Python (Flask, FastAPI, Django, etc.) backend.
- Integrate with frontend frameworks (like Next.js) that need to proxy API requests to a Python backend.
- Use as a development tool to forward requests, add logging, or simulate production-like proxying locally.
- Robust and Secure. 

## Features
- Fast HTTP proxying using Go's fasthttp library
- Simple Python API to launch and control the proxy
- Automatic Go binary build if not present
- Easily configurable target and port
- **Planned:** Pre-built binaries for easier installation (coming soon!)
- **More features coming soon!**

## Installation

You can install from PyPI:

```bash
pip install pygofastproxy
```

Or, for local development:

```bash
pip install /path/to/pygofastproxy
```

## Usage

### As a Python Module

```python
from pygofastproxy.runner import run_proxy

# Start the proxy (forwards :8080 to your backend at :4000)
run_proxy(target="http://localhost:4000", port=8080)
```

- By default, the proxy will listen on `localhost:8080` and forward to your backend at `localhost:4000`.
- You can adjust the `target` and `port` as needed.

---

## Environment Variables

You can control the proxy using these environment variables:
- `PY_BACKEND_TARGET`: The backend server URL to forward requests to (default: `http://localhost:4000`).
- `PY_BACKEND_PORT`: The port for the proxy to listen on (default: `8080`).

---

## Testing

To manually test the proxy:
1. Start a backend server on port 4000 (e.g., `python3 -m http.server 4000`).
2. Start the proxy as shown above.
3. In another terminal, run:
   ```bash
   curl http://localhost:8080
   ```
   You should see the response from your backend, confirming the proxy is working.

---

## Example: Use with Flask and Next.js (Dockerized)

Suppose you have a Flask backend and a Next.js frontend. You can use pygofastproxy as a reverse proxy between them:

- Frontend (Next.js) sends API requests to `localhost:8080`
- pygofastproxy forwards requests to Flask backend at `localhost:4000`

**docker-compose.yml** (simplified):

```yaml
version: '3.8'
services:
  proxy:
    build: ./proxy
    ports:
      - "8080:8080"
    depends_on:
      - backend

  backend:
    build: ./backend
    ports:
      - "4000:4000"

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://localhost:8080
```

## Use Cases

- **Development proxy**: Quickly forward requests from a frontend to a Python backend, simulating production proxying.
- **Performance**: Add a fast Go-based proxy in front of Python services for better throughput.
- **API Gateway**: Use as a lightweight API gateway for microservices.
- **Testing**: Intercept and forward requests for integration testing.

## Requirements
- Python 3.7+
- Go (for building the proxy binary)

## License

This project is licensed under the [MIT License](LICENSE).

---

## Contributing

Contributions are welcome! Please open issues or submit pull requests for bug fixes, improvements, or new features. For major changes, please open an issue first to discuss what you would like to change.

---

## Credits

- This project is powered by the amazing [fasthttp](https://github.com/valyala/fasthttp) library by [valyala](https://github.com/valyala). Huge thanks to the fasthttp contributors for their work on one of the fastest HTTP libraries for Go.

---