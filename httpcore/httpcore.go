// Package httpcore provides the thin Go HTTP bridge for the GALA server library.
// This is the only package that touches net/http. All types are Go primitives —
// GALA-native types (Request, Response, Method, StatusCode) live in the server package.
package httpcore

import (
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ============================================================================
// Bridge Types
// ============================================================================

// BridgeRequest carries raw HTTP request data from Go to GALA.
// Closures provide lazy access to path params, headers, and query params
// without exposing *http.Request.
type BridgeRequest struct {
	Method     string
	Path       string
	RawQuery   string
	Body       string
	pathValue  func(string) string
	headerGet  func(string) string
	queryGet   func(string) string
	formValue  func(string) string
	cookieGet  func(string) (string, bool)
	formFile   func(string) ([]byte, string, error)
	allHeaders func() map[string][]string
	allQuery   func() url.Values
	allForm    func() url.Values
	remoteAddr string
	host       string
	scheme     string
	ctxValues  *ContextValues
	isTLS      bool
	ctxDeadline func() (time.Time, bool)
	ctxDone     func() bool
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

func (r BridgeRequest) FormValue(name string) string {
	if r.formValue == nil {
		return ""
	}
	return r.formValue(name)
}

func (r BridgeRequest) CookieValue(name string) (string, bool) {
	if r.cookieGet == nil {
		return "", false
	}
	return r.cookieGet(name)
}

func (r BridgeRequest) FormFile(name string) ([]byte, string, error) {
	if r.formFile == nil {
		return nil, "", fmt.Errorf("no form file handler")
	}
	return r.formFile(name)
}

func (r BridgeRequest) AllHeaders() map[string][]string {
	if r.allHeaders == nil {
		return nil
	}
	return r.allHeaders()
}

func (r BridgeRequest) AllQueryParams() url.Values {
	if r.allQuery == nil {
		return nil
	}
	return r.allQuery()
}

func (r BridgeRequest) AllFormParams() url.Values {
	if r.allForm == nil {
		return nil
	}
	return r.allForm()
}

func (r BridgeRequest) RemoteAddr() string { return r.remoteAddr }
func (r BridgeRequest) Host() string       { return r.host }
func (r BridgeRequest) Scheme() string     { return r.scheme }
func (r BridgeRequest) IsTLS() bool        { return r.isTLS }
func (r BridgeRequest) CtxValues() *ContextValues { return r.ctxValues }

// ContextDeadline returns the Go context deadline if set.
func (r BridgeRequest) ContextDeadline() (time.Time, bool) {
	if r.ctxDeadline == nil {
		return time.Time{}, false
	}
	return r.ctxDeadline()
}

// ContextDone returns true if the Go context has been cancelled.
func (r BridgeRequest) ContextDone() bool {
	if r.ctxDone == nil {
		return false
	}
	return r.ctxDone()
}

// RealIP extracts the real client IP considering proxy headers.
func (r BridgeRequest) RealIP() string {
	// Check X-Real-IP first
	if ip := r.HeaderValue("X-Real-Ip"); ip != "" {
		return ip
	}
	// Check X-Forwarded-For
	if xff := r.HeaderValue("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	// Fall back to remote addr
	return ExtractIP(r.remoteAddr)
}

// IsWebSocket checks if the request is a WebSocket upgrade request.
func (r BridgeRequest) IsWebSocket() bool {
	upgrade := strings.ToLower(r.HeaderValue("Upgrade"))
	return upgrade == "websocket"
}

// ============================================================================
// Context Values (per-request key-value store)
// ============================================================================

type ContextValues struct {
	mu     sync.RWMutex
	values map[string]any
}

func NewContextValues() *ContextValues {
	return &ContextValues{values: make(map[string]any)}
}

func (cv *ContextValues) Set(key string, value any) {
	cv.mu.Lock()
	cv.values[key] = value
	cv.mu.Unlock()
}

func (cv *ContextValues) Get(key string) (any, bool) {
	cv.mu.RLock()
	v, ok := cv.values[key]
	cv.mu.RUnlock()
	return v, ok
}

func (cv *ContextValues) GetString(key string) (string, bool) {
	v, ok := cv.Get(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (cv *ContextValues) GetInt(key string) (int, bool) {
	v, ok := cv.Get(key)
	if !ok {
		return 0, false
	}
	i, ok := v.(int)
	return i, ok
}

// ============================================================================
// Bridge Response
// ============================================================================

type BridgeResponse struct {
	Status  int
	Body    string
	BodyBytes []byte // for binary responses (file, blob)
	headers map[string]string
	cookies []BridgeCookie
}

type BridgeCookie struct {
	Name     string
	Value    string
	Path     string
	Domain   string
	MaxAge   int
	Secure   bool
	HttpOnly bool
	SameSite string // "Strict", "Lax", "None", ""
}

func NewBridgeResponse(status int, body string) *BridgeResponse {
	return &BridgeResponse{Status: status, Body: body, headers: make(map[string]string)}
}

func NewBridgeResponseBytes(status int, body []byte, contentType string) *BridgeResponse {
	resp := &BridgeResponse{Status: status, BodyBytes: body, headers: make(map[string]string)}
	resp.headers["Content-Type"] = contentType
	return resp
}

func (r *BridgeResponse) SetHeader(key, value string) {
	r.headers[key] = value
}

func (r *BridgeResponse) AddCookie(c BridgeCookie) {
	r.cookies = append(r.cookies, c)
}

func (r *BridgeResponse) Headers() map[string]string {
	return r.headers
}

// ============================================================================
// SSE (Server-Sent Events) Response
// ============================================================================

// SSEEvent represents a single Server-Sent Event.
type SSEEvent struct {
	Event string
	Data  string
	Id    string
	Retry int
}

// NewSSEResponse formats an array of SSE events into a proper text/event-stream response.
func NewSSEResponse(events []SSEEvent) *BridgeResponse {
	var sb strings.Builder
	for _, e := range events {
		if e.Event != "" {
			sb.WriteString("event: ")
			sb.WriteString(e.Event)
			sb.WriteString("\n")
		}
		if e.Id != "" {
			sb.WriteString("id: ")
			sb.WriteString(e.Id)
			sb.WriteString("\n")
		}
		if e.Retry > 0 {
			sb.WriteString("retry: ")
			sb.WriteString(strconv.Itoa(e.Retry))
			sb.WriteString("\n")
		}
		// Data can be multi-line; each line must be prefixed with "data: "
		lines := strings.Split(e.Data, "\n")
		for _, line := range lines {
			sb.WriteString("data: ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n") // blank line separates events
	}

	resp := NewBridgeResponse(200, sb.String())
	resp.SetHeader("Content-Type", "text/event-stream")
	resp.SetHeader("Cache-Control", "no-cache")
	resp.SetHeader("Connection", "keep-alive")
	return resp
}

// ============================================================================
// Server Builder
// ============================================================================

type BridgeHandler = func(BridgeRequest) *BridgeResponse

type ServerBuilder struct {
	routes       []route
	statics      map[string]string // prefix -> dir
	errorHandler func(error) *BridgeResponse
	notFound     BridgeHandler // custom 404
	preFilters   []func(BridgeRequest) *BridgeRequest // pre-routing transforms
	server       *http.Server
}

type route struct {
	method  string
	pattern string
	handler BridgeHandler
}

func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{
		statics: make(map[string]string),
	}
}

func (sb *ServerBuilder) AddRoute(method, pattern string, handler BridgeHandler) {
	sb.routes = append(sb.routes, route{method, pattern, handler})
}

func (sb *ServerBuilder) AddStatic(prefix, dir string) {
	sb.statics[prefix] = dir
}

func (sb *ServerBuilder) SetErrorHandler(handler func(error) *BridgeResponse) {
	sb.errorHandler = handler
}

func (sb *ServerBuilder) SetNotFoundHandler(handler BridgeHandler) {
	sb.notFound = handler
}

func bridgeCookieToHTTP(bc BridgeCookie) *http.Cookie {
	c := &http.Cookie{
		Name:     bc.Name,
		Value:    bc.Value,
		Path:     bc.Path,
		Domain:   bc.Domain,
		MaxAge:   bc.MaxAge,
		Secure:   bc.Secure,
		HttpOnly: bc.HttpOnly,
	}
	switch strings.ToLower(bc.SameSite) {
	case "strict":
		c.SameSite = http.SameSiteStrictMode
	case "lax":
		c.SameSite = http.SameSiteLaxMode
	case "none":
		c.SameSite = http.SameSiteNoneMode
	}
	return c
}

// buildMux creates the http.ServeMux with all registered routes and static handlers.
func (sb *ServerBuilder) buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	// Register static file handlers
	for prefix, dir := range sb.statics {
		cleanPrefix := prefix
		if !strings.HasSuffix(cleanPrefix, "/") {
			cleanPrefix += "/"
		}
		fs := http.FileServer(http.Dir(dir))
		mux.Handle(cleanPrefix, http.StripPrefix(cleanPrefix, fs))
	}

	// Register API routes
	for _, r := range sb.routes {
		h := r.handler
		pattern := r.method + " " + r.pattern
		mux.HandleFunc(pattern, sb.makeHTTPHandler(h))
	}

	return mux
}

func (sb *ServerBuilder) makeHTTPHandler(h BridgeHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body := ""
		if req.Body != nil {
			b, err := io.ReadAll(req.Body)
			if err == nil {
				body = string(b)
			}
			req.Body.Close()
		}

		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		} else if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}

		ctxValues := NewContextValues()

		br := BridgeRequest{
			Method:   req.Method,
			Path:     req.URL.Path,
			RawQuery: req.URL.RawQuery,
			Body:     body,
			pathValue: req.PathValue,
			headerGet: func(name string) string { return req.Header.Get(name) },
			queryGet:  func(name string) string { return req.URL.Query().Get(name) },
			formValue: func(name string) string {
				if req.Form == nil {
					req.ParseForm()
				}
				return req.FormValue(name)
			},
			cookieGet: func(name string) (string, bool) {
				c, err := req.Cookie(name)
				if err != nil {
					return "", false
				}
				return c.Value, true
			},
			formFile: func(name string) ([]byte, string, error) {
				file, header, err := req.FormFile(name)
				if err != nil {
					return nil, "", err
				}
				defer file.Close()
				data, err := io.ReadAll(file)
				if err != nil {
					return nil, "", err
				}
				return data, header.Filename, nil
			},
			allHeaders: func() map[string][]string { return req.Header },
			allQuery:   func() url.Values { return req.URL.Query() },
			allForm: func() url.Values {
				if req.Form == nil {
					req.ParseForm()
				}
				return req.Form
			},
			remoteAddr: req.RemoteAddr,
			host:       req.Host,
			scheme:     scheme,
			isTLS:      req.TLS != nil,
			ctxValues:  ctxValues,
			ctxDeadline: func() (time.Time, bool) {
				return req.Context().Deadline()
			},
			ctxDone: func() bool {
				select {
				case <-req.Context().Done():
					return true
				default:
					return false
				}
			},
		}

		resp := h(br)
		sb.writeResponse(w, resp)
	}
}

func (sb *ServerBuilder) writeResponse(w http.ResponseWriter, resp *BridgeResponse) {
	for k, v := range resp.headers {
		w.Header().Set(k, v)
	}
	for _, c := range resp.cookies {
		http.SetCookie(w, bridgeCookieToHTTP(c))
	}
	w.WriteHeader(resp.Status)
	if resp.BodyBytes != nil {
		w.Write(resp.BodyBytes)
	} else if resp.Body != "" {
		w.Write([]byte(resp.Body))
	}
}

// ListenAndServe starts the HTTP server on the given address.
func (sb *ServerBuilder) ListenAndServe(addr string) error {
	mux := sb.buildMux()
	sb.server = &http.Server{Addr: addr, Handler: mux}
	return sb.server.ListenAndServe()
}

// ListenAndServeGraceful starts the HTTP server and shuts down gracefully on SIGINT/SIGTERM.
func (sb *ServerBuilder) ListenAndServeGraceful(addr string, shutdownTimeout time.Duration) error {
	mux := sb.buildMux()
	sb.server = &http.Server{Addr: addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		if err := sb.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		fmt.Println("\nShutdown signal received, draining connections...")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return sb.server.Shutdown(ctx)
	}
}

// ListenAndServeTLS starts the HTTPS server with TLS on the given address.
func (sb *ServerBuilder) ListenAndServeTLS(addr, certFile, keyFile string) error {
	mux := sb.buildMux()
	sb.server = &http.Server{Addr: addr, Handler: mux}
	return sb.server.ListenAndServeTLS(certFile, keyFile)
}

// ListenAndServeTLSGraceful starts the HTTPS server with TLS and graceful shutdown.
func (sb *ServerBuilder) ListenAndServeTLSGraceful(addr, certFile, keyFile string, shutdownTimeout time.Duration) error {
	mux := sb.buildMux()
	sb.server = &http.Server{Addr: addr, Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		if err := sb.server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		fmt.Println("\nShutdown signal received, draining connections...")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return sb.server.Shutdown(ctx)
	}
}

// ListenAndServeWithStatic is kept for backward compatibility.
// New code should use AddStatic() then ListenAndServe().
func (sb *ServerBuilder) ListenAndServeWithStatic(addr string, statics map[string]string) error {
	for k, v := range statics {
		sb.statics[k] = v
	}
	return sb.ListenAndServe(addr)
}

// Shutdown gracefully shuts down the server.
func (sb *ServerBuilder) Shutdown(timeout time.Duration) error {
	if sb.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return sb.server.Shutdown(ctx)
}

// ============================================================================
// Request Cloning (for MethodOverride, Decompress)
// ============================================================================

// CloneBridgeRequestWithMethod creates a copy of a BridgeRequest with a different HTTP method.
func CloneBridgeRequestWithMethod(br BridgeRequest, method string) BridgeRequest {
	return BridgeRequest{
		Method:     method,
		Path:       br.Path,
		RawQuery:   br.RawQuery,
		Body:       br.Body,
		pathValue:  br.pathValue,
		headerGet:  br.headerGet,
		queryGet:   br.queryGet,
		formValue:  br.formValue,
		cookieGet:  br.cookieGet,
		formFile:   br.formFile,
		allHeaders: br.allHeaders,
		allQuery:   br.allQuery,
		allForm:    br.allForm,
		remoteAddr: br.remoteAddr,
		host:       br.host,
		scheme:     br.scheme,
		ctxValues:  br.ctxValues,
		isTLS:      br.isTLS,
		ctxDeadline: br.ctxDeadline,
		ctxDone:     br.ctxDone,
	}
}

// CloneBridgeRequestWithBody creates a copy of a BridgeRequest with a different body.
func CloneBridgeRequestWithBody(br BridgeRequest, body string) BridgeRequest {
	return BridgeRequest{
		Method:     br.Method,
		Path:       br.Path,
		RawQuery:   br.RawQuery,
		Body:       body,
		pathValue:  br.pathValue,
		headerGet:  br.headerGet,
		queryGet:   br.queryGet,
		formValue:  br.formValue,
		cookieGet:  br.cookieGet,
		formFile:   br.formFile,
		allHeaders: br.allHeaders,
		allQuery:   br.allQuery,
		allForm:    br.allForm,
		remoteAddr: br.remoteAddr,
		host:       br.host,
		scheme:     br.scheme,
		ctxValues:  br.ctxValues,
		isTLS:      br.isTLS,
		ctxDeadline: br.ctxDeadline,
		ctxDone:     br.ctxDone,
	}
}

// ============================================================================
// Atomic Counter (for RequestId)
// ============================================================================

type AtomicCounter struct {
	val int64
}

func NewAtomicCounter() *AtomicCounter {
	return &AtomicCounter{}
}

func (c *AtomicCounter) Next() int64 {
	return atomic.AddInt64(&c.val, 1)
}

// ============================================================================
// Rate Limiter (Token Bucket)
// ============================================================================

type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time
}

