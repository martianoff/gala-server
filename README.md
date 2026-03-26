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

- [Bazel](https://github.com/bazelbuild/bazelisk) (via Bazelisk)
- [Go SDK](https://go.dev/dl/) on PATH (for Go type inference during transpilation)

## Build & Test

```shell
# Build everything
bazel build //...

# Run all tests
bazel test //...

# Run tests with verbose output
bazel test //... --test_output=errors --verbose_failures

# Run a specific test target
bazel test //:server_test
bazel test //:filter_test
bazel test //:integration_test

# Regenerate BUILD files after adding/removing .gala files
bazel run //:gazelle

# Update Go dependencies
go mod tidy && bazel run //:gazelle && bazel run //:gazelle-update-repos && bazel run //:gazelle && bazel mod tidy
```

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

235 test functions across 3 test files covering server builder, all 16 response constructors, content-type constructors, blob variants, JSONP, redirects, response builders, cookies, sealed types, request API (params, query, headers, body, form, context, connection info), all 27+ filters (including JWT, Proxy), groups (nesting, composition, filter scoping), HTTPError, route naming/URL generation, JWT token creation/validation, and full HTTP integration tests.
