package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
)

// Pre-allocated byte slices for headers (avoid per-request allocations)
var (
	healthPath = []byte("/health")

	// CORS header names
	hdrACAllowOrigin  = []byte("Access-Control-Allow-Origin")
	hdrACAllowHeaders = []byte("Access-Control-Allow-Headers")
	hdrACAllowMethods = []byte("Access-Control-Allow-Methods")
	hdrACAllowCreds   = []byte("Access-Control-Allow-Credentials")
	hdrACMaxAge       = []byte("Access-Control-Max-Age")

	// CORS header values
	valCORSHeaders = []byte("Content-Type, Authorization, X-Requested-With")
	valCORSMethods = []byte("GET, POST, PUT, DELETE, PATCH, OPTIONS")
	valCORSMaxAge  = []byte("86400")
	valTrue        = []byte("true")

	// Security header names and values
	hdrCacheControl = []byte("Cache-Control")
	valNoStore      = []byte("no-store")
	hdrXContentType = []byte("X-Content-Type-Options")
	valNosniff      = []byte("nosniff")
	hdrXFrame       = []byte("X-Frame-Options")
	valDeny         = []byte("DENY")
	hdrXXSS        = []byte("X-XSS-Protection")
	valXXSSBlock   = []byte("1; mode=block")

	// Vary header
	hdrVary        = []byte("Vary")
	valVaryOrigin  = []byte("Origin")
	valVaryHeaders = []byte("Access-Control-Request-Headers")
	valVaryMethod  = []byte("Access-Control-Request-Method")
)

// CORS allowed origins cache
var (
	allowedOriginsCache map[string]bool
	allowedOriginsMutex sync.RWMutex
	allowedOriginsEnv   string
	corsAllowCreds      bool
)

func initAllowedOrigins(allowCredentials bool) {
	allowedOriginsMutex.Lock()
	defer allowedOriginsMutex.Unlock()

	currentEnv := os.Getenv("ALLOWED_ORIGINS")
	if currentEnv == allowedOriginsEnv && allowedOriginsCache != nil {
		return
	}

	allowedOriginsEnv = currentEnv
	corsAllowCreds = allowCredentials
	allowedOriginsCache = make(map[string]bool)

	if currentEnv != "" {
		for _, o := range strings.Split(currentEnv, ",") {
			origin := strings.TrimSpace(o)
			if origin != "" && origin != "null" {
				allowedOriginsCache[origin] = true
			}
		}
	}
}

// Refreshes allowed origins periodically instead of on every request.
func startOriginsRefresh(allowCredentials bool, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			initAllowedOrigins(allowCredentials)
		}
	}()
}

func addCORSHeaders(ctx *fasthttp.RequestCtx) {
	origin := ctx.Request.Header.PeekBytes(hdrACAllowOrigin)
	if len(origin) == 0 {
		origin = ctx.Request.Header.Peek("Origin")
	}
	if len(origin) == 0 {
		return
	}
	originStr := string(origin)

	allowedOriginsMutex.RLock()
	isAllowed := allowedOriginsCache[originStr]
	creds := corsAllowCreds
	allowedOriginsMutex.RUnlock()

	if !isAllowed {
		return
	}

	h := &ctx.Response.Header
	h.SetBytesKV(hdrACAllowOrigin, origin)
	h.SetBytesKV(hdrACAllowHeaders, valCORSHeaders)
	h.SetBytesKV(hdrACAllowMethods, valCORSMethods)
	h.SetBytesKV(hdrACMaxAge, valCORSMaxAge)
	if creds {
		h.SetBytesKV(hdrACAllowCreds, valTrue)
	}
	h.AddBytesKV(hdrVary, valVaryOrigin)
	h.AddBytesKV(hdrVary, valVaryHeaders)
	h.AddBytesKV(hdrVary, valVaryMethod)
}

// Hop-by-hop headers (RFC 7230 Section 6.1)
var hopByHopHeaders = [...][]byte{
	[]byte("Connection"),
	[]byte("Proxy-Connection"),
	[]byte("Keep-Alive"),
	[]byte("TE"),
	[]byte("Trailer"),
	[]byte("Transfer-Encoding"),
	[]byte("Upgrade"),
	[]byte("Proxy-Authenticate"),
	[]byte("Proxy-Authorization"),
}

