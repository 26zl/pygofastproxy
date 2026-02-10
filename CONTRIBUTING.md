# Contributing to pygofastproxy

Thanks for your interest in contributing!

## Reporting Bugs

Open an issue using the [bug report template](https://github.com/26zl/pygofastproxy/issues/new?template=bug_report.md). Include:

- Your OS and Python/Go versions
- Steps to reproduce
- Expected vs actual behavior
- Error output or logs

## Suggesting Features

Open an issue using the [feature request template](https://github.com/26zl/pygofastproxy/issues/new?template=feature_request.md).

## Development Setup

```bash
# Clone the repo
git clone https://github.com/26zl/pygofastproxy.git
cd pygofastproxy

# Install with test dependencies
pip install -e ".[test]"

# Run tests
pytest tests/ -v

# Run the benchmark
python benchmarks/benchmark.py
```

**Requirements:** Python 3.8+ and Go 1.25+.

## Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b my-feature`)
3. Make your changes
4. Run tests (`pytest tests/ -v`) and make sure they pass
5. Commit and push
6. Open a PR against `main`

### Code Style

- **Python:** Follow [ruff](https://docs.astral.sh/ruff/) defaults. Run `ruff check .` and `ruff format .` before committing.
- **Go:** Follow standard Go conventions. Run `go vet ./...` in `pygofastproxy/go/`.

### What Makes a Good PR

- Focused on a single change
- Includes tests for new functionality
- Doesn't break existing tests
- Has a clear description of what and why