func NewTokenBucket(maxTokens float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

// --- Per-IP Rate Limiter ---

type PerIPRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*TokenBucket
	max      float64
	rate     float64
}

func NewPerIPRateLimiter(maxTokens float64, refillRate float64) *PerIPRateLimiter {
	return &PerIPRateLimiter{
		limiters: make(map[string]*TokenBucket),
		max:      maxTokens,
		rate:     refillRate,
	}
}

func (pl *PerIPRateLimiter) AllowIP(ip string) bool {
	pl.mu.RLock()
	limiter, ok := pl.limiters[ip]
	pl.mu.RUnlock()

	if !ok {
		pl.mu.Lock()
		limiter, ok = pl.limiters[ip]
		if !ok {
			limiter = NewTokenBucket(pl.max, pl.rate)
			pl.limiters[ip] = limiter
		}
		pl.mu.Unlock()
	}

	return limiter.Allow()
}

// ============================================================================
// IP Extraction
// ============================================================================

func ExtractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// ============================================================================
// Gzip Compression
// ============================================================================

func CompressGzip(body string) ([]byte, error) {
	var buf strings.Builder
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(body))
	if err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func AcceptsGzip(acceptEncoding string) bool {
	return strings.Contains(acceptEncoding, "gzip")
}

// DecompressGzip decompresses a gzip-encoded string body.
// Returns the decompressed string, or empty string on error.
func DecompressGzip(body string) string {
	reader, err := gzip.NewReader(strings.NewReader(body))
	if err != nil {
		return ""
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return ""
	}
	return string(data)
}

