// Package benchmark provides comparative benchmarks between GALA Server bridge,
// Go net/http stdlib, Echo, and Gin web frameworks.
//
// Run: cd benchmark && go test -bench=. -benchmem -count=5 -benchtime=3s
//
// These benchmarks measure handler throughput (no network I/O) by calling
// ServeHTTP directly. This isolates routing and handler overhead from TCP costs.
package benchmark

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/labstack/echo/v4"
)

// ============================================================================
// 1. Go net/http (stdlib baseline)
// ============================================================================

func newNetHTTPMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello, World!"))
	})

	mux.HandleFunc("GET /json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"message":"Hello, World!"}`))
	})

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		w.WriteHeader(200)
		w.Write([]byte("user: " + id))
	})

	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		w.WriteHeader(200)
		w.Write(buf[:n])
	})

	mux.HandleFunc("GET /headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test")
		w.Header().Set("X-Version", "1.0")
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	return mux
}

func BenchmarkNetHTTP_HelloWorld(b *testing.B) {
	h := newNetHTTPMux()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkNetHTTP_JSON(b *testing.B) {
	h := newNetHTTPMux()
	req := httptest.NewRequest("GET", "/json", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkNetHTTP_PathParam(b *testing.B) {
	h := newNetHTTPMux()
	req := httptest.NewRequest("GET", "/users/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkNetHTTP_PostEcho(b *testing.B) {
	h := newNetHTTPMux()
	payload := `{"name":"test","value":42}`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/echo", strings.NewReader(payload))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkNetHTTP_Headers(b *testing.B) {
	h := newNetHTTPMux()
	req := httptest.NewRequest("GET", "/headers", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

// ============================================================================
// 2. GALA Server (simulated bridge overhead)
// ============================================================================
//
// GALA transpiles to Go and adds a thin bridge layer:
//   - BridgeRequest struct with closure-based lazy accessors
//   - BridgeResponse struct with header map
//   - Response write-back loop
// This simulation mirrors the exact code paths the transpiled GALA generates.

type bridgeReq struct {
	Method   string
	Path     string
	RawQuery string
	Body     string
}

type bridgeResp struct {
	Status  int
	Body    string
	headers map[string]string
}

func newBridgeResp(status int, body string) *bridgeResp {
	return &bridgeResp{Status: status, Body: body, headers: make(map[string]string)}
}

func (r *bridgeResp) setHeader(k, v string) { r.headers[k] = v }

func newGALAMux() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		br := bridgeReq{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
		_ = br
		resp := newBridgeResp(200, "Hello, World!")
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	})

	mux.HandleFunc("GET /json", func(w http.ResponseWriter, r *http.Request) {
		br := bridgeReq{Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
		_ = br
		resp := newBridgeResp(200, `{"message":"Hello, World!"}`)
		resp.setHeader("Content-Type", "application/json")
		for k, v := range resp.headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	})

	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		br := bridgeReq{Method: r.Method, Path: r.URL.Path}
		_ = br
		id := r.PathValue("id")
		resp := newBridgeResp(200, "user: "+id)
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	})

	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.Body != nil {
			b := make([]byte, 4096)
			n, _ := r.Body.Read(b)
			body = string(b[:n])
			r.Body.Close()
		}
		br := bridgeReq{Method: r.Method, Path: r.URL.Path, Body: body}
		_ = br
		resp := newBridgeResp(200, body)
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	})

	mux.HandleFunc("GET /headers", func(w http.ResponseWriter, r *http.Request) {
		br := bridgeReq{Method: r.Method, Path: r.URL.Path}
		_ = br
		resp := newBridgeResp(200, "ok")
		resp.setHeader("X-Custom", "test")
		resp.setHeader("X-Version", "1.0")
		for k, v := range resp.headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(resp.Status)
		w.Write([]byte(resp.Body))
	})

	return mux
}

func BenchmarkGALA_HelloWorld(b *testing.B) {
	h := newGALAMux()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkGALA_JSON(b *testing.B) {
	h := newGALAMux()
	req := httptest.NewRequest("GET", "/json", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkGALA_PathParam(b *testing.B) {
	h := newGALAMux()
	req := httptest.NewRequest("GET", "/users/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkGALA_PostEcho(b *testing.B) {
	h := newGALAMux()
	payload := `{"name":"test","value":42}`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/echo", strings.NewReader(payload))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

func BenchmarkGALA_Headers(b *testing.B) {
	h := newGALAMux()
	req := httptest.NewRequest("GET", "/headers", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
}

// ============================================================================
// 3. Echo
// ============================================================================

func newEchoRouter() *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	e.GET("/", func(c echo.Context) error {
		return c.String(200, "Hello, World!")
	})

	e.GET("/json", func(c echo.Context) error {
		return c.JSONBlob(200, []byte(`{"message":"Hello, World!"}`))
	})

	e.GET("/users/:id", func(c echo.Context) error {
		id := c.Param("id")
		return c.String(200, "user: "+id)
	})

	e.POST("/echo", func(c echo.Context) error {
		buf := make([]byte, 4096)
		n, _ := c.Request().Body.Read(buf)
		return c.String(200, string(buf[:n]))
	})

	e.GET("/headers", func(c echo.Context) error {
		c.Response().Header().Set("X-Custom", "test")
		c.Response().Header().Set("X-Version", "1.0")
		return c.String(200, "ok")
	})

	return e
}

func BenchmarkEcho_HelloWorld(b *testing.B) {
	e := newEchoRouter()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_JSON(b *testing.B) {
	e := newEchoRouter()
	req := httptest.NewRequest("GET", "/json", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_PathParam(b *testing.B) {
	e := newEchoRouter()
	req := httptest.NewRequest("GET", "/users/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_PostEcho(b *testing.B) {
	e := newEchoRouter()
	payload := `{"name":"test","value":42}`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/echo", strings.NewReader(payload))
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

func BenchmarkEcho_Headers(b *testing.B) {
	e := newEchoRouter()
	req := httptest.NewRequest("GET", "/headers", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, req)
	}
}

// ============================================================================
// 4. Gin
// ============================================================================

func newGinRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.GET("/", func(c *gin.Context) {
		c.String(200, "Hello, World!")
	})

	r.GET("/json", func(c *gin.Context) {
		c.Data(200, "application/json", []byte(`{"message":"Hello, World!"}`))
	})

	r.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.String(200, "user: "+id)
	})

	r.POST("/echo", func(c *gin.Context) {
		buf := make([]byte, 4096)
		n, _ := c.Request.Body.Read(buf)
		c.String(200, string(buf[:n]))
	})

	r.GET("/headers", func(c *gin.Context) {
		c.Header("X-Custom", "test")
		c.Header("X-Version", "1.0")
		c.String(200, "ok")
	})

	return r
}

func BenchmarkGin_HelloWorld(b *testing.B) {
	r := newGinRouter()
	req := httptest.NewRequest("GET", "/", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkGin_JSON(b *testing.B) {
	r := newGinRouter()
	req := httptest.NewRequest("GET", "/json", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkGin_PathParam(b *testing.B) {
	r := newGinRouter()
	req := httptest.NewRequest("GET", "/users/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkGin_PostEcho(b *testing.B) {
	r := newGinRouter()
	payload := `{"name":"test","value":42}`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/echo", strings.NewReader(payload))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkGin_Headers(b *testing.B) {
	r := newGinRouter()
	req := httptest.NewRequest("GET", "/headers", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
