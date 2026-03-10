// Package httpcore provides the thin Go HTTP bridge for the GALA server library.
// This is the only package that touches net/http. All types are Go primitives —
// GALA-native types (Request, Response, Method, StatusCode) live in the server package.
package httpcore

import (
	"io"
	"net/http"
)

// --- Bridge Types ---

// BridgeRequest carries raw HTTP request data from Go to GALA.
// Closures provide lazy access to path params, headers, and query params
// without exposing *http.Request.
type BridgeRequest struct {
	Method   string
	Path     string
	RawQuery string
	Body     string
	pathValue func(string) string
	headerGet func(string) string
	queryGet  func(string) string
}

func (r BridgeRequest) PathValue(name string) string {
	if r.pathValue == nil {
		return ""
	}
	return r.pathValue(name)
}

func (r BridgeRequest) HeaderValue(name string) string {
	if r.headerGet == nil {
		return ""
	}
	return r.headerGet(name)
}

func (r BridgeRequest) QueryValue(name string) string {
	if r.queryGet == nil {
		return ""
	}
	return r.queryGet(name)
}

// BridgeResponse carries raw HTTP response data from GALA back to Go.
type BridgeResponse struct {
	Status  int
	Body    string
	headers map[string]string
}

func NewBridgeResponse(status int, body string) *BridgeResponse {
	return &BridgeResponse{Status: status, Body: body, headers: make(map[string]string)}
}

func (r *BridgeResponse) SetHeader(key, value string) {
	r.headers[key] = value
}

// --- Server ---

// BridgeHandler processes a BridgeRequest and returns a *BridgeResponse.
type BridgeHandler = func(BridgeRequest) *BridgeResponse

// ServerBuilder accumulates routes for HTTP server construction.
type ServerBuilder struct {
	routes []route
}

type route struct {
	method  string
	pattern string
	handler BridgeHandler
}

func NewServerBuilder() *ServerBuilder { return &ServerBuilder{} }

func (sb *ServerBuilder) AddRoute(method, pattern string, handler BridgeHandler) {
	sb.routes = append(sb.routes, route{method, pattern, handler})
}

// ListenAndServe starts the HTTP server on the given address.
func (sb *ServerBuilder) ListenAndServe(addr string) error {
	mux := http.NewServeMux()

	for _, r := range sb.routes {
		h := r.handler
		pattern := r.method + " " + r.pattern
		mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
			body := ""
			if req.Body != nil {
				b, err := io.ReadAll(req.Body)
				if err == nil {
					body = string(b)
				}
				req.Body.Close()
			}

			br := BridgeRequest{
				Method:   req.Method,
				Path:     req.URL.Path,
				RawQuery: req.URL.RawQuery,
				Body:     body,
				pathValue: req.PathValue,
				headerGet: func(name string) string { return req.Header.Get(name) },
				queryGet:  func(name string) string { return req.URL.Query().Get(name) },
			}

			resp := h(br)
			for k, v := range resp.headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(resp.Status)
			if resp.Body != "" {
				w.Write([]byte(resp.Body))
			}
		})
	}

	return http.ListenAndServe(addr, mux)
}