// ============================================================================
// Basic Auth Decoder
// ============================================================================

type BasicAuthResult struct {
	Username string
	Password string
	Valid    bool
}

func DecodeBasicAuth(header string) BasicAuthResult {
	if !strings.HasPrefix(header, "Basic ") {
		return BasicAuthResult{Valid: false}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(header, "Basic "))
	if err != nil {
		return BasicAuthResult{Valid: false}
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return BasicAuthResult{Valid: false}
	}
	return BasicAuthResult{Username: parts[0], Password: parts[1], Valid: true}
}

// ============================================================================
// HMAC Signer (for CSRF tokens)
// ============================================================================

type HMACSigner struct {
	key []byte
}

func NewHMACSigner(secret string) *HMACSigner {
	return &HMACSigner{key: []byte(secret)}
}

func (s *HMACSigner) GenerateToken() string {
	// Generate 32 random bytes
	token := make([]byte, 32)
	rand.Read(token)
	tokenHex := hex.EncodeToString(token)

	// Sign it
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(tokenHex))
	sig := hex.EncodeToString(mac.Sum(nil))

	return tokenHex + "." + sig
}

func (s *HMACSigner) ValidateToken(token string) bool {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(parts[1]), []byte(expected))
}

// ============================================================================
// Byte Size Parser (for BodyLimit)
// ============================================================================