var connectionHeader = []byte("Connection")

// Strips hop-by-hop headers including dynamic ones listed in the Connection header.
func stripHopByHopReq(h *fasthttp.RequestHeader) {
	if conn := h.PeekBytes(connectionHeader); len(conn) > 0 {
		for _, name := range strings.Split(string(conn), ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				h.Del(name)
			}
		}
	}
	for _, k := range hopByHopHeaders {
		h.DelBytes(k)
	}
}

func stripHopByHopRes(h *fasthttp.ResponseHeader) {
	if conn := h.PeekBytes(connectionHeader); len(conn) > 0 {
		for _, name := range strings.Split(string(conn), ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				h.Del(name)
			}
		}
	}
	for _, k := range hopByHopHeaders {
		h.DelBytes(k)
	}
}

// isPrivateHost checks if a host string is a private/loopback/link-local address.
func isPrivateHost(host string) bool {
	h := host
	if idx := strings.LastIndex(h, ":"); idx != -1 {
		h = h[:idx]
	}
	h = strings.TrimPrefix(strings.TrimSuffix(h, "]"), "[")

	if strings.EqualFold(h, "localhost") {
		return true
	}

	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func validateTarget(target string) (*url.URL, error) {
	backendURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if backendURL.Scheme != "http" && backendURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported target scheme: %s", backendURL.Scheme)
	}
	if backendURL.Host == "" {
		return nil, fmt.Errorf("target is missing host")
	}
	if isPrivateHost(backendURL.Host) {
		log.Printf("WARNING: target %q resolves to a private/loopback address. This is expected for local development but should not be used in production.", backendURL.Host)
	}
	return backendURL, nil
}

// Health check caching
var (
	healthCacheOK   atomic.Bool
	healthCacheTime atomic.Int64 // unix nanoseconds
)

func isBackendReachable(client *fasthttp.Client, backendURL *url.URL, timeout, cacheTTL time.Duration) bool {
	if time.Since(time.Unix(0, healthCacheTime.Load())) < cacheTTL {
		return healthCacheOK.Load()
	}

	healthURL := *backendURL
	if healthURL.Path == "" {
		healthURL.Path = "/"
	}

	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(healthURL.String())
	req.Header.SetMethod(fasthttp.MethodGet)

	ok := client.DoTimeout(req, res, timeout) == nil
	healthCacheOK.Store(ok)
	healthCacheTime.Store(time.Now().UnixNano())
	return ok
}

