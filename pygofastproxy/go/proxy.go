package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/valyala/fasthttp"
)

//CORS / allowed origins cache

var (
	allowedOriginsCache map[string]bool
	allowedOriginsMutex sync.RWMutex
	allowedOriginsEnv   string
)

// Initializes the allowed origins cache.
func initAllowedOrigins() {
	allowedOriginsMutex.Lock()
	defer allowedOriginsMutex.Unlock()

	currentEnv := os.Getenv("ALLOWED_ORIGINS")
	if currentEnv == allowedOriginsEnv && allowedOriginsCache != nil {
		return
	}

	allowedOriginsEnv = currentEnv
	allowedOriginsCache = make(map[string]bool)

	if currentEnv != "" {
		for _, o := range strings.Split(currentEnv, ",") {
			allowedOriginsCache[strings.TrimSpace(o)] = true
		}
	}
}

// Adds CORS headers to the response.
func addCORSHeaders(ctx *fasthttp.RequestCtx) {
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return
	}
	allowedOriginsMutex.RLock()
	isAllowed := allowedOriginsCache[origin]
	allowedOriginsMutex.RUnlock()
	if !isAllowed {
		return
	}
	h := &ctx.Response.Header
	h.Set("Access-Control-Allow-Origin", origin)
	h.SetBytesKV([]byte("Access-Control-Allow-Headers"), []byte("Content-Type, Authorization, X-Requested-With"))
	h.SetBytesKV([]byte("Access-Control-Allow-Methods"), []byte("GET, POST, PUT, DELETE, PATCH, OPTIONS"))
	h.SetBytesKV([]byte("Access-Control-Allow-Credentials"), []byte("true"))
	h.SetBytesKV([]byte("Access-Control-Max-Age"), []byte("86400"))
	h.Add("Vary", "Origin")
	h.Add("Vary", "Access-Control-Request-Headers")
	h.Add("Vary", "Access-Control-Request-Method")
}

// Hop-by-hop headers

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

func stripHopByHopReq(h *fasthttp.RequestHeader) {
	for _, k := range hopByHopHeaders {
		h.DelBytes(k)
	}
}
func stripHopByHopRes(h *fasthttp.ResponseHeader) {
	for _, k := range hopByHopHeaders {
		h.DelBytes(k)
	}
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
	return backendURL, nil
}

func isBackendReachable(client *fasthttp.Client, backendURL *url.URL, timeout time.Duration) bool {
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

	if err := client.DoTimeout(req, res, timeout); err != nil {
		return false
	}
	return true
}

func newProxyServer(target string) (*fasthttp.Server, error) {
	config := LoadConfig()
	initAllowedOrigins()

	backendURL, err := validateTarget(target)
	if err != nil {
		return nil, err
	}

	var rateLimiter *RateLimiter
	if config.RateLimitRPS > 0 {
		rateLimiter = NewRateLimiter(config.RateLimitRPS, time.Second)
	}

	client := &fasthttp.Client{
		ReadTimeout:                   config.ReadTimeout,
		WriteTimeout:                  config.WriteTimeout,
		MaxIdleConnDuration:           config.MaxIdleConnDuration,
		MaxConnsPerHost:               config.MaxConnsPerHost,
		ReadBufferSize:                config.ReadBufferSize,
		WriteBufferSize:               config.WriteBufferSize,
		DisableHeaderNamesNormalizing: true,
			NoDefaultUserAgentHeader:      true,
	}

	handler := func(ctx *fasthttp.RequestCtx) {
		// refresh allow list from env if it changed
		initAllowedOrigins()

		// lightweight health endpoint
		if string(ctx.Path()) == "/health" {
			healthy := isBackendReachable(client, backendURL, 2*time.Second)
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

		// global rate limit (skip when disabled)
		if rateLimiter != nil && !rateLimiter.Allow() {
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

		// Copy original request
		ctx.Request.CopyTo(req)

		// Strip hop-by-hop on the way to backend
		stripHopByHopReq(&req.Header)

		// Set scheme/host/URI explicitly to backend
		req.SetRequestURI(u.String())
		req.URI().SetScheme(backendURL.Scheme)
		req.URI().SetHost(backendURL.Host)
		req.Header.SetHost(backendURL.Host)

		// X-Forwarded-*
		clientIP := ctx.RemoteIP().String()
		if xff := req.Header.Peek("X-Forwarded-For"); len(xff) > 0 {
			req.Header.Set("X-Forwarded-For", string(xff)+", "+clientIP)
		} else {
			req.Header.Set("X-Forwarded-For", clientIP)
		}
		if ctx.IsTLS() {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
		if req.Header.Peek("X-Forwarded-Host") == nil {
			req.Header.Set("X-Forwarded-Host", string(ctx.Host()))
		}

		// Do backend call
		if err := client.Do(req, res); err != nil {
			log.Printf("Proxy error for %s: %v", u.String(), err)
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
			ctx.SetContentType("application/json")
			ctx.SetBodyString(`{"error":"proxy failed","details":"backend unreachable"}`)
			return
		}

		// Copy response headers/status, strip hop-by-hop, then add security/CORS
		ctx.SetStatusCode(res.StatusCode())
		res.Header.CopyTo(&ctx.Response.Header)
		stripHopByHopRes(&ctx.Response.Header)

		addCORSHeaders(ctx)
		ctx.Response.Header.SetBytesKV([]byte("Cache-Control"), []byte("no-store"))
		ctx.Response.Header.SetBytesKV([]byte("X-Content-Type-Options"), []byte("nosniff"))
		ctx.Response.Header.SetBytesKV([]byte("X-Frame-Options"), []byte("DENY"))
		ctx.Response.Header.SetBytesKV([]byte("X-XSS-Protection"), []byte("1; mode=block"))
		ctx.Response.Header.SetBytesKV([]byte("X-Proxy-Server"), []byte("pygofastproxy"))
		ctx.Response.Header.SetBytesKV([]byte("X-Proxy-Target"), []byte(target))

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

// Proxy starts a reverse proxy on the given port and forwards to the given target backend URL.
func Proxy(target string, port string) {
	server, err := newProxyServer(target)
	if err != nil {
		log.Fatalf("Invalid proxy configuration: %v", err)
	}

	addr := ":" + port
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Pygofastproxy running at %s, forwarding to %s\n", addr, target)
		if err := server.ListenAndServe(addr); err != nil {
			log.Fatalf("Proxy server error: %v", err)
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

// Main initializes the proxy server.
func main() {
	target := os.Getenv("PY_BACKEND_TARGET")
	port := os.Getenv("PY_BACKEND_PORT")

	if target == "" {
		log.Fatal("Environment variable PY_BACKEND_TARGET is not set")
	}
	if port == "" {
		log.Fatal("Environment variable PY_BACKEND_PORT is not set")
	}

	log.Printf("Starting proxy on port %s -> forwarding to %s", port, target)
	Proxy(target, port)
}