func ParseByteSize(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	// Find where the numeric part ends
	var i int
	for i = 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			break
		}
	}

	if i == 0 {
		return 0
	}

	num, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0
	}

	unit := strings.ToUpper(strings.TrimSpace(s[i:]))
	switch unit {
	case "", "B":
		return num
	case "K", "KB":
		return num * 1024
	case "M", "MB":
		return num * 1024 * 1024
	case "G", "GB":
		return num * 1024 * 1024 * 1024
	default:
		return num
	}
}

// ============================================================================
// URL Rewrite Helpers
// ============================================================================

// MatchRewriteRule checks if a path matches a rewrite rule pattern.
// Supports * as a wildcard matching one or more path segments.
func MatchRewriteRule(path, pattern string) bool {
	if !strings.Contains(pattern, "*") {
		return path == pattern
	}
	parts := strings.SplitN(pattern, "*", 2)
	prefix := parts[0]
	return strings.HasPrefix(path, prefix)
}

// ApplyRewriteRule replaces a path using a rewrite rule.
// $1 in the target is replaced with the wildcard-matched portion.
func ApplyRewriteRule(path, pattern, target string) string {
	if !strings.Contains(pattern, "*") {
		return target
	}
	prefix := strings.SplitN(pattern, "*", 2)[0]
	captured := strings.TrimPrefix(path, prefix)
	return strings.ReplaceAll(target, "$1", captured)
}

// ============================================================================
// File Serving Helpers
// ============================================================================

// ReadFile reads a file and returns its contents and content type.
func ReadFile(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	ct := mime.TypeByExtension(filepath.Ext(path))
	if ct == "" {
		ct = "application/octet-stream"
	}
	return data, ct, nil
}

// ============================================================================
// URL Query Parsing
// ============================================================================

func ParseQueryValues(rawQuery string) url.Values {
	v, _ := url.ParseQuery(rawQuery)
	return v
}

func QueryValues(v url.Values, key string) []string {
	return v[key]
}

// ============================================================================
// Reverse Proxy Helper
// ============================================================================

// ProxyConfig holds configuration for reverse proxy requests.
type ProxyConfig struct {
	TargetBaseURL string
	Timeout       time.Duration
}

// ProxyResult holds the result of a proxied request.
type ProxyResult struct {
	Status  int
	Body    string
	Headers map[string]string
	Error   string
}