func newProxyServer(target string, config *Config) (*fasthttp.Server, error) {
	initAllowedOrigins(config.CORSAllowCredentials)
	startOriginsRefresh(config.CORSAllowCredentials, config.CORSRefreshInterval)

	backendURL, err := validateTarget(target)
	if err != nil {
		return nil, err
	}

	var rateLimiter *RateLimiter
	if config.RateLimitRPS > 0 {
		rateLimiter = NewRateLimiter(config.RateLimitRPS)
		cleanupInterval := config.RateLimitCleanupInterval
		go func() {
			ticker := time.NewTicker(cleanupInterval)
			defer ticker.Stop()
			for range ticker.C {
				rateLimiter.Cleanup(cleanupInterval)
			}
		}()
	}

	client := &fasthttp.Client{
		ReadTimeout:                   config.ReadTimeout,
		WriteTimeout:                  config.WriteTimeout,
		MaxIdleConnDuration:           config.MaxIdleConnDuration,
		MaxConnsPerHost:               config.MaxConnsPerHost,
		MaxConnWaitTimeout:            config.MaxConnWaitTimeout,
		ReadBufferSize:                config.ReadBufferSize,
		WriteBufferSize:               config.WriteBufferSize,
		DisableHeaderNamesNormalizing: true,
		NoDefaultUserAgentHeader:      true,
	}

	handler := func(ctx *fasthttp.RequestCtx) {
		clientIP := ctx.RemoteIP().String()

		// Health endpoint (rate-limited + cached)
		if bytes.Equal(ctx.Path(), healthPath) {
			if rateLimiter != nil && !rateLimiter.Allow(clientIP) {
				ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"error":"rate limit exceeded"}`)
				return
			}
			healthy := isBackendReachable(client, backendURL, 2*time.Second, config.HealthCacheTTL)
			if !healthy {
				ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
				ctx.SetContentType("application/json")
				ctx.SetBodyString(`{"status":"unhealthy","details":"backend unreachable"}`)
				return
			}
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetContentType("text/plain")
			ctx.SetBodyString("ok")
			return
		}

		// Per-IP rate limit
		if rateLimiter != nil && !rateLimiter.Allow(clientIP) {
			ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
			ctx.SetContentType("application/json")
			ctx.SetBodyString(`{"error":"rate limit exceeded"}`)
			return
		}

		// CORS preflight
		if ctx.IsOptions() {
			addCORSHeaders(ctx)
			ctx.SetStatusCode(fasthttp.StatusNoContent)
			ctx.Response.ResetBody()
			return
		}

		// Build backend URL from original path and query
		u := *backendURL
		uri := ctx.URI()
		u.Path = string(uri.PathOriginal())
		u.RawQuery = string(uri.QueryString())

		// Prepare proxied request and response
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(res)

		ctx.Request.CopyTo(req)
		stripHopByHopReq(&req.Header)

		req.SetRequestURI(u.String())
		req.URI().SetScheme(backendURL.Scheme)
		req.URI().SetHost(backendURL.Host)
		req.Header.SetHost(backendURL.Host)

		// X-Forwarded-* (always overwrite to prevent spoofing)
		req.Header.Set("X-Forwarded-For", clientIP)
		if ctx.IsTLS() {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
		req.Header.Set("X-Forwarded-Host", string(ctx.Host()))

		if err := client.Do(req, res); err != nil {
			log.Printf("Proxy error: %v", err)
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
			ctx.SetContentType("application/json")
			ctx.SetBodyString(`{"error":"backend unreachable"}`)
			return
		}

		// Copy response
		ctx.SetStatusCode(res.StatusCode())
		res.Header.CopyTo(&ctx.Response.Header)
		stripHopByHopRes(&ctx.Response.Header)

		addCORSHeaders(ctx)

		// Security headers (only set Cache-Control if backend didn't)
		if len(ctx.Response.Header.PeekBytes(hdrCacheControl)) == 0 {
			ctx.Response.Header.SetBytesKV(hdrCacheControl, valNoStore)
		}
		ctx.Response.Header.SetBytesKV(hdrXContentType, valNosniff)
		ctx.Response.Header.SetBytesKV(hdrXFrame, valDeny)
		ctx.Response.Header.SetBytesKV(hdrXXSS, valXXSSBlock)

		ctx.SetBody(res.Body())
	}

	server := &fasthttp.Server{
		Handler:            handler,
		ReadTimeout:        config.ReadTimeout,
		WriteTimeout:       config.WriteTimeout,
		MaxRequestBodySize: config.MaxRequestBodySize,
	}

	return server, nil
}

func proxy(target string, port string, config *Config) {
	server, err := newProxyServer(target, config)
	if err != nil {
		log.Fatalf("Invalid proxy configuration: %v", err)
	}

	addr := ":" + port
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Pygofastproxy running at %s, forwarding to backend", addr)
		if config.TLSCertFile != "" && config.TLSKeyFile != "" {
			log.Printf("TLS enabled")
			if err := server.ListenAndServeTLS(addr, config.TLSCertFile, config.TLSKeyFile); err != nil {
				log.Fatalf("Proxy server error: %v", err)
			}
		} else {
			if err := server.ListenAndServe(addr); err != nil {
				log.Fatalf("Proxy server error: %v", err)
			}
		}
	}()

	<-stop
	log.Printf("Shutdown signal received, stopping proxy...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.ShutdownWithContext(ctx); err != nil {
		log.Printf("Graceful shutdown error: %v", err)
	}
}

func main() {
	target := os.Getenv("PY_BACKEND_TARGET")
	port := os.Getenv("PY_BACKEND_PORT")

	if target == "" {
		log.Fatal("Environment variable PY_BACKEND_TARGET is not set")
	}
	if port == "" {
		log.Fatal("Environment variable PY_BACKEND_PORT is not set")
	}

	for _, c := range port {
		if c < '0' || c > '9' {
			log.Fatalf("Invalid port: %s (must be numeric)", port)
		}
	}

	config := LoadConfig()
	log.Printf("Starting proxy on port %s", port)
	proxy(target, port, config)
}
