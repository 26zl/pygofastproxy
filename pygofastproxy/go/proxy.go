package main

import (
	"log"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/valyala/fasthttp"
)

// Proxy starts a reverse proxy on the given port and forwards to the given target backend URL.
func Proxy(target string, port string) {
	// Parse the target backend URL to ensure it is valid.
	backendURL, err := url.Parse(target)
	if err != nil {
		log.Fatalf("Invalid target URL: %v", err)
	}

	// Define the HTTP handler function for incoming requests.
	handler := func(ctx *fasthttp.RequestCtx) {
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
		err = fasthttp.Do(req, res)
		if err != nil {
			// If backend is unreachable or errors out, respond with 502 Bad Gateway.
			log.Printf("Proxy error: %v", err)
			ctx.SetStatusCode(fasthttp.StatusBadGateway)
			ctx.SetBodyString(`{"error": "proxy failed"}`)
			ctx.SetContentType("application/json")
			return
		}

		// Forward the backend response to the client.
		ctx.SetStatusCode(res.StatusCode())
		ctx.Response.Header.SetContentType(string(res.Header.ContentType()))
		ctx.SetBody(res.Body())
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
