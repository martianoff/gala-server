# Filters

Filters are the middleware system in gala-server. Inspired by Twitter's Finagle, a Filter wraps a handler and can inspect/modify the request and response.

```gala
type Filter func(Request, Handler) Future[Response]
```

## Three Levels of Filters

Filters can be applied at three granularity levels:

### 1. Per-Route

Pass filters directly to the HTTP method — they apply only to that route:

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

Add filters directly to the Server — they apply to every route:

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

```gala
RateLimit(1000.0, 100.0)      // Global token bucket (max tokens, refill/sec)
RateLimitPerIP(100.0, 10.0)   // Per-IP rate limiting (separate bucket per IP)
```

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
