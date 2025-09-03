package main

import (
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
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

// Proxy starts a reverse proxy on the given port and forwards to the given target backend URL.
func Proxy(target string, port string) {
	config := LoadConfig()
	initAllowedOrigins()

	var metrics *Metrics
	if config.EnableMetrics {
		metrics = NewMetrics()
	}
	rateLimiter := NewRateLimiter(config.RateLimitRPS, time.Second)

	backendURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("Invalid target URL: %v", err)
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
		// metrics endpoint
		if config.EnableMetrics && string(ctx.Path()) == "/__proxy_metrics" {
			reqCount, errCount, avgDuration, uptime := metrics.GetStats()
			var errRate float64
			if reqCount > 0 {
				errRate = float64(errCount) / float64(reqCount) * 100
			}
			resp := map[string]any{
				"requests":        reqCount,
				"errors":          errCount,
				"avg_duration_ms": float64(avgDuration) / float64(time.Millisecond),
				"uptime_seconds":  uptime.Seconds(),
				"error_rate":      errRate,
			}
			b, _ := json.Marshal(resp)
			ctx.SetContentType("application/json")
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.SetBody(b)
			return
		}

		start := time.Now()

		// global rate limit
		if !rateLimiter.Allow() {
			if config.EnableMetrics {
				metrics.RecordRequest(time.Since(start), true)
			}
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
			if config.EnableMetrics {
				metrics.RecordRequest(time.Since(start), false)
			}
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
			if config.EnableMetrics {
				metrics.RecordRequest(time.Since(start), true)
			}
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

		// Metrics/logging
		isError := res.StatusCode() >= 400
		if config.EnableMetrics {
			metrics.RecordRequest(time.Since(start), isError)
		}
		if isError {
			log.Printf("Proxy request %s -> %d in %v", u.String(), res.StatusCode(), time.Since(start))
		}
	}

	addr := ":" + port
	log.Printf("Fasthttp proxy running at %s, forwarding to %s\n", addr, target)
	log.Fatal(fasthttp.ListenAndServe(addr, handler))
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
