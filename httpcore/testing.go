package httpcore

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ============================================================================
// Test Request Builder
// ============================================================================

// TestRequestBuilder helps construct BridgeRequest instances for testing.
// Uses a mutable builder pattern — call Build() to produce the final BridgeRequest.
type TestRequestBuilder struct {
	method     string
	path       string
	body       string
	rawQuery   string
	headers    map[string]string
	pathParams map[string]string
	cookies    map[string]string
	formParams map[string]string
	remoteAddr string
	host       string
	scheme     string
	isTLS      bool
}

// NewTestRequestBuilder creates a builder for test BridgeRequests.
func NewTestRequestBuilder(method, path string) *TestRequestBuilder {
	return &TestRequestBuilder{
		method:     method,
		path:       path,
		headers:    make(map[string]string),
		pathParams: make(map[string]string),
		cookies:    make(map[string]string),
		formParams: make(map[string]string),
		remoteAddr: "127.0.0.1:1234",
		host:       "localhost",
		scheme:     "http",
	}
}

func (b *TestRequestBuilder) WithBody(body string) *TestRequestBuilder {
	b.body = body
	return b
}

func (b *TestRequestBuilder) WithRawQuery(q string) *TestRequestBuilder {
	b.rawQuery = q
	return b
}

func (b *TestRequestBuilder) WithHeader(key, value string) *TestRequestBuilder {
	b.headers[key] = value
	return b
}

func (b *TestRequestBuilder) WithPathParam(key, value string) *TestRequestBuilder {
	b.pathParams[key] = value
	return b
}

func (b *TestRequestBuilder) WithCookie(key, value string) *TestRequestBuilder {
	b.cookies[key] = value
	return b
}

func (b *TestRequestBuilder) WithFormParam(key, value string) *TestRequestBuilder {
	b.formParams[key] = value
	return b
}

func (b *TestRequestBuilder) WithRemoteAddr(addr string) *TestRequestBuilder {
	b.remoteAddr = addr
	return b
}

func (b *TestRequestBuilder) WithHost(host string) *TestRequestBuilder {
	b.host = host
	return b
}

func (b *TestRequestBuilder) WithScheme(scheme string) *TestRequestBuilder {
	b.scheme = scheme
	return b
}

func (b *TestRequestBuilder) WithTLS(tls bool) *TestRequestBuilder {
	b.isTLS = tls
	return b
}

// Build produces the final BridgeRequest.
func (b *TestRequestBuilder) Build() BridgeRequest {
	headers := b.headers
	pathParams := b.pathParams
	cookies := b.cookies
	formParams := b.formParams
	rawQuery := b.rawQuery
	ctx := NewContextValues()

	return BridgeRequest{
		Method:   b.method,
		Path:     b.path,
		Body:     b.body,
		RawQuery: rawQuery,
		pathValue: func(name string) string { return pathParams[name] },
		headerGet: func(name string) string { return headers[name] },
		queryGet: func(name string) string {
			vals, err := url.ParseQuery(rawQuery)
			if err != nil {
				return ""
			}
			return vals.Get(name)
		},
		formValue: func(name string) string { return formParams[name] },
		cookieGet: func(name string) (string, bool) {
			v, ok := cookies[name]
			return v, ok
		},
		formFile: func(name string) ([]byte, string, error) {
			return nil, "", fmt.Errorf("no file in test request")
		},
		allHeaders: func() map[string][]string { return nil },
		allQuery: func() url.Values {
			vals, _ := url.ParseQuery(rawQuery)
			return vals
		},
		allForm:    func() url.Values { return nil },
		remoteAddr: b.remoteAddr,
		host:       b.host,
		scheme:     b.scheme,
		ctxValues:  ctx,
		isTLS:      b.isTLS,
	}
}

// ============================================================================
// Test Helpers
// ============================================================================

