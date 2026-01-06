import os
import subprocess
import threading
from pathlib import Path
from urllib.parse import urlparse

# Check if running on Windows.
def _is_windows() -> bool:
    return os.name == "nt"

# Return executable name, appending .exe on Windows.
def _bin_name(base: str) -> str:
    return f"{base}.exe" if _is_windows() else base

# Check if Go is installed and available in PATH.
def check_go_installed():
    try:
        subprocess.run(
            ["go", "version"],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
    except Exception as e:
        raise RuntimeError("Go is not installed or not found in PATH.") from e

# Build the Go proxy binary from source.
def build_proxy(go_dir: Path, binary_path: Path):
    print("Building Go proxy ...")
    if not (go_dir / "go.mod").exists():
        raise FileNotFoundError(f"Missing go.mod in {go_dir}")

    cmd = [
        "go", "build",
        "-trimpath",
        "-ldflags", "-s -w",
        "-o", binary_path.name,
        ".",
    ]
    result = subprocess.run(
        cmd,
        cwd=go_dir,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True
    )
    if result.returncode != 0:
        raise RuntimeError(f"Failed to build Go proxy:\n{result.stdout}")

    if not _is_windows():
        binary_path.chmod(0o755)

# Check if the Go binary is missing or older than source files.
def is_rebuild_needed(go_dir: Path, binary_file: Path) -> bool:
    if not binary_file.exists():
        return True

    binary_mtime = binary_file.stat().st_mtime
    for go_file in go_dir.glob("*.go"):
        if go_file.stat().st_mtime > binary_mtime:
            return True
    for meta in ("go.mod", "go.sum"):
        p = go_dir / meta
        if p.exists() and p.stat().st_mtime > binary_mtime:
            return True
    return False

# Run the Go proxy, rebuilding if needed.
def run_proxy(target="http://localhost:4000", port=8080, **kwargs):
    # Validate target URL
    try:
        parsed = urlparse(target)
        if parsed.scheme not in ('http', 'https'):
            raise ValueError(f"Target must use http or https scheme, got: {target}")
        if not parsed.netloc:
            raise ValueError(f"Invalid target URL (missing host): {target}")
    except Exception as e:
        raise ValueError(f"Invalid target URL: {e}")

    # Validate port range
    if not isinstance(port, int) or not (1 <= port <= 65535):
        raise ValueError(f"Port must be an integer between 1-65535, got: {port}")

    # Ensure Go is installed.
    check_go_installed()

    # Define paths to Go directory and binary.
    go_dir = (Path(__file__).parent / "go").resolve()
    if not go_dir.exists():
        raise FileNotFoundError(f"{go_dir} does not exist")
    binary_path = (go_dir / _bin_name("proxy")).resolve()

    # Rebuild if missing or outdated.
    if is_rebuild_needed(go_dir, binary_path):
        build_proxy(go_dir, binary_path)

    # Prepare environment variables for proxy configuration.
    env = os.environ.copy()
    env["PY_BACKEND_TARGET"] = str(target)
    env["PY_BACKEND_PORT"] = str(port)

    if "max_conns_per_host" in kwargs:
        env["PROXY_MAX_CONNS_PER_HOST"] = str(kwargs["max_conns_per_host"])
    if "read_timeout" in kwargs:
        env["PROXY_READ_TIMEOUT"] = str(kwargs["read_timeout"])
    if "write_timeout" in kwargs:
        env["PROXY_WRITE_TIMEOUT"] = str(kwargs["write_timeout"])
    if "rate_limit_rps" in kwargs:
        env["PROXY_RATE_LIMIT_RPS"] = str(kwargs["rate_limit_rps"])
    if "max_request_body_size" in kwargs:
        env["PROXY_MAX_REQUEST_BODY_SIZE"] = str(kwargs["max_request_body_size"])
    if "allowed_origins" in kwargs:
        env["ALLOWED_ORIGINS"] = str(kwargs["allowed_origins"])

    print(f"Starting Go proxy at http://localhost:{port} -> {target}")

    # Start the Go binary as a subprocess, capturing output.
    proc = subprocess.Popen(
        [str(binary_path)],
        cwd=go_dir,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True
    )

    # Log output from the proxy process in a background thread.
    def log_output():
        try:
            assert proc.stdout is not None
            for line in proc.stdout:
                print(f"[proxy] {line.rstrip()}")
        except Exception:
            pass

    threading.Thread(target=log_output, daemon=True).start()

    return proc