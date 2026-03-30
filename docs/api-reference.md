# API Reference

Complete reference for gala-server types, functions, and filters.

## Response Constructors

### Status Code Constructors

| Function | Status | Description |
|----------|--------|-------------|
| `Ok(body)` | 200 | Success |
| `Created(body)` | 201 | Resource created |
| `Accepted(body)` | 202 | Accepted for processing |
| `NoContent()` | 204 | No content |
| `BadRequest(body)` | 400 | Client error |
| `Unauthorized(body)` | 401 | Authentication required |
| `Forbidden(body)` | 403 | Access denied |
| `NotFound(body)` | 404 | Resource not found |
| `MethodNotAllowed()` | 405 | Method not allowed |
| `RequestTimeout()` | 408 | Timeout |
| `Conflict(body)` | 409 | Conflict |
| `UnprocessableEntity(body)` | 422 | Unprocessable |
| `TooManyRequests(body)` | 429 | Rate limited |
| `InternalError(body)` | 500 | Server error |
| `BadGateway(body)` | 502 | Bad gateway |
| `ServiceUnavailable(body)` | 503 | Unavailable |

### Content-Type Constructors

| Function | Content-Type | Description |
|----------|-------------|-------------|
| `JsonResponse(body)` | application/json | JSON response |
| `Text(body)` | text/plain | Plain text |
| `Html(body)` | text/html | HTML |
| `Xml(body)` | application/xml | XML |
| `JSONP(callback, body)` | application/javascript | JSONP with callback |
| `Blob(status, ct, data)` | custom | Binary response |
| `File(path)` | auto-detect | Serve file |
| `Attachment(path, name)` | auto-detect | File download |
| `Inline(path, name)` | auto-detect | File inline |
| `Stream(status, ct, body)` | custom | Streaming response |
| `String(status, body)` | text/plain | Text with custom status |

### Content Negotiation

`Negotiate` selects a response format based on the request's `Accept` header:

```gala
func Negotiate(req Request, json func() Response, html func() Response, fallback func() Response) Response
```

Example:

```gala
func handler(req Request) Response =
    Negotiate(req,
        () => JsonResponse("{\"msg\": \"hello\"}"),
        () => Html("<p>hello</p>"),
        () => Text("hello"),
    )
```

### JSON Serialization (zero-reflection)

```gala
func JsonFrom[T any](value T, codec JsonEncoder[T]) Response
func JsonFromWithStatus[T any](status StatusCode, value T, codec JsonEncoder[T]) Response
func JsonPretty[T any](value T, codec JsonEncoder[T]) Response
```

### Blob Variants

| Function | Description |
|----------|-------------|
| `HTMLBlob(data)` | HTML from bytes |
| `JSONBlob(data)` | JSON from bytes |
| `XMLBlob(data)` | XML from bytes |
| `HTMLBlobWithStatus(status, data)` | HTML blob with custom status |
| `JSONBlobWithStatus(status, data)` | JSON blob with custom status |
| `XMLBlobWithStatus(status, data)` | XML blob with custom status |

### Redirect Helpers

| Function | Status | Description |
|----------|--------|-------------|
| `Redirect(url)` | 302 | Temporary redirect |
| `RedirectPermanent(url)` | 301 | Permanent redirect |

### Response Builders

All response builder methods return a new Response (immutable):

```gala
resp.WithHeader("X-Custom", "value")
resp.WithBody("new body")
resp.WithBodyBytes([]byte("bytes"))
resp.WithStatus(StatusCreated())
resp.WithCookie(cookie)
```

### Cookie Helpers

```gala
resp.SetCookie("name", "value")
resp.SetSecureCookie("name", "value", 3600)
resp.DeleteCookie("name")
```

## Sealed Types

### Method

```gala
sealed type Method {
    case GET()
    case POST()
    case PUT()
    case DELETE()
    case PATCH()
    case HEAD()
    case OPTIONS()
    case CUSTOM(MethodName string)
}
```

The `CUSTOM(MethodName)` case handles non-standard HTTP methods. `Method.Name()` returns the method string. The `toMethod` parser returns `CUSTOM(s)` for unrecognized method strings.

### StatusCode

```gala
sealed type StatusCode {
    case StatusOk()                  // 200
    case StatusCreated()             // 201
    case StatusAccepted()            // 202
    case StatusNoContent()           // 204
    case StatusBadRequest()          // 400
    case StatusUnauthorized()        // 401
    case StatusForbidden()           // 403
    case StatusNotFound()            // 404
    case StatusMethodNotAllowed()    // 405
    case StatusRequestTimeout()      // 408
    case StatusConflict()            // 409
    case StatusUnprocessableEntity() // 422
    case StatusTooManyRequests()     // 429
    case StatusInternalError()       // 500
    case StatusBadGateway()          // 502
    case StatusServiceUnavailable()  // 503
    case StatusCustom(Value int)
}
```

