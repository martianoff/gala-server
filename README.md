# GALA Server

A fast, immutable HTTP server library for the [GALA language](https://github.com/martianoff/gala). Inspired by Twitter Finagle's filter model, Scala's immutable builders, and Go Echo's ergonomics -- with resilience features that go well beyond any Go framework.

## Features

- **Immutable builder pattern** -- every method returns a new Server (no mutation, no races)
- **Sealed types** -- Method and StatusCode are exhaustive, pattern-matchable enums
- **35+ built-in filters** -- logging, auth (Bearer, Basic, JWT, API key), CORS, CSRF, rate limiting, gzip, proxy, security headers, circuit breaker, retry, bulkhead, ETag, caching, and more
- **Filter algebra** -- compose, conditionally apply, or skip filters with `ComposeFilters`, `When`, `Skip`
- **Type-safe extractors** -- `PathParam`, `QueryRequired`, `HeaderRequired`, `BodyAs[T]` returning `Try[T]`
- **TLS/HTTPS** -- `ListenTLS`, `ListenGracefulTLS` with cert/key
- **Server-Sent Events** -- `SSE()` and `SSEStream()` for real-time push
- **Resilience** -- `CircuitBreaker` (3-state), `Retry` / `RetryWithBackoff`, `Bulkhead` concurrency limiter
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

import . "martianoff/gala-server"

func main() {
    val server = NewServer().
        WithName("My API").
        WithPort(8080).
        GET("/", (req) => Ok("Hello, GALA!")).
        GET("/users/{id}", (req) => {
            val id = req.Param("id").GetOrElse("0")
            return JsonResponse(s"{\"id\": \"$id\"}")
        }).
        WithFilter(Logger()).
        WithFilter(Recovery()).
        WithFilter(Cors())

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
server.WithFilter(RetryWithBackoff(maxRetries = 3, initialBackoff = 100 * time.Millisecond))

// Bulkhead: limit concurrent requests
server.WithFilter(Bulkhead(maxConcurrent = 100))
```

## Filter Algebra

Compose and conditionally apply filters:

```gala
// Compose two filters into one
val secured = ComposeFilters(Auth(), RateLimit(100.0, 10.0))

// Apply filter only when predicate is true
val authWhenNotHealth = When((req) => req.Path() != "/health", Auth())

// Skip filter for specific paths
val logExceptHealth = Skip(Logger(), "/health", "/ready")
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

- [GALA](https://github.com/martianoff/gala) compiler (dev build or v0.21.0+)
- [Go SDK](https://go.dev/dl/) 1.25+ on PATH
- [Bazel](https://github.com/bazelbuild/bazelisk) (via Bazelisk) — optional, for Bazel builds

## Build & Test

### Using GALA CLI (recommended)

```shell
# Transpile and compile-check (library)
gala build

# Transpile and run all 235 tests
gala test
```

### Using Bazel

```shell
# Run all tests
bazel test //:server_test //:filter_test //:integration_test

# Run a specific test target
bazel test //:server_test

# Build library only
bazel build //...
```

> **Note:** `MODULE.bazel` uses `local_path_override` pointing to the local GALA checkout (`../gala_simple`). Update the path if your GALA repo is elsewhere.

## Project Structure

```
gala-server/
  types.gala          # Core types: Method, StatusCode, HTTPError, ErrorMapper, etc.
  request.gala        # Request API (params, query, headers, body, context, content negotiation)
  response.gala       # Response constructors and builders (including Negotiate)
  filter.gala         # All built-in filters (35+) including resilience and filter algebra
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

235 tests across 3 test files — **all passing** via `gala test` and `bazel test`:

- **Server builder**: NewServer, WithPort, WithName, WithDebug, WithBanner, immutability, Any method
- **Response constructors**: all 16 status codes, content types (JSON, HTML, XML, Text), JSONP, blobs, redirects, streams
- **Response builder**: WithHeader, WithBody, WithStatus, cookies (set, secure, delete), immutability
- **Sealed types**: Method (7 variants), StatusCode (13 variants)
- **Request API**: params, query params, headers, body, form, cookies, context, host, scheme, TLS, RealIP
- **Filters**: Logger, Recovery, CORS, Auth, AuthBearer, BasicAuth, KeyAuth, RateLimit, RateLimitPerIP, Timeout, Gzip, RequestId, Secure, BodyLimit, CSRF, HTTPSRedirect, WWWRedirect, NonWWWRedirect, TrailingSlash, RemoveTrailingSlash, MethodOverride, BodyDump, Decompress, Rewrite, JWTAuth, Proxy
- **Groups**: nesting, composition, filter scoping, filter isolation
- **HTTPError**: structured errors, JSON responses
- **Route naming**: URL generation with parameter substitution
- **JWT**: token creation, validation, expiry, claims, signature verification
- **Integration**: full HTTP request/response cycle with real server