// ProxyRequest forwards a request to the target URL and returns the response.
func ProxyRequest(config *ProxyConfig, method string, path string, rawQuery string, body string, headers map[string]string) ProxyResult {
	targetURL := config.TargetBaseURL + path
	if rawQuery != "" {
		targetURL += "?" + rawQuery
	}

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, targetURL, reqBody)
	if err != nil {
		return ProxyResult{Error: "failed to create proxy request: " + err.Error()}
	}

	// Forward headers (skip hop-by-hop headers)
	hopByHop := map[string]bool{
		"connection": true, "keep-alive": true, "proxy-authenticate": true,
		"proxy-authorization": true, "te": true, "trailers": true,
		"transfer-encoding": true, "upgrade": true,
	}
	for k, v := range headers {
		if !hopByHop[strings.ToLower(k)] {
			req.Header.Set(k, v)
		}
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		return ProxyResult{Error: "proxy request failed: " + err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProxyResult{Error: "failed to read proxy response: " + err.Error()}
	}

	respHeaders := make(map[string]string)
	for k := range resp.Header {
		if !hopByHop[strings.ToLower(k)] {
			respHeaders[k] = resp.Header.Get(k)
		}
	}

	return ProxyResult{
		Status:  resp.StatusCode,
		Body:    string(respBody),
		Headers: respHeaders,
	}
}

// NewProxyConfig creates a proxy configuration with the given target URL.
func NewProxyConfig(targetBaseURL string) *ProxyConfig {
	return &ProxyConfig{
		TargetBaseURL: strings.TrimRight(targetBaseURL, "/"),
		Timeout:       30 * time.Second,
	}
}

// NewProxyConfigWithTimeout creates a proxy configuration with custom timeout.
func NewProxyConfigWithTimeout(targetBaseURL string, timeout time.Duration) *ProxyConfig {
	return &ProxyConfig{
		TargetBaseURL: strings.TrimRight(targetBaseURL, "/"),
		Timeout:       timeout,
	}
}

// ProxyRequestHeaders extracts a flat map of request headers from BridgeRequest.
func ProxyRequestHeaders(br BridgeRequest) map[string]string {
	result := make(map[string]string)
	allHeaders := br.AllHeaders()
	for k, v := range allHeaders {
		if len(v) > 0 {
			result[k] = v[0]
		}
	}
	return result
}

// ProxyResultHeaderKeys returns the list of header keys from a ProxyResult.
func ProxyResultHeaderKeys(r ProxyResult) []string {
	keys := make([]string, 0, len(r.Headers))
	for k := range r.Headers {
		keys = append(keys, k)
	}
	return keys
}

// ProxyResultHeaderValue returns a header value from a ProxyResult.
func ProxyResultHeaderValue(r ProxyResult, key string) string {
	return r.Headers[key]
}

// ============================================================================
// JWT Helpers (HMAC-SHA256)
// ============================================================================

// JWTClaims holds the decoded JWT payload as raw key-value pairs.
type JWTClaims struct {
	values map[string]any
}