// EncodeBasicAuth creates a "Basic <base64>" Authorization header value.
func EncodeBasicAuth(username, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

// ToBytes converts a string to []byte. Helper for GALA code that cannot use Go type conversions.
func ToBytes(s string) []byte {
	return []byte(s)
}

// ============================================================================
// Integration Test HTTP Helpers
// ============================================================================

// HTTPResult holds the result of an HTTP request for testing.
type HTTPResult struct {
	Status int
	Body   string
}

// HTTPGet performs a GET request and returns status + body.
func HTTPGet(url string) HTTPResult {
	resp, err := http.Get(url)
	if err != nil {
		return HTTPResult{0, "error: " + err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return HTTPResult{resp.StatusCode, string(body)}
}

// HTTPPost performs a POST request and returns status + body.
func HTTPPost(url, contentType, body string) HTTPResult {
	resp, err := http.Post(url, contentType, strings.NewReader(body))
	if err != nil {
		return HTTPResult{0, "error: " + err.Error()}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return HTTPResult{resp.StatusCode, string(respBody)}
}

// HTTPGetWithHeader performs a GET with a custom header.
func HTTPGetWithHeader(url, key, value string) HTTPResult {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return HTTPResult{0, "error: " + err.Error()}
	}
	req.Header.Set(key, value)
	resp, err := client.Do(req)
	if err != nil {
		return HTTPResult{0, "error: " + err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return HTTPResult{resp.StatusCode, string(body)}
}

// HTTPDoRequest performs a request with the given method and returns the full response info.
type HTTPFullResult struct {
	Status  int
	Body    string
	Headers map[string]string
	Cookies []*http.Cookie
}

// HTTPDo performs a request and returns full result including headers/cookies.
func HTTPDo(method, url string) HTTPFullResult {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return HTTPFullResult{Status: 0, Body: "error: " + err.Error()}
	}
	resp, err := client.Do(req)
	if err != nil {
		return HTTPFullResult{Status: 0, Body: "error: " + err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}
	return HTTPFullResult{
		Status:  resp.StatusCode,
		Body:    string(body),
		Headers: headers,
		Cookies: resp.Cookies(),
	}
}

// HTTPGetWithCookie performs a GET with a cookie header.
func HTTPGetWithCookie(url, cookieName, cookieValue string) HTTPFullResult {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return HTTPFullResult{Status: 0, Body: "error: " + err.Error()}
	}
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookieValue})
	resp, err := client.Do(req)
	if err != nil {
		return HTTPFullResult{Status: 0, Body: "error: " + err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}
	return HTTPFullResult{
		Status:  resp.StatusCode,
		Body:    string(body),
		Headers: headers,
		Cookies: resp.Cookies(),
	}
}

// HTTPPostWithCookie performs a POST with a cookie header.
func HTTPPostWithCookie(url, contentType, body, cookieName, cookieValue string) HTTPFullResult {
	client := &http.Client{}
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return HTTPFullResult{Status: 0, Body: "error: " + err.Error()}
	}
	req.Header.Set("Content-Type", contentType)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: cookieValue})
	resp, err := client.Do(req)
	if err != nil {
		return HTTPFullResult{Status: 0, Body: "error: " + err.Error()}
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}
	return HTTPFullResult{
		Status:  resp.StatusCode,
		Body:    string(respBody),
		Headers: headers,
		Cookies: resp.Cookies(),
	}
}

// HeaderValue returns a header value from HTTPFullResult.
func (r HTTPFullResult) HeaderValue(key string) string {
	return r.Headers[key]
}

// CookieCount returns the number of cookies.
func (r HTTPFullResult) CookieCount() int {
	return len(r.Cookies)
}

// CookieName returns the name of cookie at index i.
func (r HTTPFullResult) CookieName(i int) string {
	if i < len(r.Cookies) {
		return r.Cookies[i].Name
	}
	return ""
}

// CookieValue returns the value of cookie at index i.
func (r HTTPFullResult) CookieValue(i int) string {
	if i < len(r.Cookies) {
		return r.Cookies[i].Value
	}
	return ""
}
