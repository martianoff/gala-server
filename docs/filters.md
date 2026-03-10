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
server.GET("/metrics", metricsHandler, Auth(), RateLimit())
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

For a request, the chain is: Logger → Recovery → Cors → Handler → Cors → Recovery → Logger

## Built-in Filters

### Logger

Logs each request with method, path, status code, and duration:

```gala
server.WithFilter(Logger())
// Output: GET /users -> 200 (1.234ms)
```

### Recovery

Catches handler panics/failures and returns a 500 response instead of crashing:

```gala
server.WithFilter(Recovery())
```

### Cors / CorsWithOrigins

Adds CORS headers. `Cors()` allows all origins; `CorsWithOrigins(origin)` restricts:

```gala
server.WithFilter(Cors())                       // Access-Control-Allow-Origin: *
server.WithFilter(CorsWithOrigins("https://example.com"))
```

### Auth / AuthWithValidator

`Auth()` checks for the `Authorization` header (returns 401 if missing). `AuthWithValidator` validates the header value:

```gala
// Simple presence check
server.GET("/secret", handler, Auth())

// Custom validation
server.GET("/api", handler, AuthWithValidator((token) => token == "Bearer my-secret"))
```

## Writing Custom Filters

A filter receives a request and the next handler in the chain. It must return `Future[Response]`:

```gala
func RateLimit() Filter = rateLimitFilter

func rateLimitFilter(req Request, next Handler) Future[Response] {
    // Check rate limit...
    if isOverLimit(req) {
        return FutureOf[Response](StatusCode(429, "Too Many Requests"))
    }
    return next(req)
}
```

**Important**: Due to GALA transpiler BUG-006, filter implementations must be **named functions** (not block lambdas). The factory function (e.g., `RateLimit()`) returns the named function as a `Filter`.

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
- `GET /` → Logger → Recovery → home
- `GET /health` → Logger → Recovery → health
- `GET /metrics` → Logger → Recovery → Auth → metrics
- `GET /admin/dashboard` → Logger → Recovery → Auth → dashboard
- `GET /admin/settings` → Logger → Recovery → Auth → settings
