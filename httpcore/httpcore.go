// Package httpcore provides the Go HTTP bridge for the GALA server library.
// It wraps net/http with immutable, functional-friendly types that GALA code
// can use directly. This is the only package that touches net/http internals.
package httpcore

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"martianoff/gala/std"
)

// Handler processes an HTTP request and returns an immutable response.
type Handler = func(Request) Response

// Middleware wraps a Handler with additional behavior.
// Middleware is applied in reverse order: last added = outermost wrapper.
type Middleware = func(Handler) Handler

// --- Request ---

// Request is an immutable representation of an HTTP request.
// It wraps Go's *http.Request and provides functional accessors
// that return Option[string] for safe value extraction.
type Request struct {
	goReq *http.Request
	body  string
}

// Method returns the HTTP method (GET, POST, PUT, DELETE, etc.).
func (r Request) Method() string { return r.goReq.Method }

// Path returns the URL path.
func (r Request) Path() string { return r.goReq.URL.Path }

// RawQuery returns the raw query string.
func (r Request) RawQuery() string { return r.goReq.URL.RawQuery }

// Body returns the request body as a string.
func (r Request) Body() string { return r.body }

// Param returns a path parameter by name (Go 1.22+ wildcard patterns).
// Example: for pattern "/users/{id}", req.Param("id") returns Some("123").
func (r Request) Param(name string) std.Option[string] {
	v := r.goReq.PathValue(name)
	if v == "" {
		return std.None[string]{}.Apply()
	}
	return std.Some[string]{}.Apply(v)
}

// QueryParam returns a query parameter by name.
// Example: for URL "/search?q=gala", req.QueryParam("q") returns Some("gala").
func (r Request) QueryParam(name string) std.Option[string] {
	v := r.goReq.URL.Query().Get(name)
	if v == "" {
		return std.None[string]{}.Apply()
	}
	return std.Some[string]{}.Apply(v)
}

// Header returns a header value by name (case-insensitive).
// Example: req.Header("Content-Type") returns Some("application/json").
func (r Request) Header(name string) std.Option[string] {
	v := r.goReq.Header.Get(name)
	if v == "" {
		return std.None[string]{}.Apply()
	}
	return std.Some[string]{}.Apply(v)
}

// ContentType returns the Content-Type header value.
func (r Request) ContentType() std.Option[string] {
	return r.Header("Content-Type")
}

// --- Response ---

// Response is an immutable HTTP response.
// Use NewResponse() and the With* builder methods to construct responses.
// Each builder returns a new Response (no mutation).
type Response struct {
	Status  int
	headers map[string]string
	body    string
}

// NewResponse creates a Response with the given status code.
func NewResponse(status int) Response {
	return Response{Status: status, headers: make(map[string]string)}
}

// WithHeader returns a new Response with an additional header set.
func (r Response) WithHeader(key, value string) Response {
	newHeaders := make(map[string]string, len(r.headers)+1)
	for k, v := range r.headers {
		newHeaders[k] = v
	}
	newHeaders[key] = value
	return Response{Status: r.Status, headers: newHeaders, body: r.body}
}

// WithBody returns a new Response with the given body.
func (r Response) WithBody(body string) Response {
	return Response{Status: r.Status, headers: r.headers, body: body}
}

// WithStatus returns a new Response with the given status code.
func (r Response) WithStatus(status int) Response {
	return Response{Status: status, headers: r.headers, body: r.body}
}

// Body returns the response body.
func (r Response) Body() string { return r.body }

// HeaderValue returns a response header value by key.
func (r Response) HeaderValue(key string) string {
	if r.headers == nil {
		return ""
	}
	return r.headers[key]
}

// --- Server Builder ---

// ServerBuilder accumulates routes and middleware for HTTP server construction.
// This is a mutable builder used internally by App.Listen().
type ServerBuilder struct {
	routes      []routeEntry
	middlewares []Middleware
}

type routeEntry struct {
	method  string
	pattern string
	handler Handler
}

// NewServerBuilder creates a new ServerBuilder.
func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{}
}

// AddRoute registers a route with the server.
func (sb *ServerBuilder) AddRoute(method, pattern string, handler Handler) {
	sb.routes = append(sb.routes, routeEntry{method, pattern, handler})
}

// AddMiddleware registers middleware with the server.
func (sb *ServerBuilder) AddMiddleware(mw Middleware) {
	sb.middlewares = append(sb.middlewares, mw)
}

// ListenAndServe starts the HTTP server on the given address.
// It blocks until the server is shut down or encounters an error.
func (sb *ServerBuilder) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	for _, route := range sb.routes {
		handler := route.handler

		// Apply middleware in reverse order (last added = outermost)
		for i := len(sb.middlewares) - 1; i >= 0; i-- {
			handler = sb.middlewares[i](handler)
		}

		// Go 1.22+ pattern: "METHOD /path"
		pattern := route.method + " " + route.pattern
		h := handler
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			req := newRequest(r)
			resp := h(req)
			writeResponse(w, resp)
		})
	}

	fmt.Printf("GALA Server listening on %s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func newRequest(r *http.Request) Request {
	body := ""
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil {
			body = string(bodyBytes)
		}
		r.Body.Close()
	}
	return Request{goReq: r, body: body}
}

func writeResponse(w http.ResponseWriter, resp Response) {
	for k, v := range resp.headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.Status)
	if resp.body != "" {
		w.Write([]byte(resp.body))
	}
}

// --- Built-in Middleware ---

// LoggerMiddleware logs each request: method, path, status code, and duration.
func LoggerMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(req Request) Response {
			start := time.Now()
			resp := next(req)
			duration := time.Since(start)
			log.Printf("%s %s -> %d (%s)", req.Method(), req.Path(), resp.Status, duration)
			return resp
		}
	}
}

// RecoveryMiddleware catches panics in handlers and returns a 500 response.
func RecoveryMiddleware() Middleware {
	return func(next Handler) Handler {
		return func(req Request) (resp Response) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC: %v\n%s", r, debug.Stack())
					resp = NewResponse(500).
						WithBody("Internal Server Error").
						WithHeader("Content-Type", "text/plain")
				}
			}()
			return next(req)
		}
	}
}

// CorsMiddleware adds CORS headers to responses.
// If no origins are specified, defaults to "*".
func CorsMiddleware(origins ...string) Middleware {
	allowOrigin := "*"
	if len(origins) > 0 {
		allowOrigin = strings.Join(origins, ", ")
	}
	return func(next Handler) Handler {
		return func(req Request) Response {
			resp := next(req)
			return resp.
				WithHeader("Access-Control-Allow-Origin", allowOrigin).
				WithHeader("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS").
				WithHeader("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
	}
}