// Get returns a claim value as a string, or empty string if not found.
func (c *JWTClaims) Get(key string) string {
	v, ok := c.values[key]
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetFloat returns a claim value as float64 (useful for exp, iat, nbf).
func (c *JWTClaims) GetFloat(key string) (float64, bool) {
	v, ok := c.values[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

// Has checks if a claim key exists.
func (c *JWTClaims) Has(key string) bool {
	_, ok := c.values[key]
	return ok
}

// JWTResult holds the result of JWT validation.
type JWTResult struct {
	Valid  bool
	Claims *JWTClaims
	Error  string
}

// ParseAndValidateJWT validates an HMAC-SHA256 (HS256) JWT token.
// It verifies the signature and optionally checks expiration.
// Returns JWTResult with claims on success, or error details on failure.
func ParseAndValidateJWT(tokenString string, secret string, checkExpiry bool) JWTResult {
	parts := strings.SplitN(tokenString, ".", 3)
	if len(parts) != 3 {
		return JWTResult{Valid: false, Error: "malformed token"}
	}

	headerB64, payloadB64, signatureB64 := parts[0], parts[1], parts[2]

	// Decode header to verify algorithm
	headerJSON, err := base64URLDecode(headerB64)
	if err != nil {
		return JWTResult{Valid: false, Error: "invalid header encoding"}
	}

	// Simple JSON parsing for alg field
	alg := extractJSONString(headerJSON, "alg")
	if alg != "HS256" {
		return JWTResult{Valid: false, Error: "unsupported algorithm: " + alg}
	}

	// Verify signature
	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64URLDecode(signatureB64)
	if err != nil {
		return JWTResult{Valid: false, Error: "invalid signature encoding"}
	}

	if !hmac.Equal(expectedSig, []byte(actualSig)) {
		return JWTResult{Valid: false, Error: "invalid signature"}
	}

	// Decode payload
	payloadJSON, err := base64URLDecode(payloadB64)
	if err != nil {
		return JWTResult{Valid: false, Error: "invalid payload encoding"}
	}

	claims := parseJSONClaims(payloadJSON)

	// Check expiration if requested
	if checkExpiry {
		if exp, ok := claims.GetFloat("exp"); ok {
			if float64(time.Now().Unix()) > exp {
				return JWTResult{Valid: false, Claims: claims, Error: "token expired"}
			}
		}
	}

	return JWTResult{Valid: true, Claims: claims}
}

// CreateJWT creates an HMAC-SHA256 signed JWT token from claims.
func CreateJWT(claimsJSON string, secret string) string {
	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64URLEncode([]byte(claimsJSON))

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	signature := base64URLEncode(mac.Sum(nil))

	return signingInput + "." + signature
}

func base64URLDecode(s string) (string, error) {
	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// extractJSONString extracts a string value from a JSON object by key.
// Simple extraction without a full JSON parser.
func extractJSONString(json, key string) string {
	search := `"` + key + `"`
	idx := strings.Index(json, search)
	if idx < 0 {
		return ""
	}
	rest := json[idx+len(search):]
	// Skip whitespace and colon
	rest = strings.TrimLeft(rest, " \t\n\r:")
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// parseJSONClaims does minimal JSON parsing for JWT claims.
// Handles string and numeric values.
func parseJSONClaims(json string) *JWTClaims {
	claims := &JWTClaims{values: make(map[string]any)}
	json = strings.TrimSpace(json)
	if len(json) < 2 || json[0] != '{' {
		return claims
	}
	json = json[1 : len(json)-1] // strip { }

	for len(json) > 0 {
		json = strings.TrimLeft(json, " \t\n\r,")
		if len(json) == 0 || json[0] != '"' {
			break
		}

		// Extract key
		json = json[1:]
		keyEnd := strings.Index(json, `"`)
		if keyEnd < 0 {
			break
		}
		key := json[:keyEnd]
		json = json[keyEnd+1:]

		// Skip colon
		json = strings.TrimLeft(json, " \t\n\r:")

		if len(json) == 0 {
			break
		}

		// Extract value
		if json[0] == '"' {
			// String value
			json = json[1:]
			valEnd := strings.Index(json, `"`)
			if valEnd < 0 {
				break
			}
			claims.values[key] = json[:valEnd]
			json = json[valEnd+1:]
		} else if json[0] == '-' || (json[0] >= '0' && json[0] <= '9') {
			// Number value
			end := 0
			for end < len(json) && json[end] != ',' && json[end] != '}' && json[end] != ' ' {
				end++
			}
			numStr := json[:end]
			if f, err := strconv.ParseFloat(numStr, 64); err == nil {
				claims.values[key] = f
			}
			json = json[end:]
		} else if strings.HasPrefix(json, "true") {
			claims.values[key] = "true"
			json = json[4:]
		} else if strings.HasPrefix(json, "false") {
			claims.values[key] = "false"
			json = json[5:]
		} else if strings.HasPrefix(json, "null") {
			json = json[4:]
		} else {
			// Skip unknown value types (arrays, nested objects)
			depth := 0
			end := 0
			for end < len(json) {
				if json[end] == '{' || json[end] == '[' {
					depth++
				} else if json[end] == '}' || json[end] == ']' {
					if depth == 0 {
						break
					}
					depth--
				} else if json[end] == ',' && depth == 0 {
					break
				}
				end++
			}
			json = json[end:]
		}
	}

	return claims
}

// ============================================================================
// Circuit Breaker (state machine for resilience)
// ============================================================================

// CircuitBreakerState represents the circuit breaker's current state.
type CircuitBreakerState int

const (
	CircuitClosed   CircuitBreakerState = iota // normal operation
	CircuitOpen                                // rejecting requests
	CircuitHalfOpen                            // testing recovery
)

// CircuitBreaker implements a circuit breaker pattern with atomic state management.
type CircuitBreaker struct {
	mu            sync.Mutex
	state         CircuitBreakerState
	failures      int
	maxFailures   int
	resetTimeout  time.Duration
	halfOpenMax   int
	halfOpenCount int
	lastFailure   time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration, halfOpenMax int) *CircuitBreaker {
	return &CircuitBreaker{
		state:        CircuitClosed,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  halfOpenMax,
	}
}

// Allow checks if a request should be allowed through the circuit breaker.
// Returns true if the request is allowed, false if the circuit is open.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if reset timeout has elapsed
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.halfOpenCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		if cb.halfOpenCount < cb.halfOpenMax {
			cb.halfOpenCount++
			return true
		}
		return false
	}
	return false
}

// RecordSuccess records a successful request, potentially closing the circuit.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.failures = 0
		cb.halfOpenCount = 0
	} else if cb.state == CircuitClosed {
		cb.failures = 0
	}
}

// RecordFailure records a failed request, potentially opening the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
	} else if cb.state == CircuitClosed && cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// ============================================================================
// Bulkhead (Concurrency Limiter via semaphore)
// ============================================================================

// Semaphore implements a counting semaphore using a buffered channel.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a new semaphore with the given capacity.
func NewSemaphore(maxConcurrent int) *Semaphore {
	return &Semaphore{ch: make(chan struct{}, maxConcurrent)}
}

// TryAcquire attempts to acquire a slot without blocking.
// Returns true if a slot was acquired, false if at capacity.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases a slot back to the semaphore.
func (s *Semaphore) Release() {
	<-s.ch
}

// ============================================================================
// ETag Helper
// ============================================================================

// ComputeETag computes an ETag from the given body using SHA256 truncated to 16 hex chars.
func ComputeETag(body string) string {
	h := sha256.Sum256([]byte(body))
	return `"` + hex.EncodeToString(h[:8]) + `"`
}

// ============================================================================
// Metrics Collector (Finatra-inspired stats)
// ============================================================================

// MetricsCollector tracks per-route request counts, latency, and status codes.
// Thread-safe via atomic operations and mutexes.
type MetricsCollector struct {
	mu           sync.RWMutex
	totalReqs    int64
	routeMetrics map[string]*RouteMetrics
	statusCounts map[int]*int64 // status code -> count
	startTime    time.Time
}

