#!/usr/bin/env python3
"""
Simple benchmark for pygofastproxy.

Starts a test backend and proxy, then measures throughput using
concurrent requests. No external dependencies beyond the standard library.

Usage:
    python benchmarks/benchmark.py
    python benchmarks/benchmark.py --requests 20000 --concurrency 100
"""

import argparse
import http.server
import socket
import threading
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

from pygofastproxy import run_proxy


class _BenchHandler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"ok":true}')

    def log_message(self, *a, **k):
        return


class _BenchServer(http.server.ThreadingHTTPServer):
    request_queue_size = 256
    allow_reuse_address = True


def _find_free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def run_benchmark(total_requests: int, concurrency: int):
    # Start backend
    backend_port = _find_free_port()
    server = _BenchServer(("127.0.0.1", backend_port), _BenchHandler)
    threading.Thread(target=server.serve_forever, daemon=True).start()

    # Start proxy (rate limiting disabled for pure throughput test)
    proxy_port = _find_free_port()
    proc = run_proxy(
        target=f"http://127.0.0.1:{backend_port}",
        port=proxy_port,
        rate_limit_rps=0,
    )
    time.sleep(1.5)

    url = f"http://127.0.0.1:{proxy_port}/bench"

    # Warmup
    print("Warming up with 200 requests...")
    for _ in range(200):
        try:
            urllib.request.urlopen(url, timeout=5)
        except Exception:
            pass

    # Benchmark
    print(f"Benchmarking: {total_requests} requests, {concurrency} concurrent...")
    successes = 0
    errors = 0

    def make_request():
        try:
            urllib.request.urlopen(url, timeout=10)
            return True
        except Exception:
            return False

    start = time.perf_counter()
    with ThreadPoolExecutor(max_workers=concurrency) as pool:
        futures = [pool.submit(make_request) for _ in range(total_requests)]
        for f in as_completed(futures):
            if f.result():
                successes += 1
            else:
                errors += 1
    elapsed = time.perf_counter() - start

    rps = successes / elapsed if elapsed > 0 else 0
    avg_latency = (elapsed / total_requests) * 1000 if total_requests > 0 else 0

    print()
    print("=" * 45)
    print("  pygofastproxy benchmark results")
    print("=" * 45)
    print(f"  Total requests:    {total_requests}")
    print(f"  Concurrency:       {concurrency}")
    print(f"  Successes:         {successes}")
    print(f"  Errors:            {errors}")
    print(f"  Total time:        {elapsed:.2f}s")
    print(f"  Requests/sec:      {rps:,.0f}")
    print(f"  Avg latency:       {avg_latency:.2f}ms")
    print("=" * 45)

    proc.terminate()
    proc.wait(timeout=5)
    server.shutdown()


def main():
    parser = argparse.ArgumentParser(description="Benchmark pygofastproxy")
    parser.add_argument(
        "-n",
        "--requests",
        type=int,
        default=10000,
        help="Total requests (default: 10000)",
    )
    parser.add_argument(
        "-c",
        "--concurrency",
        type=int,
        default=50,
        help="Concurrent workers (default: 50)",
    )
    args = parser.parse_args()
    run_benchmark(args.requests, args.concurrency)


if __name__ == "__main__":
    main()
