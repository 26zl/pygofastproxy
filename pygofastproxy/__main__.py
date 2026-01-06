"""CLI entry point for pygofastproxy."""
import os
import sys
from .runner import run_proxy


def main():
    """Main entry point for pygofastproxy CLI."""
    target = os.getenv("PY_BACKEND_TARGET", "http://localhost:4000")
    port = int(os.getenv("PY_BACKEND_PORT", "8080"))

    # Parse additional configuration from environment
    kwargs = {}

    if max_conns := os.getenv("PROXY_MAX_CONNS_PER_HOST"):
        kwargs["max_conns_per_host"] = int(max_conns)
    if read_timeout := os.getenv("PROXY_READ_TIMEOUT"):
        kwargs["read_timeout"] = read_timeout
    if write_timeout := os.getenv("PROXY_WRITE_TIMEOUT"):
        kwargs["write_timeout"] = write_timeout
    if rate_limit := os.getenv("PROXY_RATE_LIMIT_RPS"):
        kwargs["rate_limit_rps"] = int(rate_limit)
    if max_body := os.getenv("PROXY_MAX_REQUEST_BODY_SIZE"):
        kwargs["max_request_body_size"] = int(max_body)
    if origins := os.getenv("ALLOWED_ORIGINS"):
        kwargs["allowed_origins"] = origins
    
    proc = None
    try:
        proc = run_proxy(target=target, port=port, **kwargs)
        # Wait for the process to complete
        proc.wait()
    except KeyboardInterrupt:
        print("\nShutting down proxy...")
        if proc:
            proc.terminate()
            proc.wait()
        sys.exit(0)
    except Exception as e:
        print(f"Error starting proxy: {e}", file=sys.stderr)
        if proc:
            proc.terminate()
        sys.exit(1)


if __name__ == "__main__":
    main()