// RouteMetrics holds metrics for a single route pattern.
type RouteMetrics struct {
	Count      int64
	TotalNs    int64 // total latency in nanoseconds
	MinNs      int64
	MaxNs      int64
	StatusHist map[int]*int64 // per-route status histogram
	mu         sync.Mutex
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		routeMetrics: make(map[string]*RouteMetrics),
		statusCounts: make(map[int]*int64),
		startTime:    time.Now(),
	}
}

// Record records a request's metrics.
func (mc *MetricsCollector) Record(method, path string, status int, durationNs int64) {
	atomic.AddInt64(&mc.totalReqs, 1)

	// Global status count
	mc.mu.RLock()
	counter, ok := mc.statusCounts[status]
	mc.mu.RUnlock()
	if !ok {
		mc.mu.Lock()
		counter, ok = mc.statusCounts[status]
		if !ok {
			var c int64
			counter = &c
			mc.statusCounts[status] = counter
		}
		mc.mu.Unlock()
	}
	atomic.AddInt64(counter, 1)

	// Per-route metrics
	key := method + " " + path
	mc.mu.RLock()
	rm, ok := mc.routeMetrics[key]
	mc.mu.RUnlock()
	if !ok {
		mc.mu.Lock()
		rm, ok = mc.routeMetrics[key]
		if !ok {
			rm = &RouteMetrics{
				MinNs:      durationNs,
				MaxNs:      durationNs,
				StatusHist: make(map[int]*int64),
			}
			mc.routeMetrics[key] = rm
		}
		mc.mu.Unlock()
	}

	atomic.AddInt64(&rm.Count, 1)
	atomic.AddInt64(&rm.TotalNs, durationNs)

	rm.mu.Lock()
	if durationNs < rm.MinNs {
		rm.MinNs = durationNs
	}
	if durationNs > rm.MaxNs {
		rm.MaxNs = durationNs
	}
	sc, ok := rm.StatusHist[status]
	if !ok {
		var c int64
		sc = &c
		rm.StatusHist[status] = sc
	}
	*sc++
	rm.mu.Unlock()
}

// TotalRequests returns the total number of recorded requests.
func (mc *MetricsCollector) TotalRequests() int64 {
	return atomic.LoadInt64(&mc.totalReqs)
}

// UptimeSeconds returns the server uptime in seconds.
func (mc *MetricsCollector) UptimeSeconds() float64 {
	return time.Since(mc.startTime).Seconds()
}

// PrometheusFormat returns all metrics in Prometheus text exposition format.
func (mc *MetricsCollector) PrometheusFormat() string {
	var sb strings.Builder

	total := atomic.LoadInt64(&mc.totalReqs)
	uptime := mc.UptimeSeconds()

	sb.WriteString("# HELP gala_requests_total Total number of HTTP requests.\n")
	sb.WriteString("# TYPE gala_requests_total counter\n")
	sb.WriteString(fmt.Sprintf("gala_requests_total %d\n\n", total))

	sb.WriteString("# HELP gala_uptime_seconds Server uptime in seconds.\n")
	sb.WriteString("# TYPE gala_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("gala_uptime_seconds %.1f\n\n", uptime))

	// Global status code counts
	sb.WriteString("# HELP gala_responses_total Total responses by status code.\n")
	sb.WriteString("# TYPE gala_responses_total counter\n")
	mc.mu.RLock()
	for code, counter := range mc.statusCounts {
		sb.WriteString(fmt.Sprintf("gala_responses_total{status=\"%d\"} %d\n", code, atomic.LoadInt64(counter)))
	}
	mc.mu.RUnlock()
	sb.WriteString("\n")

	// Per-route metrics
	sb.WriteString("# HELP gala_route_requests_total Requests per route.\n")
	sb.WriteString("# TYPE gala_route_requests_total counter\n")
	mc.mu.RLock()
	for route, rm := range mc.routeMetrics {
		count := atomic.LoadInt64(&rm.Count)
		sb.WriteString(fmt.Sprintf("gala_route_requests_total{route=\"%s\"} %d\n", route, count))
	}
	mc.mu.RUnlock()
	sb.WriteString("\n")

	sb.WriteString("# HELP gala_route_latency_avg_ms Average latency per route in milliseconds.\n")
	sb.WriteString("# TYPE gala_route_latency_avg_ms gauge\n")
	mc.mu.RLock()
	for route, rm := range mc.routeMetrics {
		count := atomic.LoadInt64(&rm.Count)
		totalNs := atomic.LoadInt64(&rm.TotalNs)
		if count > 0 {
			avgMs := float64(totalNs) / float64(count) / 1e6
			sb.WriteString(fmt.Sprintf("gala_route_latency_avg_ms{route=\"%s\"} %.3f\n", route, avgMs))
		}
	}
	mc.mu.RUnlock()
	sb.WriteString("\n")

	sb.WriteString("# HELP gala_route_latency_max_ms Max latency per route in milliseconds.\n")
	sb.WriteString("# TYPE gala_route_latency_max_ms gauge\n")
	mc.mu.RLock()
	for route, rm := range mc.routeMetrics {
		rm.mu.Lock()
		maxMs := float64(rm.MaxNs) / 1e6
		rm.mu.Unlock()
		sb.WriteString(fmt.Sprintf("gala_route_latency_max_ms{route=\"%s\"} %.3f\n", route, maxMs))
	}
	mc.mu.RUnlock()

	return sb.String()
}

// ============================================================================
// Session Store (concurrent in-memory sessions)
// ============================================================================

// SessionStore manages in-memory sessions with cookie-based IDs.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionData
	secret   string
}

