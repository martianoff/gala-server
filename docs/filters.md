# Filters

Filters are the middleware system in gala-server. Inspired by Twitter's Finagle, a Filter wraps a handler and can inspect/modify the request and response.

```gala
type Filter func(Request, Handler) Future[Response]
```

## Three Levels of Filters

Filters can be applied at three granularity levels:

### 1. Per-Route

Pass filters directly to the HTTP method -- they apply only to that route:

```gala
server.GET("/admin", adminHandler, Auth())
server.GET("/metrics", metricsHandler, Auth(), RateLimit(1000.0, 100.0))
```

### 2. Group-Level

Create a `Group`, add filters with `WithFilter()`, then mount it. Filters apply to all routes in the group:

```gala
val api = NewGroup().
    GET("/users", listUsers).
    POST("/users", createUser).
    WithFilter(Auth())

val server = NewServer().
    Group("/api/v1", api)   // Auth applies to /api/v1/users only
```

### 3. Global

Add filters directly to the Server -- they apply to every route:

```gala
val server = NewServer().
    GET("/", home).
    Group("/api", api).
    WithFilter(Logger()).     // wraps ALL routes
    WithFilter(Recovery())    // wraps ALL routes
```

## Filter Execution Order

Filters execute in **declaration order** (first added = outermost):

```gala
server.
    WithFilter(Logger()).     // 1st: runs first (outermost)
    WithFilter(Recovery()).   // 2nd: catches panics
    WithFilter(Cors())        // 3rd: adds CORS headers (innermost)
```

For a request, the chain is: Logger -> Recovery -> Cors -> Handler -> Cors -> Recovery -> Logger

## Filter Algebra

gala-server provides combinators for composing and conditionally applying filters.

### ComposeFilters

Composes two filters into one. The outer filter runs first, then delegates to the inner filter which wraps the handler:

```gala
val secured = ComposeFilters(Auth(), RateLimit(100.0, 10.0))

// Equivalent to applying both filters, but as a single unit:
server.WithFilter(secured)
```

### When

Conditionally applies a filter only when the predicate returns true. If the predicate returns false, the handler is called directly:

```gala
// Only apply auth to non-health-check paths
val conditionalAuth = WhenFilter((req) => req.Path() != "/health", Auth())

server.WithFilter(conditionalAuth)
```

### Skip

Skips applying a filter for specific paths. Useful for excluding health check, readiness, or metrics endpoints:

```gala
val logExceptHealth = Skip(Logger(), "/health", "/ready", "/metrics")

server.WithFilter(logExceptHealth)
```

### Use

Sugar for creating an `Array[Filter]` from variadic arguments:

```gala
val filters = Use(Logger(), Recovery(), Cors())
```

## Built-in Filters

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

KeyAuth source format: `"header:X-API-Key"`, `"query:api_key"`, or `"cookie:api_key"`.

### Security Headers

```gala
Secure()                                       // Default XSS, HSTS, CSP, frame, content-type headers
SecureWithConfig(xss, csp, hsts, frame, ct)    // Custom configuration
```

### CSRF Protection

```gala
CSRF("my-secret-key")   // HMAC-signed token validation, skips GET/HEAD/OPTIONS
```

### CORS

```gala
Cors()                                                  // Allow all origins (*)
CorsWithOrigins("https://example.com")                  // Specific origin
CorsWithConfig(origins, methods, headers, maxAge)       // Full configuration with OPTIONS preflight
```

### Rate Limiting

Two rate limiting strategies are available:

**Token Bucket** — allows bursts up to `maxTokens`, then refills at a steady rate. Good for smooth traffic shaping:

```gala
RateLimit(1000.0, 100.0)      // Global (max tokens, refill/sec)
RateLimitPerIP(100.0, 10.0)   // Per-IP (separate bucket per IP)
```

**Sliding Window** — counts exact requests within a time window. More accurate than token bucket for bursty traffic patterns. No burst allowance beyond the limit:

```gala
RateLimitSlidingWindow(100, 1 * time.Minute)       // Global (max requests, window duration)
RateLimitSlidingWindowPerIP(50, 1 * time.Minute)   // Per-IP (separate window per IP)
```

When the limit is exceeded, both strategies return 429 Too Many Requests with a `Retry-After: 1` header.

### Request Processing

```gala
BodyLimit("2M")                                 // Reject large requests (K, M, G suffixes)
BodyLimitBytes(2097152)                         // Reject by exact byte count
Gzip()                                          // Compress responses when client accepts gzip
Decompress()                                    // Decompress gzip request bodies
RequestId()                                     // Add X-Request-ID header + set "request_id" in context
Timeout(5 * time.Second)                        // Cancel slow handlers via Future racing
MethodOverride()                                // Override POST method via X-HTTP-Method-Override or _method form field
BodyDump((reqBody, respBody) => ...)            // Debug: log request and response bodies
BodyDumpWithRequest((reqBody, respBody) => ...) // Same with different handler signature
```

### JWT Authentication

```gala
JWTAuth("my-hmac-secret")                      // Validate HS256 JWT Bearer tokens, check expiry
JWTAuthWithConfig("my-secret", false)           // Same but skip expiry check
```

Extracts the Bearer token from the Authorization header, validates the HMAC-SHA256 signature, and optionally checks the `exp` claim. On success, stores `jwt-sub`, `jwt-iss`, `jwt-aud` in the request context.

### Reverse Proxy

```gala
Proxy("http://backend:3000")                             // Forward all matching requests to backend
ProxyWithTimeout("http://backend:3000", 10 * time.Second) // With custom timeout
```

A terminal filter that forwards the request to a backend server and returns the backend's response. Strips hop-by-hop headers and returns 502 Bad Gateway on failure.

### URL Manipulation

```gala
TrailingSlash()        // Add trailing slash to path
RemoveTrailingSlash()  // Remove trailing slash from path
HTTPSRedirect()        // Redirect HTTP requests to HTTPS (preserves query string)
WWWRedirect()          // Redirect non-www to www
NonWWWRedirect()       // Redirect www to non-www
Rewrite(rules)         // URL path rewriting with wildcard matching
```

### ETag

Generates ETags from response body hashes. Supports conditional requests via `If-None-Match` -- returns 304 Not Modified when the client already has the current version:

```gala
server.WithFilter(ETag())
```

When a request includes `If-None-Match` with a matching ETag, the filter short-circuits and returns a 304 response with no body, saving bandwidth.

### Cache-Control / NoCacheFilter

Control caching behavior on responses:

```gala
// Set Cache-Control: public, max-age=3600 (1 hour)
server.WithFilter(CacheControl(3600))

// Disable caching entirely:
// Cache-Control: no-cache, no-store, must-revalidate
// Pragma: no-cache
// Expires: 0
server.WithFilter(NoCacheFilter())
```

### Circuit Breaker

Implements the circuit breaker pattern to protect against cascading failures:

```gala
server.WithFilter(CircuitBreaker(
    maxFailures = 5,
    resetTimeout = 30 * time.Second,
    halfOpenMax = 1,
))
```

The circuit breaker has three states:

1. **Closed** (normal) -- requests pass through normally. Each 5xx response or handler failure increments the failure counter. When failures reach `maxFailures`, the circuit transitions to Open.

2. **Open** (tripped) -- all requests are immediately rejected with 503 Service Unavailable. After `resetTimeout` elapses, the circuit transitions to Half-Open.

3. **Half-Open** (probing) -- allows up to `halfOpenMax` requests through as probes. If a probe succeeds, the circuit closes (resets failure count). If a probe fails, the circuit re-opens.

This prevents a failing service from being overwhelmed with requests and allows automatic recovery.

### Retry / RetryWithBackoff

Retry failed requests with configurable backoff strategies:

