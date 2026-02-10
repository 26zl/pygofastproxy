# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in pygofastproxy, please report it.

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | Yes       |

## Security Features

pygofastproxy includes several built-in security features:

- Per-IP rate limiting (token bucket)
- Automatic security headers (X-Content-Type-Options, X-Frame-Options, X-XSS-Protection)
- Hop-by-hop header stripping (RFC 7230)
- X-Forwarded header overwriting (prevents client spoofing)
- Request body size limits
- CORS origin allowlist with `null` origin rejection
- No internal URL or error detail leakage to clients
- Private/loopback target warnings
- Optional TLS termination
- Docker: non-root user, multi-stage build
