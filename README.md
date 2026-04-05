# GALA Server

A fast, immutable HTTP server library for the [GALA language](https://github.com/martianoff/gala). Inspired by Twitter Finagle's filter model, Scala's immutable builders, and Go Echo's ergonomics -- with resilience features that go well beyond any Go framework.

## Features

- **Immutable builder pattern** -- every method returns a new Server (no mutation, no races)
- **Sealed types** -- Method and StatusCode are exhaustive, pattern-matchable enums
- **35+ built-in filters** -- logging, auth (Bearer, Basic, JWT, API key), CORS, CSRF, rate limiting, gzip, proxy, security headers, circuit breaker, retry, bulkhead, ETag, caching, and more
- **Filter algebra** -- compose, conditionally apply, or skip filters with `ComposeFilters`, `WhenFilter`, `Skip`
- **Type-safe extractors** -- `PathParam`, `QueryRequired`, `HeaderRequired`, `BodyAs[T]` returning `Try[T]`
- **TLS/HTTPS** -- `ListenTLS`, `ListenGracefulTLS` with cert/key
- **Server-Sent Events** -- `SSE()` and `SSEStream()` for real-time push
- **Resilience** -- `CircuitBreaker` (3-state), `RetryFilter` / `RetryFilterWithBackoff`, `Bulkhead` concurrency limiter
- **Prometheus metrics** -- `MetricsFilter` tracks per-route request count, latency, status codes; `WithMetricsEndpoint` exposes `/metrics`
- **Session management** -- cookie-based in-memory sessions with `SessionFilter`, `SessionGet/Set/Delete`, configurable expiry
- **Sliding window rate limiter** -- `RateLimitSlidingWindow` and `RateLimitSlidingWindowPerIP` for accurate per-window throttling
- **Content negotiation** -- `Accepts`, `AcceptsJSON`, `Negotiate` for multi-format APIs
- **Health & readiness** -- `WithHealthCheck`, `WithReadiness` for container orchestration
- **Error mapping** -- `WithErrorMapper` converts domain errors to HTTP responses
- **Zero-reflection JSON** -- codec-based serialization via `JsonFrom[T]`
- **Base path** -- `WithBasePath("/api/v1")` prefixes all routes
- **Graceful shutdown** -- `ListenGraceful()` with configurable timeout
- **Warmup** -- `WithWarmup` runs initialization before accepting traffic
- **Go bridge** -- thin httpcore layer (~500 lines) is the only Go code

## Quick Start

```gala
package main

import (
    . "martianoff/gala-server"
    "time"
)

func main() {
    // Prometheus metrics collector
    val stats = NewMetrics()

    // Cookie-based session store
    val sessions = NewSessions("my-secret-key").
        WithCookieName("sid").
        WithMaxAge(3600)

    val server = NewServer().
        WithName("My API").
        WithPort(8080).
        WithBasePath("/api").
        WithHealthCheck("/health").
        WithReadiness("/ready", () => true).
        WithMetricsEndpoint("/metrics", stats).
        // Routes
        GET("/", (req) => Ok("Hello, GALA!")).
        GET("/users/{id}", (req) => {
            val id = req.Param("id").GetOrElse("0")
            return JsonResponse(s"{\"id\": \"$id\", \"name\": \"User $id\"}")
        }).
        GET("/profile", (req) => {
            val name = req.SessionGet("username").GetOrElse("Guest")
            return Ok(s"Hello, $name!")
        }).
        POST("/login", (req) => {
            req.SessionSet("username", "Alice")
            return Ok("Logged in")
        }).
        // Global filters
        WithFilter(Logger()).
        WithFilter(Recovery()).
        WithFilter(Cors()).
        WithFilter(MetricsFilter(stats)).
        WithFilter(SessionFilter(sessions)).
        WithFilter(RateLimitSlidingWindowPerIP(100, 1 * time.Minute)).
        WithFilter(CircuitBreaker(maxFailures = 5, resetTimeout = 30 * time.Second))

    server.ListenGraceful()
}
```

## TLS / HTTPS

```gala
server.ListenTLS("cert.pem", "key.pem")
server.ListenGracefulTLS("cert.pem", "key.pem")
```

## Type-Safe Extractors

Extractors return `Try[T]` for clean error handling via pattern matching:

```gala
func getUser(req Request) Response =
    PathParamInt(req, "id") match {
        case Success(id) => Ok(s"User $id")
        case Failure(err) => BadRequest(s"Invalid ID: $err")
    }
```

Available extractors: `PathParam`, `PathParamInt`, `QueryRequired`, `QueryInt`, `HeaderRequired`, `BodyAs[T]`.

## Server-Sent Events

```gala
server.GET("/events", (req) => {
    val events = ArrayOf(
        SSEEvent(Event = "message", Data = "Hello!", Id = "1", Retry = 0),
        SSEEvent(Event = "update", Data = "{\"count\": 42}", Id = "2", Retry = 0),
    )
    return SSE(events)
})
```

## Resilience Filters

```gala
// Circuit breaker: opens after 5 failures, resets after 30s
server.WithFilter(CircuitBreaker(maxFailures = 5, resetTimeout = 30 * time.Second))

// Retry with exponential backoff
server.WithFilter(RetryFilterWithBackoff(maxRetries = 3, initialBackoff = 100 * time.Millisecond))

// Bulkhead: limit concurrent requests
server.WithFilter(Bulkhead(maxConcurrent = 100))
```

## Filter Algebra

Compose and conditionally apply filters:

```gala
// Compose two filters into one
val secured = ComposeFilters(Auth(), RateLimit(100.0, 10.0))

// Apply filter only when predicate is true
val authWhenNotHealth = WhenFilter((req) => req.Path() != "/health", Auth())

// Skip filter for specific paths
val logExceptHealth = Skip(Logger(), "/health", "/ready")
```

## Prometheus Metrics

```gala
val stats = NewMetrics()

val server = NewServer().
    WithMetricsEndpoint("/metrics", stats).
    GET("/users", userHandler).
    WithFilter(MetricsFilter(stats))

// GET /metrics returns Prometheus text format:
//   gala_requests_total 42
//   gala_responses_total{status="200"} 38
//   gala_route_requests_total{route="/users"} 15
//   gala_route_latency_avg_ms{route="/users"} 2.34
//   gala_uptime_seconds 3600
```

## Session Management

```gala
val sessions = NewSessions("secret-key").
    WithCookieName("sid").
    WithMaxAge(3600).
    WithSecure(true)

server.
    WithFilter(SessionFilter(sessions)).
    GET("/profile", (req) => {
        val name = req.SessionGet("username").GetOrElse("Guest")
        return Ok(s"Hello, $name!")
    }).
    POST("/login", (req) => {
        req.SessionSet("username", "Alice")
        return Ok("Logged in")
    }).
    POST("/logout", (req) => {
        req.SessionDelete("username")
        return Ok("Logged out")
    })

// Periodic cleanup of expired sessions
sessions.Cleanup(1 * time.Hour)
```

## Sliding Window Rate Limiter

```gala
// Global: 1000 requests per minute
server.WithFilter(RateLimitSlidingWindow(1000, 1 * time.Minute))

// Per-IP: 100 requests per minute per client
server.WithFilter(RateLimitSlidingWindowPerIP(100, 1 * time.Minute))
```

## Content Negotiation

```gala
func handler(req Request) Response =
    Negotiate(req,
        () => JsonResponse("{\"msg\": \"hello\"}"),
        () => Html("<p>hello</p>"),
        () => Text("hello"),
    )
```

## Health & Readiness

```gala
server.
    WithHealthCheck("/health").
    WithReadiness("/ready", () => dbPool.IsConnected())
```

## Prerequisites

- [GALA](https://github.com/martianoff/gala) compiler (v0.25.3+)
- [Go SDK](https://go.dev/dl/) 1.25+ on PATH
- [Bazel](https://github.com/bazelbuild/bazelisk) (via Bazelisk)
- GALA toolchain cloned as a sibling directory: `git clone https://github.com/martianoff/gala.git ../gala_simple`

## Build & Test

```shell
# Build library
bazel build //:gala-server

# Run all tests (7 test targets, 330+ test functions)
bazel test //:server_test //:filter_test //:integration_test //:extractor_test //:sse_test //:metrics_test //:session_test //:readme_test

# Run a specific test target
bazel test //:server_test

# Build everything including examples
bazel build //...
```

> **Note:** `MODULE.bazel` uses `local_path_override` pointing to `../gala_simple`. Clone the [GALA repo](https://github.com/martianoff/gala) there, or update the path.

## Running the Example

The `examples/hello/` directory contains a full-featured demo with 20 routes, 8 filters, route groups, auth, SSE, content negotiation, health checks, and more.

```shell
# Build and run
bazel run //examples/hello
```

The server starts on `http://localhost:8080` with base path `/api`. Test with:

```shell
# Basic routes
curl http://localhost:8080/api/
curl http://localhost:8080/api/health
curl http://localhost:8080/api/ready

# Search with query params
curl "http://localhost:8080/api/public/search?q=gala&page=2"

# Server-Sent Events
curl http://localhost:8080/api/public/sse

# Content negotiation
curl -H "Accept: application/json" http://localhost:8080/api/public/negotiate
curl -H "Accept: text/html" http://localhost:8080/api/public/negotiate

# Auth required (401 without header)
curl http://localhost:8080/api/v1/users
curl -H "Authorization: Bearer any-token" http://localhost:8080/api/v1/users
curl -H "Authorization: Bearer any-token" http://localhost:8080/api/v1/users/42

# Basic auth
curl -u admin:password http://localhost:8080/api/basic

# API key auth
curl -H "X-API-Key: my-secret-key" http://localhost:8080/api/api-key

# Admin (bearer token)
curl -H "Authorization: Bearer admin-secret" http://localhost:8080/api/admin/stats
```

See [`examples/hello/README.md`](examples/hello/README.md) for the full list of test commands.

## Project Structure

```
gala-server/
  types.gala          # Core types: Method, StatusCode, HTTPError, ErrorMapper, etc.
  request.gala        # Request API (params, query, headers, body, context, content negotiation)
  response.gala       # Response constructors and builders (including Negotiate)
  filter.gala         # All built-in filters (35+) including resilience and filter algebra
  metrics.gala        # Prometheus metrics collector and MetricsFilter
  session.gala        # Cookie-based session management (SessionFilter, SessionGet/Set/Delete)
  extractor.gala      # Type-safe extractors (PathParam, QueryRequired, BodyAs, etc.)
  sse.gala            # Server-Sent Events (SSEEvent, SSE, SSEStream)
  group.gala          # Route groups with scoped filters
  server.gala         # Server builder, TLS, health/readiness, warmup, error mapper
  router.gala         # Route type and registration
  httpcore/            # Go bridge (~500 lines) — the only Go code
    httpcore.go        # BridgeRequest, BridgeResponse, ServerBuilder, CircuitBreaker, Semaphore
    testing.go         # TestRequestBuilder for unit tests
  server_test.gala     # Server, response, request unit tests
  filter_test.gala     # Filter behavior tests
  integration_test.gala # HTTP-level integration tests
  examples/
    hello/main.gala    # Example application showcasing all features
  docs/                # Documentation
```

## Architecture

```
+---------------------------------------------+
|              GALA Server Layer               |
|  types, request, response, filter, group,   |
|  server, router, extractor, sse  (all .gala) |
+---------------------------------------------+
|           httpcore Bridge (Go)               |
|  BridgeRequest, BridgeResponse,             |
|  ServerBuilder, CircuitBreaker, Semaphore,   |
|  TokenBucket, HMACSigner, ETag              |
+---------------------------------------------+
|              Go net/http                     |
+---------------------------------------------+
```

The GALA layer is purely functional and immutable. The httpcore bridge is a thin Go adapter that translates between `net/http` and GALA's type system. All HTTP types (Request, Response, Method, StatusCode) are GALA-native sealed types.

## Documentation

- [Getting Started](docs/getting-started.md) -- installation, core concepts, first server
- [API Reference](docs/api-reference.md) -- complete reference for all types, responses, request API, filters
- [Filters](docs/filters.md) -- filter system: three scopes, execution order, built-in filters, filter algebra, resilience
- [Route Groups](docs/groups.md) -- grouping routes with shared prefixes and scoped filters
- [Transpiler Notes](TRANSPILER_NOTES.md) -- known transpiler issues and workarounds for gala-server development

## Test Coverage

330+ tests across 7 test targets — **all passing** via `bazel test`:

- **Server builder**: NewServer, WithPort, WithName, WithDebug, WithBanner, immutability, Any method
- **Response constructors**: all 16 status codes, content types (JSON, HTML, XML, Text), JSONP, blobs, redirects, streams
- **Response builder**: WithHeader, WithBody, WithStatus, cookies (set, secure, delete), immutability
- **Sealed types**: Method (7 variants), StatusCode (13 variants)
- **Request API**: params, query params, headers, body, form, cookies, context, host, scheme, TLS, RealIP
- **Filters**: Logger, Recovery, CORS, Auth, AuthBearer, BasicAuth, KeyAuth, RateLimit, RateLimitPerIP, Timeout, Gzip, RequestId, Secure, BodyLimit, CSRF, HTTPSRedirect, WWWRedirect, NonWWWRedirect, TrailingSlash, RemoveTrailingSlash, MethodOverride, BodyDump, Decompress, Rewrite, JWTAuth, Proxy
- **Groups**: nesting, composition, filter scoping, filter isolation
- **HTTPError**: structured errors, JSON responses
- **Route naming**: URL generation with parameter substitution
- **Metrics**: MetricsFilter recording, Prometheus exposition endpoint, concurrent counter safety
- **Sessions**: SessionFilter lifecycle, SessionGet/Set/Delete, cleanup, cookie configuration
- **Sliding window**: RateLimitSlidingWindow and per-IP variant, window expiry, burst accuracy
- **JWT**: token creation, validation, expiry, claims, signature verification
- **Integration**: full HTTP request/response cycle with real server