`StatusCode.Code()` returns the numeric HTTP status code.

### ErrorMapper

```gala
type ErrorMapper func(error) Option[Response]
```

Converts specific error types to HTTP responses. Returns `Some(response)` if the error is handled, `None` otherwise. Register with `WithErrorMapper`:

```gala
val mapper ErrorMapper = (err) => {
    if errors.Is(err, ErrNotFound) {
        return Some(NotFound("resource not found"))
    }
    return None[Response]()
}

server.WithErrorMapper(mapper)
```

## Request API

### Path Parameters

```gala
req.Param("id")         // Option[string]
req.ParamInt("id")      // Option[int]
req.ParamInt64("id")    // Option[int64]
```

### Query Parameters

```gala
req.QueryParam("q")               // Option[string]
req.QueryParamInt("page")         // Option[int]
req.QueryParamDefault("q", "*")   // string
req.QueryParams("tag")            // Array[string] (multi-value)
req.RawQuery()                    // string
```

### Headers & Cookies

```gala
req.Header("Authorization")       // Option[string]
req.ContentType()                  // Option[string]
req.Cookie("session")              // Option[string]
```

### Content Negotiation

```gala
req.Accepts("application/json")   // bool — checks Accept header
req.AcceptsJSON()                  // bool — shorthand for application/json
req.AcceptsHTML()                  // bool — shorthand for text/html
req.AcceptsXML()                   // bool — shorthand for application/xml
```

These check whether the request's `Accept` header contains the given content type (or `*/*`).

### Context Propagation

```gala
req.Deadline()       // Option[time.Time] — Go context deadline, if set
req.IsCancelled()    // bool — true if Go context is cancelled/timed out
```

Use these to respect client-side timeouts and cancellation:

```gala
func handler(req Request) Response {
    if req.IsCancelled() {
        return RequestTimeout()
    }
    // ... do work
    return Ok("done")
}
```

### Body & Form

```gala
req.Body()                         // string
req.FormValue("username")          // Option[string]
req.FormParams("tags")             // Array[string]
BindJson[T](req, codec)           // Try[T]
```

### Connection Info

```gala
req.Method()       // Method (sealed type: GET(), POST(), CUSTOM("PURGE"), etc.)
req.Path()         // string
req.Host()         // string
req.Scheme()       // "http" or "https"
req.RealIP()       // string (checks X-Real-IP, X-Forwarded-For)
req.RemoteAddr()   // string
req.IsTLS()        // bool
req.IsWebSocket()  // bool
```

### Context Values

Per-request key-value store for passing data between filters and handlers:

```gala
req.CtxSet("user", "alice")
req.CtxGet("user")        // Option[string]
req.CtxGetInt("count")    // Option[int]
```

### Request Cloning

Used internally by filters like MethodOverride and Decompress:

```gala
req.WithMethod("PUT")       // Request with overridden method
req.WithBody("new body")    // Request with replaced body
```

## Type-Safe Extractors

Extractors provide a `Try`-based API for extracting values from requests. They return `Try[T]` for clean error handling via pattern matching.

### PathParam / PathParamInt

```gala
PathParam(req, "id")      // Try[string] — Failure if missing
PathParamInt(req, "id")   // Try[int] — Failure if missing or not an integer
```

### QueryRequired / QueryInt

```gala
QueryRequired(req, "q")       // Try[string] — Failure if missing
QueryInt(req, "page", 1)      // int — returns default if missing or invalid
```

### HeaderRequired

```gala
HeaderRequired(req, "X-Api-Key")   // Try[string] — Failure if missing
```

### BodyAs[T]

```gala
BodyAs[User](req, userCodec)   // Try[User] — Failure if body cannot be parsed
```

### Example

```gala
func getUser(req Request) Response =
    PathParamInt(req, "id") match {
        case Success(id) => Ok(s"User $id")
        case Failure(err) => BadRequest(s"Invalid ID: $err")
    }
```

## Server-Sent Events (SSE)

### SSEEvent

```gala
struct SSEEvent(Event string, Data string, Id string, Retry int)
```

### SSE

Creates an SSE response from an array of events. Sets `Content-Type: text/event-stream` with `Cache-Control: no-cache` and `Connection: keep-alive`.

