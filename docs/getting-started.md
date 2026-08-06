# Getting Started

## Installation

Add `gala-server` as a dependency in your `MODULE.bazel`:

```python
bazel_dep(name = "gala-server", version = "0.1.0")
```

## Quick Start

```gala
package main

import . "github.com/martianoff/gala-server"

func main() {
    val app = NewHTTP().
        GET("/", (req) => Ok("Hello, GALA!")).
        GET("/health", (req) => JsonResponse("{\"status\": \"ok\"}")).
        WithFilter(Logger()).
        WithFilter(Recovery())

    NewServer().WithPort(8080).ServeHTTPGraceful(app)
}
```

## Core Concepts

### Server and HTTP

Two builders, deliberately separate. `NewHTTP()` describes the protocol —
routes, filters, error handling. `NewServer()` describes the host — name, port,
shutdown timeout, banner, warmup. You hand the protocol to the host.

The split exists because gala-server also serves protocols that are not HTTP
(see [tcp.md](tcp.md)): a port and a shutdown timeout are not HTTP concepts,
but routes and filters are.

```gala
val app = NewHTTP().
    GET("/users", listUsers).
    POST("/users", createUser).
    WithFilter(Logger())

NewServer().WithPort(8080).ServeHTTPGraceful(app)
```

### Handlers

Handlers are functions that take a `Request` and return a `Response`:

```gala
func hello(req Request) Response = Ok("Hello!")

func getUser(req Request) Response {
    val id = req.Param("id").GetOrElse("0")
    return JsonResponse(s"{\"id\": \"$id\"}")
}
```

### Request

Access request data through the `Request` type:

```gala
req.Method()                    // sealed type: GET(), POST(), CUSTOM("PURGE"), etc.
req.Path()                      // "/users/123"
req.Body()                      // request body as string
req.Param("id")                 // path param -> Option[string]
req.QueryParam("page")          // query param -> Option[string]
req.Header("Authorization")     // header -> Option[string]
req.ContentType()               // Content-Type header -> Option[string]
```

### Response Builders

Convenience functions for common HTTP responses:

```gala
// Status codes
Ok("body")                      // 200
Created("body")                 // 201
Accepted("body")                // 202
NoContent()                     // 204
BadRequest("body")              // 400
Unauthorized("body")            // 401
Forbidden("body")               // 403
NotFound("body")                // 404
UnprocessableEntity("body")     // 422
TooManyRequests("body")         // 429
InternalError("body")           // 500
BadGateway("body")              // 502
ServiceUnavailable("body")      // 503

// Content types
JsonResponse("{\"key\": \"value\"}")   // 200 + application/json
Text("plain text")                     // 200 + text/plain
Html("<h1>Hello</h1>")                 // 200 + text/html
JsonWithStatus(StatusCreated(), "{\"id\": 1}")

// Redirects
Redirect("/new-url")            // 302
RedirectPermanent("/new-url")   // 301

// Chaining
Ok("hello").
    WithHeader("X-Custom", "value").
    WithStatus(StatusAccepted())
```

### Route Patterns

Patterns use Go 1.22+ syntax:

```gala
app.GET("/users", handler)           // exact match
app.GET("/users/{id}", handler)      // path parameter
app.GET("/files/{path...}", handler) // wildcard (rest of path)
```

## TLS / HTTPS

Serve over HTTPS by providing a certificate and key:

```gala
val app = NewHTTP().
    GET("/", (req) => Ok("Secure!")).
    WithFilter(Logger())

val server = NewServer().WithPort(443)

// Basic TLS
server.ServeHTTPTLS(app, "cert.pem", "key.pem")

// TLS with graceful shutdown
server.ServeHTTPGracefulTLS(app, "cert.pem", "key.pem")
```

## Type-Safe Extractors

Instead of manually unwrapping `Option` values from request parameters, use extractors that return `Try[T]` for clean error handling:

```gala
import . "github.com/martianoff/gala-server"

func getUser(req Request) Response =
    PathParamInt(req, "id") match {
        case Success(id) => Ok(s"User $id")
        case Failure(err) => BadRequest(s"Invalid ID: $err")
    }

func search(req Request) Response {
    val query = QueryRequired(req, "q")
    val page = QueryInt(req, "page", 1)
    return query match {
        case Success(q) => Ok(s"Searching for $q, page $page")
        case Failure(err) => BadRequest(err.Error())
    }
}
```

Available extractors: `PathParam`, `PathParamInt`, `QueryRequired`, `QueryInt`, `HeaderRequired`, `BodyAs[T]`.

## Content Negotiation

Respond with different formats based on the client's `Accept` header:

```gala
func handler(req Request) Response =
    Negotiate(req,
        () => JsonResponse("{\"msg\": \"hello\"}"),
        () => Html("<p>hello</p>"),
        () => Text("hello"),
    )
```

## Metrics (Prometheus)

Track request count, latency, and status codes with a built-in Prometheus endpoint:

```gala
val stats = NewMetrics()

val app = NewHTTP().
    WithFilter(MetricsFilter(stats)).
    WithMetricsEndpoint("/metrics", stats).
    GET("/hello", (req) => Ok("hello"))

NewServer().ServeHTTPGraceful(app)
```

Visit `/metrics` to see Prometheus-format stats: `gala_requests_total`, `gala_responses_total{status}`, `gala_route_latency_avg_ms{route}`, and more.

## Sessions

Cookie-based in-memory sessions for stateful request handling:

```gala
val sessions = NewSessions("my-secret")

val app = NewHTTP().
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

Sessions are stored in-memory with concurrent access support. Configure with `WithCookieName`, `WithMaxAge`, `WithSecure`.

## Next Steps

- [API Reference](api-reference.md) -- complete reference for all types, constructors, metrics, sessions, and filters
- [Filters](filters.md) -- filter system with three scopes, filter algebra, and resilience patterns
- [Route Groups](groups.md) -- organizing routes with shared prefixes and filters