```gala
// Fixed backoff: wait 100ms between each retry
server.WithFilter(RetryFilter(maxRetries = 3, backoff = 100 * time.Millisecond))

// Exponential backoff: 100ms -> 200ms -> 400ms -> ... capped at 5s
server.WithFilter(RetryFilterWithBackoff(
    maxRetries = 3,
    initialBackoff = 100 * time.Millisecond,
    maxBackoff = 5 * time.Second,
))
```

Both retry on 5xx responses or handler failures. The handler is re-invoked up to `maxRetries` additional times. After all retries are exhausted, the last response or error is returned.

**RetryWithBackoff** doubles the wait time after each attempt (exponential backoff), capped at `maxBackoff`. This reduces pressure on struggling services.

### Bulkhead (Concurrency Limiter)

Limits the number of concurrent in-flight requests using a semaphore:

```gala
server.WithFilter(Bulkhead(maxConcurrent = 100))
```

When the number of concurrent requests reaches `maxConcurrent`, new requests are immediately rejected with 503 Service Unavailable. The semaphore slot is released when the handler completes, whether it succeeds or fails.

This prevents any single service from consuming all available resources and provides backpressure to callers.

### Metrics (Finatra-inspired)

Records per-request metrics including method, path, status code, and latency. Uses `Future.Map` so latency includes the full async handler execution time. Pairs with `WithMetricsEndpoint` on the Server for a Prometheus-compatible `/metrics` endpoint.

```gala
val stats = NewMetrics()

val server = NewServer().
    WithFilter(MetricsFilter(stats)).
    WithMetricsEndpoint("/metrics", stats).
    GET("/hello", (req) => Ok("hello"))
```

**Collected metrics**: total requests, per-route request counts, per-route average/max latency, response status code distribution. All exposed in Prometheus text exposition format at the configured endpoint.

See [API Reference — Metrics](api-reference.md#metrics-prometheus-compatible) for the full list of exposed metrics.

### Session Management

Manages cookie-based sessions backed by a concurrent in-memory store. Reads or creates a session on each request, stores it in the request context, and sets the session cookie on new sessions via `Future.Map`.

```gala
val sessions = NewSessions("my-secret").
    WithCookieName("app_session").
    WithMaxAge(3600).
    WithSecure(true)

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

**Request accessors** (available when `SessionFilter` is active):
- `req.SessionGet("key")` — `Option[string]`
- `req.SessionSet("key", "value")` — store a value
- `req.SessionDelete("key")` — remove a value

See [API Reference — Sessions](api-reference.md#session-management) for configuration options and store management.

## Writing Custom Filters

A filter receives a request and the next handler in the chain. It must return `Future[Response]`. Use `Future.Map` and `Future.Recover` for non-blocking composition:

```gala
func TimingFilter() Filter =
    (req Request, next Handler) => {
        val start = time.Now()
        return next(req).Map((resp) => {
            val elapsed = time.Since(start)
            Println(s"${req.Path()} took $elapsed")
            return resp.WithHeader("X-Response-Time", elapsed.String())
        })
    }
```

## Combining All Three Levels

```gala
val admin = NewGroup().
    GET("/dashboard", dashboard).
    GET("/settings", settings).
    WithFilter(Auth())              // group: only admin routes

val server = NewServer().
    GET("/", home).                 // no per-route filters
    GET("/health", health).         // no per-route filters
    GET("/metrics", metrics, Auth()).  // per-route: just this endpoint
    Group("/admin", admin).         // group filters from above
    WithFilter(Logger()).           // global: all routes
    WithFilter(Recovery())          // global: all routes
```

Result:

- `GET /` -> Logger -> Recovery -> home
- `GET /health` -> Logger -> Recovery -> health
- `GET /metrics` -> Logger -> Recovery -> Auth -> metrics
- `GET /admin/dashboard` -> Logger -> Recovery -> Auth -> dashboard
- `GET /admin/settings` -> Logger -> Recovery -> Auth -> settings

See the [API Reference](api-reference.md) for the complete list of filter signatures and parameters.