```gala
val events = ArrayOf(
    SSEEvent(Event = "message", Data = "Hello!", Id = "1", Retry = 0),
    SSEEvent(Event = "update", Data = "{\"count\": 42}", Id = "2", Retry = 0),
)
return SSE(events)
```

### SSEStream

Convenience for streaming a single event:

```gala
return SSEStream("message", "Hello!")
```

## HTTPError

Structured errors using a sealed type for pattern matching:

```gala
sealed type HTTPError {
    case HTTPErr(Status StatusCode, Message string, Internal error)
}
```

### Constructors

```gala
NewHTTPError(StatusNotFound(), "user not found")
NewHTTPError(StatusInternalError(), "db failed", dbErr)
```

### Methods

```gala
err.Code()           // int (e.g. 404)
err.Msg()            // string (e.g. "user not found")
err.Error()          // string (e.g. "code=404, message=user not found")
err.ToResponse()     // Response with status code and message body
err.ToJsonResponse() // JSON Response: {"error": "...", "code": 404}
```

## Server Configuration

### Builder Methods

```gala
NewServer().
    WithPort(8080).
    WithName("My API").
    WithBasePath("/api/v1").
    WithDebug(true).
    WithBanner(false).
    WithShutdownTimeout(10 * time.Second).
    WithErrorHandler((err) => InternalError(err.Error())).
    WithNotFound((req) => NotFound("page not found")).
    WithErrorMapper(myMapper).
    WithWarmup(() => initCaches()).
    WithRenderer(myRenderer).
    WithValidator(myValidator)
```

### BasePath

`WithBasePath` sets a prefix for all registered routes:

```gala
val server = NewServer().
    WithBasePath("/api/v1").
    GET("/users", listUsers).        // matches /api/v1/users
    GET("/users/{id}", getUser)      // matches /api/v1/users/{id}
```

### Error Mapping

`WithErrorMapper` registers a function that converts domain errors to HTTP responses before the default error handler runs:

```gala
type ErrorMapper func(error) Option[Response]

val mapper ErrorMapper = (err) => {
    if errors.Is(err, ErrNotFound) {
        return Some(NotFound("not found"))
    }
    return None[Response]()
}

server.WithErrorMapper(mapper)
```

### Warmup

`WithWarmup` registers a function that runs before the server starts accepting traffic:

```gala
server.WithWarmup(() => {
    loadCaches()
    warmConnectionPools()
})
```

### Health Check & Readiness

```gala
// Simple health check — returns 200 "OK"
server.WithHealthCheck("/health")

// Readiness with custom check — returns 200 "READY" or 503 "NOT READY"
server.WithReadiness("/ready", () => dbPool.IsConnected())
```

### Route Naming & URL Generation

```gala
val s = NewServer().
    GET("/users/{id}", handler).Named("user-detail")

s.URL("user-detail", "id", "42")  // Some("/users/42")
```

### Static File Serving

```gala
server.Static("/public/", "./static")
```

### Pluggable Interfaces

```gala
type Renderer interface { Render(name string, data any) string }
type Validator interface { Validate(i any) error }
```

### Listening (HTTP)

```gala
server.Listen()                    // Start on configured port (default 8080)
server.ListenOn(":9090")           // Start on explicit address
server.ListenGraceful()            // Start with graceful shutdown
server.ListenGracefulOn(":9090")   // Graceful on explicit address
```

### Listening (TLS/HTTPS)

```gala
server.ListenTLS("cert.pem", "key.pem")                   // HTTPS on configured port
server.ListenTLSOn(":443", "cert.pem", "key.pem")         // HTTPS on explicit address
server.ListenGracefulTLS("cert.pem", "key.pem")            // HTTPS + graceful shutdown
server.ListenGracefulTLSOn(":443", "cert.pem", "key.pem") // HTTPS + graceful on address
```

All TLS methods accept a certificate file and private key file path. They print a banner indicating HTTPS mode.

## Filters (Middleware)

All filters have the type `func(Request, Handler) Future[Response]` and use `Future.Map`/`Future.Recover` for non-blocking composition.

### Logging

```gala
Logger()                                              // Default: "METHOD /path -> STATUS (duration)"
LoggerWithFormat((method, path, status, dur) => ...)  // Custom format function
```

### Error Recovery

```gala
Recovery()    // Catches panics, returns 500 Internal Server Error
```

### Authentication

```gala
Auth()                                        // Requires any Authorization header
AuthWithValidator((header) => bool)           // Custom header validation
AuthBearer((token) => bool)                   // Bearer token validation
BasicAuth((user, pass) => bool)               // HTTP Basic Auth (sets "username" in context)
KeyAuth("header:X-API-Key", (key) => bool)    // API key from header, query, or cookie
```

