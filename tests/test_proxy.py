"""Comprehensive tests for pygofastproxy."""

import pytest
import requests

from pygofastproxy.runner import run_proxy


# ---------------------------------------------------------------------------
# Basic proxy forwarding
# ---------------------------------------------------------------------------


class TestProxyForwarding:
    def test_get_request(self, proxy_url):
        res = requests.get(f"{proxy_url}/hello", timeout=3)
        assert res.status_code == 200
        data = res.json()
        assert data["method"] == "GET"
        assert data["path"] == "/hello"

    def test_post_request(self, proxy_url):
        res = requests.post(f"{proxy_url}/data", data=b"hello world", timeout=3)
        assert res.status_code == 200
        data = res.json()
        assert data["method"] == "POST"
        assert data["body_size"] == 11

    def test_put_request(self, proxy_url):
        res = requests.put(f"{proxy_url}/update", data=b"updated", timeout=3)
        assert res.status_code == 200
        assert res.json()["method"] == "PUT"

    def test_delete_request(self, proxy_url):
        res = requests.delete(f"{proxy_url}/remove", timeout=3)
        assert res.status_code == 200
        assert res.json()["method"] == "DELETE"

    def test_query_string_forwarded(self, proxy_url):
        res = requests.get(f"{proxy_url}/search?q=test&page=2", timeout=3)
        assert res.status_code == 200
        data = res.json()
        assert "q=test" in data["path"]
        assert "page=2" in data["path"]

    def test_response_content_type(self, proxy_url):
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert "application/json" in res.headers.get("Content-Type", "")


# ---------------------------------------------------------------------------
# Security headers
# ---------------------------------------------------------------------------


class TestSecurityHeaders:
    def test_x_content_type_options(self, proxy_url):
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert res.headers.get("X-Content-Type-Options") == "nosniff"

    def test_x_frame_options(self, proxy_url):
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert res.headers.get("X-Frame-Options") == "DENY"

    def test_x_xss_protection(self, proxy_url):
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert res.headers.get("X-XSS-Protection") == "1; mode=block"

    def test_no_proxy_target_header(self, proxy_url):
        """X-Proxy-Target must NOT be present (leaks backend URL)."""
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert "X-Proxy-Target" not in res.headers

    def test_no_proxy_server_header(self, proxy_url):
        """X-Proxy-Server must NOT be present (fingerprinting)."""
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert "X-Proxy-Server" not in res.headers

    def test_cache_control_default(self, proxy_url):
        """When backend doesn't set Cache-Control, proxy adds no-store."""
        res = requests.get(f"{proxy_url}/", timeout=3)
        assert res.headers.get("Cache-Control") == "no-store"

    def test_cache_control_preserved(self, caching_proxy):
        """When backend sets its own Cache-Control, proxy preserves it."""
        res = requests.get(f"{caching_proxy}/", timeout=3)
        assert "max-age=3600" in res.headers.get("Cache-Control", "")


# ---------------------------------------------------------------------------
# Health endpoint
# ---------------------------------------------------------------------------


class TestHealthEndpoint:
    def test_health_returns_ok(self, proxy_url):
        res = requests.get(f"{proxy_url}/health", timeout=3)
        assert res.status_code == 200
        assert res.text == "ok"

    def test_health_content_type(self, proxy_url):
        res = requests.get(f"{proxy_url}/health", timeout=3)
        assert "text/plain" in res.headers.get("Content-Type", "")


# ---------------------------------------------------------------------------
# CORS
# ---------------------------------------------------------------------------


class TestCORS:
    def test_allowed_origin_gets_cors_headers(self, cors_proxy):
        res = requests.get(
            f"{cors_proxy}/",
            headers={"Origin": "http://allowed.example.com"},
            timeout=3,
        )
        assert (
            res.headers.get("Access-Control-Allow-Origin")
            == "http://allowed.example.com"
        )

    def test_disallowed_origin_no_cors_headers(self, cors_proxy):
        res = requests.get(
            f"{cors_proxy}/",
            headers={"Origin": "http://evil.example.com"},
            timeout=3,
        )
        assert "Access-Control-Allow-Origin" not in res.headers

    def test_no_origin_no_cors_headers(self, cors_proxy):
        res = requests.get(f"{cors_proxy}/", timeout=3)
        assert "Access-Control-Allow-Origin" not in res.headers

    def test_preflight_options(self, cors_proxy):
        res = requests.options(
            f"{cors_proxy}/api",
            headers={
                "Origin": "http://allowed.example.com",
                "Access-Control-Request-Method": "POST",
            },
            timeout=3,
        )
        assert res.status_code == 204
        assert (
            res.headers.get("Access-Control-Allow-Origin")
            == "http://allowed.example.com"
        )
        assert "POST" in res.headers.get("Access-Control-Allow-Methods", "")

    def test_no_credentials_by_default(self, cors_proxy):
        """Access-Control-Allow-Credentials should not be set by default."""
        res = requests.get(
            f"{cors_proxy}/",
            headers={"Origin": "http://allowed.example.com"},
            timeout=3,
        )
        assert "Access-Control-Allow-Credentials" not in res.headers

    def test_vary_header_set(self, cors_proxy):
        res = requests.get(
            f"{cors_proxy}/",
            headers={"Origin": "http://allowed.example.com"},
            timeout=3,
        )
        vary = res.headers.get("Vary", "")
        assert "Origin" in vary


# ---------------------------------------------------------------------------
# Rate limiting
# ---------------------------------------------------------------------------


class TestRateLimiting:
    def test_rate_limit_enforced(self, rate_limited_proxy):
        """With rate_limit_rps=3, rapid requests should eventually get 429."""
        got_429 = False
        for _ in range(20):
            res = requests.get(f"{rate_limited_proxy}/", timeout=3)
            if res.status_code == 429:
                got_429 = True
                break
        assert got_429, "Expected at least one 429 response with rate_limit_rps=3"

    def test_rate_limit_response_body(self, rate_limited_proxy):
        """429 responses should have a JSON error body."""
        for _ in range(20):
            res = requests.get(f"{rate_limited_proxy}/", timeout=3)
            if res.status_code == 429:
                data = res.json()
                assert "rate limit" in data.get("error", "").lower()
                return
        pytest.skip("Did not trigger rate limit")


# ---------------------------------------------------------------------------
# Configuration validation
# ---------------------------------------------------------------------------


class TestConfigValidation:
    def test_invalid_scheme_raises(self):
        with pytest.raises(ValueError, match="http"):
            run_proxy(target="ftp://localhost:4000", port=9999)

    def test_missing_host_raises(self):
        with pytest.raises(ValueError):
            run_proxy(target="http://", port=9999)

    def test_invalid_port_raises(self):
        with pytest.raises(ValueError, match="Port"):
            run_proxy(target="http://localhost:4000", port=0)

    def test_port_too_high_raises(self):
        with pytest.raises(ValueError, match="Port"):
            run_proxy(target="http://localhost:4000", port=99999)

    def test_port_not_int_raises(self):
        with pytest.raises(ValueError, match="Port"):
            run_proxy(target="http://localhost:4000", port="abc")