// SessionData holds per-session key-value pairs.
type SessionData struct {
	mu        sync.RWMutex
	values    map[string]string
	createdAt time.Time
	lastAccess time.Time
}

// NewSessionStore creates a new in-memory session store.
func NewSessionStore(secret string) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*SessionData),
		secret:   secret,
	}
}

// GenerateSessionID creates a cryptographically random session ID.
func (ss *SessionStore) GenerateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetOrCreate returns the session for the given ID, creating one if it doesn't exist.
// Returns (session, sessionID, isNew).
func (ss *SessionStore) GetOrCreate(sessionID string) (*SessionData, string, bool) {
	if sessionID != "" {
		ss.mu.RLock()
		session, ok := ss.sessions[sessionID]
		ss.mu.RUnlock()
		if ok {
			session.mu.Lock()
			session.lastAccess = time.Now()
			session.mu.Unlock()
			return session, sessionID, false
		}
	}

	// Create new session
	newID := ss.GenerateSessionID()
	session := &SessionData{
		values:     make(map[string]string),
		createdAt:  time.Now(),
		lastAccess: time.Now(),
	}
	ss.mu.Lock()
	ss.sessions[newID] = session
	ss.mu.Unlock()
	return session, newID, true
}

// Get returns a session value.
func (sd *SessionData) Get(key string) (string, bool) {
	sd.mu.RLock()
	v, ok := sd.values[key]
	sd.mu.RUnlock()
	return v, ok
}

// Set sets a session value.
func (sd *SessionData) Set(key, value string) {
	sd.mu.Lock()
	sd.values[key] = value
	sd.mu.Unlock()
}

// Delete removes a session value.
func (sd *SessionData) Delete(key string) {
	sd.mu.Lock()
	delete(sd.values, key)
	sd.mu.Unlock()
}

// Destroy removes a session from the store.
func (ss *SessionStore) Destroy(sessionID string) {
	ss.mu.Lock()
	delete(ss.sessions, sessionID)
	ss.mu.Unlock()
}

// SessionCount returns the number of active sessions.
func (ss *SessionStore) SessionCount() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.sessions)
}

// SessionFromContext retrieves a SessionData from a ContextValues store.
// Note: Go bridge helper required due to transpiler BUG-055 (pointer type in typed pattern match).
func SessionFromContext(cv *ContextValues) (*SessionData, bool) {
	raw, ok := cv.Get("_session")
	if !ok {
		return nil, false
	}
	sd, ok := raw.(*SessionData)
	return sd, ok
}

// Cleanup removes sessions older than maxAge.
func (ss *SessionStore) Cleanup(maxAge time.Duration) int {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	now := time.Now()
	removed := 0
	for id, session := range ss.sessions {
		session.mu.RLock()
		lastAccess := session.lastAccess
		session.mu.RUnlock()
		if now.Sub(lastAccess) > maxAge {
			delete(ss.sessions, id)
			removed++
		}
	}
	return removed
}

// ============================================================================
// Sliding Window Rate Limiter
// ============================================================================

// SlidingWindowLimiter implements a sliding window counter rate limiter.
// More accurate than token bucket for bursty traffic.
type SlidingWindowLimiter struct {
	mu          sync.Mutex
	maxRequests int
	windowSize  time.Duration
	timestamps  []time.Time
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter.
func NewSlidingWindowLimiter(maxRequests int, windowSize time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		maxRequests: maxRequests,
		windowSize:  windowSize,
		timestamps:  make([]time.Time, 0, maxRequests),
	}
}

// Allow checks if a request should be allowed and records it if so.
func (sw *SlidingWindowLimiter) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.windowSize)

	// Remove expired timestamps
	valid := 0
	for _, ts := range sw.timestamps {
		if ts.After(cutoff) {
			sw.timestamps[valid] = ts
			valid++
		}
	}
	sw.timestamps = sw.timestamps[:valid]

	if len(sw.timestamps) >= sw.maxRequests {
		return false
	}

	sw.timestamps = append(sw.timestamps, now)
	return true
}

// PerIPSlidingWindowLimiter implements per-IP sliding window rate limiting.
type PerIPSlidingWindowLimiter struct {
	mu          sync.RWMutex
	limiters    map[string]*SlidingWindowLimiter
	maxRequests int
	windowSize  time.Duration
}

// NewPerIPSlidingWindowLimiter creates a per-IP sliding window rate limiter.
func NewPerIPSlidingWindowLimiter(maxRequests int, windowSize time.Duration) *PerIPSlidingWindowLimiter {
	return &PerIPSlidingWindowLimiter{
		limiters:    make(map[string]*SlidingWindowLimiter),
		maxRequests: maxRequests,
		windowSize:  windowSize,
	}
}

// AllowIP checks if a request from the given IP should be allowed.
func (pl *PerIPSlidingWindowLimiter) AllowIP(ip string) bool {
	pl.mu.RLock()
	limiter, ok := pl.limiters[ip]
	pl.mu.RUnlock()

	if !ok {
		pl.mu.Lock()
		limiter, ok = pl.limiters[ip]
		if !ok {
			limiter = NewSlidingWindowLimiter(pl.maxRequests, pl.windowSize)
			pl.limiters[ip] = limiter
		}
		pl.mu.Unlock()
	}

	return limiter.Allow()
}