### Security Headers

```gala
Secure()                                       // Default: XSS, HSTS, CSP, etc.
SecureWithConfig(xss, csp, hsts, frame, ct)    // Custom security header configuration
```

### CSRF Protection

```gala
CSRF("my-secret-key")   // HMAC-signed token validation
```

### CORS

```gala
Cors()                                                  // Allow all origins (*)
CorsWithOrigins("https://example.com")                  // Specific origin
CorsWithConfig(origins, methods, headers, maxAge)       // Full configuration
```

### Rate Limiting

```gala
// Token bucket (burst-friendly, configurable refill rate)
RateLimit(1000.0, 100.0)              // Global (max tokens, refill/sec)
RateLimitPerIP(100.0, 10.0)           // Per-IP

// Sliding window (more accurate for bursty traffic)
RateLimitSlidingWindow(100, 1 * time.Minute)        // Global (max requests, window duration)
RateLimitSlidingWindowPerIP(50, 1 * time.Minute)    // Per-IP
```

**Token bucket** allows bursts up to `maxTokens`, then refills at `refillPerSecond`. Good for smooth rate limiting.

**Sliding window** counts exact requests within the time window. More accurate than token bucket for bursty traffic patterns — no burst allowance beyond the limit.

### Request Processing

```gala
BodyLimit("2M")                                 // Reject large requests (K, M, G suffixes)
BodyLimitBytes(2097152)                         // Reject by exact byte count
Gzip()                                          // Compress responses (Accept-Encoding: gzip)
Decompress()                                    // Decompress gzip request bodies
RequestId()                                     // Add X-Request-ID header + context value
Timeout(5 * time.Second)                        // Cancel slow handlers via Future racing
MethodOverride()                                // Override POST via X-HTTP-Method-Override or _method
BodyDump((reqBody, respBody) => ...)            // Debug: log request and response bodies
BodyDumpWithRequest((reqBody, respBody) => ...) // Same, different signature
```

### JWT Authentication

```gala
JWTAuth("my-secret-key")                     // Validate HS256 JWT with expiry check
JWTAuthWithConfig("my-secret-key", false)     // Validate HS256 JWT without expiry check
```

On success, stores claims in request context: `jwt-sub`, `jwt-iss`, `jwt-aud`, and `jwt-claims` (the full `*httpcore.JWTClaims` object).

### Reverse Proxy

```gala
Proxy("http://backend:3000")                           // Forward requests to backend
ProxyWithTimeout("http://backend:3000", 10 * time.Second) // With custom timeout
```

Terminal filter that replaces the handler response with the proxied backend response. Forwards headers, strips hop-by-hop headers, returns 502 on proxy failure.

### URL Manipulation

```gala
TrailingSlash()        // Add trailing slash to path
RemoveTrailingSlash()  // Remove trailing slash from path
HTTPSRedirect()        // Redirect HTTP requests to HTTPS
WWWRedirect()          // Redirect non-www to www
NonWWWRedirect()       // Redirect www to non-www
Rewrite(rules)         // URL path rewriting with wildcard matching
```

### Filter Algebra

Compose and conditionally apply filters:

```gala
// Compose two filters sequentially (outer wraps inner)
ComposeFilters(outer, inner)

// Conditional: apply filter only when predicate returns true
WhenFilter((req) => req.Path() != "/health", Auth())

// Skip filter for specific paths
Skip(Logger(), "/health", "/ready", "/metrics")

// Sugar for passing multiple filters as an Array
Use(Logger(), Recovery(), Cors())
```

### ETag / Cache-Control

```gala
ETag()                 // Auto-generate ETags from response body hash; returns 304 on If-None-Match match
CacheControl(3600)     // Set Cache-Control: public, max-age=3600
NoCacheFilter()        // Set Cache-Control: no-cache, no-store, must-revalidate + Pragma + Expires
```

### Circuit Breaker

```gala
CircuitBreaker(
    maxFailures = 5,                    // failures before circuit opens
    resetTimeout = 30 * time.Second,    // time before trying half-open
    halfOpenMax = 1,                    // requests allowed in half-open state
)
```

Three states:
- **Closed** -- normal operation, requests pass through
- **Open** -- after `maxFailures` consecutive failures, all requests rejected with 503
- **Half-Open** -- after `resetTimeout`, allows `halfOpenMax` probe requests; success closes the circuit, failure re-opens it

### Retry / RetryWithBackoff

