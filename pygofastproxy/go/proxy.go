package main

import (
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

// Helper to add CORS headers for specific allowed origins only
func addCORSHeaders(ctx *fasthttp.RequestCtx) {
	origin := string(ctx.Request.Header.Peek("Origin"))
	if origin == "" {
		return
	}

	// Retrieve allowed domains from environment variable
	allowedList := os.Getenv("ALLOWED_ORIGINS")
	if allowedList == "" {
		return
	}

	// Split into comma-separated list and build a map
	allowedOrigins := make(map[string]bool)
	for _, o := range strings.Split(allowedList, ",") {
		allowedOrigins[strings.TrimSpace(o)] = true
	}

	// Add CORS headers if origin is allowed
	if allowedOrigins[origin] {
		ctx.Response.Header.Set("Access-Control-Allow-Origin", origin)
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		ctx.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
	}
}

// Proxy starts a reverse proxy on the given port and forwards to the given target backend URL.
func Proxy(target string, port string) {
	// Parse the target backend URL to ensure it is valid.
	backendURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("Invalid target URL: %v", err)
	}

	// Create fasthttp client with timeout settings.
	client := &fasthttp.Client{
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        30 * time.Second,
		MaxIdleConnDuration: 10 * time.Second,
	}

	// Define the HTTP handler function for incoming requests.
	handler := func(ctx *fasthttp.RequestCtx) {
		start := time.Now()

		// Add security headers.
		ctx.Response.Header.Set("X-Proxy-Server", "pygofastproxy")
		ctx.Response.Header.Set("X-Proxy-Target", target)

		// Handle CORS preflight requests.
		if string(ctx.Method()) == "OPTIONS" {
			addCORSHeaders(ctx)
			ctx.SetStatusCode(204)
			return
		}

		// Get the raw requested path.
		rawPath := string(ctx.Request.URI().PathOriginal())

		// Reject empty path.
		if rawPath == "" {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(`{"error": "empty path"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Path must start with a forward slash.
		if !strings.HasPrefix(rawPath, "/") {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(`{"error": "invalid path prefix"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Decode URL-encoded characters in the path.
		decodedPath, err := url.PathUnescape(rawPath)
		if err != nil {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(`{"error": "path decoding failed"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Reject if path contains a NULL byte (potentially malicious).
		if strings.ContainsRune(decodedPath, '\x00') {
			ctx.SetStatusCode(fasthttp.StatusBadRequest)
			ctx.SetBodyString(`{"error": "null byte in path"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Normalize the path (removes redundant slashes and resolves ".." and ".").
		cleanPath := path.Clean(decodedPath)

		// Forbid directory traversal outside root (defense-in-depth).
		if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/../") {
			ctx.SetStatusCode(fasthttp.StatusForbidden)
			ctx.SetBodyString(`{"error": "path traversal attempt"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Ensure path is still absolute after cleaning.
		if !strings.HasPrefix(cleanPath, "/") {
			cleanPath = "/" + cleanPath
		}

		// Build the full backend URL including path and query string.
		u := *backendURL
		u.Path = cleanPath
		u.RawQuery = string(ctx.URI().QueryString())

		// Prepare proxied request and response objects.
		req := fasthttp.AcquireRequest()
		res := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseRequest(req)
		defer fasthttp.ReleaseResponse(res)

		// Copy the original client request into the proxied request.
		ctx.Request.CopyTo(req)

		// Set the backend URL we constructed.
		req.SetRequestURI(u.String())

		// Perform the request to the backend server.
		err = client.Do(req, res)
		if err != nil {
			// If backend is unreachable or errors out, respond with 502 Bad Gateway.
			log.Printf("Proxy error for %s: %v", u.String(), err)
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
			ctx.SetBodyString(`{"error": "proxy failed", "details": "backend unreachable"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Forward the backend response to the client.
		ctx.SetStatusCode(res.StatusCode())
		ctx.Response.Header.SetContentType(string(res.Header.ContentType()))
		ctx.SetBody(res.Body())
		addCORSHeaders(ctx)

		// Add standard security headers.
		ctx.Response.Header.Set("Cache-Control", "no-store")
		ctx.Response.Header.Set("X-Content-Type-Options", "nosniff")
		ctx.Response.Header.Set("X-Frame-Options", "DENY")
		ctx.Response.Header.Set("X-XSS-Protection", "1; mode=block")

		// Forward CORS headers from backend response (removed for strict production CORS handling).

		// Log failed requests (to avoid excessive logging).
		if res.StatusCode() >= 400 {
			log.Printf("Proxy request to %s returned %d in %v", cleanPath, res.StatusCode(), time.Since(start))
		}
	}

	// Start the server and listen on the given port.
	log.Printf("Fasthttp proxy running at :%s, forwarding to %s\n", port, target)
	log.Fatal(fasthttp.ListenAndServe(":"+port, handler))
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

	log.Printf("Starting proxy on port %s -> forwarding to %s", port, target)
	Proxy(target, port)
}
