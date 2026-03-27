# GALA Server

A fast, immutable HTTP server library for the [GALA language](https://github.com/martianoff/gala). Inspired by Twitter Finagle's filter model, Scala's immutable builders, and Go Echo's ergonomics.

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
  types.gala          # Core types: Method, StatusCode, HTTPError, Cookie, etc.
  request.gala        # Request API (params, query, headers, body, context)
  response.gala       # Response constructors and builders
  filter.gala         # All built-in filters (middleware)
  group.gala          # Route groups with scoped filters
  server.gala         # Server builder and listener
  router.gala         # Route type and registration
  httpcore/            # Go bridge (~400 lines) — the only Go code
    httpcore.go        # BridgeRequest, BridgeResponse, ServerBuilder
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
|  server, router  (all .gala)                |
+---------------------------------------------+
|           httpcore Bridge (Go)               |
|  BridgeRequest, BridgeResponse,             |
|  ServerBuilder, TokenBucket, HMACSigner     |
+---------------------------------------------+
|              Go net/http                     |
+---------------------------------------------+
```

The GALA layer is purely functional and immutable. The httpcore bridge is a thin Go adapter that translates between `net/http` and GALA's type system. All HTTP types (Request, Response, Method, StatusCode) are GALA-native sealed types.

## Documentation

- [Getting Started](docs/getting-started.md) — installation, core concepts, first server
- [API Reference](docs/api-reference.md) — complete reference for all types, responses, request API, filters
- [Filters](docs/filters.md) — middleware system: three scopes, execution order, built-in filters, custom filters
- [Route Groups](docs/groups.md) — grouping routes with shared prefixes and scoped filters
- [Transpiler Notes](TRANSPILER_NOTES.md) — known transpiler issues and workarounds for gala-server development

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
