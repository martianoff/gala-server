# Getting Started

## Installation

Add `gala-server` as a dependency in your `MODULE.bazel`:

```python
bazel_dep(name = "gala-server", version = "0.1.0")
```

## Quick Start

```gala
package main

import . "martianoff/gala-server"

func main() {
    val server = NewServer().
        GET("/", (req) => Ok("Hello, GALA!")).
        GET("/health", (req) => Json("{\"status\": \"ok\"}")).
        WithFilter(Logger()).
        WithFilter(Recovery())

    server.Listen(":8080")
}
```

## Core Concepts

### Server

`NewServer()` creates an immutable HTTP server builder. Chain methods to add routes and filters, then call `Listen(addr)` to start.

```gala
val server = NewServer().
    GET("/users", listUsers).
    POST("/users", createUser).
    WithFilter(Logger()).
    Listen(":8080")
```

### Handlers

Handlers are functions that take a `Request` and return a `Response`:

```gala
func hello(req Request) Response = Ok("Hello!")

func getUser(req Request) Response {
    val id = req.Param("id").GetOrElse("0")
    return Json(s"{\"id\": \"$id\"}")
}
```

### Request

Access request data through the `Request` type:

```gala
req.Method()                    // sealed type: GET(), POST(), etc.
req.Path()                      // "/users/123"
req.Body()                      // request body as string
req.Param("id")                 // path param → Option[string]
req.QueryParam("page")          // query param → Option[string]
req.Header("Authorization")     // header → Option[string]
req.ContentType()               // Content-Type header → Option[string]
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
InternalError("body")           // 500

// Content types
Json("{\"key\": \"value\"}")    // 200 + application/json
Text("plain text")              // 200 + text/plain
Html("<h1>Hello</h1>")          // 200 + text/html
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
server.GET("/users", handler)           // exact match
server.GET("/users/{id}", handler)      // path parameter
server.GET("/files/{path...}", handler) // wildcard (rest of path)
```