```gala
// Fixed backoff between retries
RetryFilter(maxRetries = 3, backoff = 100 * time.Millisecond)

// Exponential backoff: doubles each attempt, capped at maxBackoff
RetryFilterWithBackoff(
    maxRetries = 3,
    initialBackoff = 100 * time.Millisecond,
    maxBackoff = 5 * time.Second,
)
```

Retries on 5xx responses or handler failures. Returns the last response/error after all retries are exhausted.

### Bulkhead (Concurrency Limiter)

```gala
Bulkhead(maxConcurrent = 100)
```

Limits the number of concurrent in-flight requests using a semaphore. When at capacity, immediately returns 503 Service Unavailable. Releases the semaphore slot when the handler completes (success or failure).

## Metrics (Prometheus-compatible)

Built-in request metrics with a Prometheus text exposition endpoint. Inspired by Twitter Finatra's `StatsReceiver`.

### Setup

```gala
val stats = NewMetrics()

val server = NewServer().
    WithFilter(MetricsFilter(stats)).
    WithMetricsEndpoint("/metrics", stats).
    GET("/hello", (req) => Ok("hello")).
    ListenGraceful()
```

### NewMetrics

Creates a new metrics collector. Thread-safe via atomic counters.

```gala
val stats = NewMetrics()
stats.TotalRequests()    // int64 — total recorded requests
stats.UptimeSeconds()    // float64 — server uptime
```

### MetricsFilter

Records per-request metrics including method, path, status code, and latency. Uses `Future.Map` so latency measurement includes the full async handler execution across goroutines.

```gala
server.WithFilter(MetricsFilter(stats))
```

### WithMetricsEndpoint

Registers a GET endpoint that serves all collected metrics in Prometheus text exposition format:

```gala
server.WithMetricsEndpoint("/metrics", stats)
```

**Exposed metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `gala_requests_total` | counter | Total HTTP requests |
| `gala_uptime_seconds` | gauge | Server uptime |
| `gala_responses_total{status}` | counter | Responses by status code |
| `gala_route_requests_total{route}` | counter | Requests per route |
| `gala_route_latency_avg_ms{route}` | gauge | Average latency per route |
| `gala_route_latency_max_ms{route}` | gauge | Max latency per route |

## Session Management

Cookie-based in-memory session management with concurrent access support. Inspired by Finatra's session handling and Express.js sessions.

### Setup

```gala
val sessions = NewSessions("my-secret")

val server = NewServer().
    WithFilter(SessionFilter(sessions)).
    GET("/login", (req) => {
        req.SessionSet("user", "alice")
        return Ok("logged in")
    }).
    GET("/profile", (req) => {
        val user = req.SessionGet("user").GetOrElse("anonymous")
        return Ok(s"Hello, $user")
    })
```

### NewSessions

Creates a new session store with default settings:

```gala
val sessions = NewSessions("my-secret")
// Defaults: cookie name = "gala_session", max age = 86400s, httpOnly = true, secure = false
```

### Configuration

```gala
val sessions = NewSessions("secret").
    WithCookieName("my_session").   // custom cookie name
    WithMaxAge(3600).               // 1 hour session lifetime
    WithSecure(true)                // HTTPS-only cookies
```

### SessionFilter

Filter that manages session lifecycle. On each request:
1. Reads the session cookie (or creates a new session)
2. Stores session data in the request context
3. After the handler completes (via `Future.Map`), sets the session cookie on new sessions

```gala
server.WithFilter(SessionFilter(sessions))
```

### Request Session Accessors

Available on any `Request` when `SessionFilter` is active:

```gala
req.SessionGet("key")                // Option[string] — retrieve a session value
req.SessionSet("key", "value")       // store a session value
req.SessionDelete("key")             // remove a session value
```

### Session Store Management

```gala
sessions.SessionCount()              // int — number of active sessions
sessions.Cleanup(1 * time.Hour)      // int — remove sessions inactive for > 1 hour, returns count removed
```

## Architecture

```
+-----------------------------------------------+
|               GALA Server Layer                |
|  server.gala, request.gala, response.gala,     |
|  filter.gala, group.gala, router.gala,         |
|  types.gala, extractor.gala, sse.gala,         |
|  metrics.gala, session.gala                    |
+-----------------------------------------------+
|            httpcore Bridge (Go)                |
|  BridgeRequest, BridgeResponse, ServerBuilder, |
|  CircuitBreaker, Semaphore, TokenBucket,       |
|  SlidingWindowLimiter, MetricsCollector,       |
|  SessionStore, HMACSigner, ETag, JWT           |
+-----------------------------------------------+
|               Go net/http                      |
+-----------------------------------------------+
```
