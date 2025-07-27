import os
import subprocess
from pathlib import Path
import threading

# Check if Go is installed and available in PATH.
def check_go_installed():
    try:
        subprocess.run(
            ["go", "version"],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL
        )
    except Exception:
        raise RuntimeError("Go is not installed or not found in PATH.")

# Build the Go proxy binary from source.
def build_proxy(go_dir, binary_path):
    print("Building Go proxy ...")
    result = subprocess.run(
        ["go", "build", "-o", binary_path.name, "proxy.go"],
        cwd=go_dir,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True
    )
    if result.returncode != 0:
        raise RuntimeError(f"Failed to build Go proxy:\n{result.stdout}")

# Check if the Go binary is missing or older than the Go source file.
def is_rebuild_needed(go_file: Path, binary_file: Path) -> bool:
    """Return True if binary is missing or older than go_file."""
    if not binary_file.exists():
        return True
    return go_file.stat().st_mtime > binary_file.stat().st_mtime

# Run the Go proxy, rebuilding if needed.
def run_proxy(target="http://localhost:4000", port=8080):
    # Ensure Go is installed.
    check_go_installed()

    # Define paths to Go source and binary.
    go_dir = Path(__file__).parent / "go"
    binary_path = go_dir / "proxy"
    go_file = go_dir / "proxy.go"

    # Validate that source file exists.
    if not go_file.exists():
        raise FileNotFoundError(f"{go_file} does not exist")

    # Prepare environment variables to pass backend target and port.
    env = os.environ.copy()
    env["PY_BACKEND_TARGET"] = target
    env["PY_BACKEND_PORT"] = str(port)

    # Rebuild the binary if missing or out of date.
    if is_rebuild_needed(go_file, binary_path):
        build_proxy(go_dir, binary_path)

    print(f"Starting Go proxy at http://localhost:{port} (forwarding to {target})")

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
        for line in proc.stdout:
            print(f"[proxy] {line.strip()}")

    threading.Thread(target=log_output, daemon=True).start()

    return proc