# GALA Server

An immutable, functional HTTP server library for the [GALA language](https://github.com/martianoff/gala).

```gala
package main

import . "martianoff/gala-server"

func main() {
    NewServer().
        WithPort(8080).
        GET("/", (req) => Ok("Hello, GALA!")).
        GET("/users/{id}", (req) => {
            val id = req.Param("id").GetOrElse("0")
            return Json(s"{\"id\": \"$id\"}")
        }).
        WithFilter(Logger()).
        Listen()
}
```

## Features

- **Immutable builder** -- every method returns a new Server, no mutation
- **Sealed types** for HTTP methods and status codes with exhaustive pattern matching
- **Three-level filter scoping** -- per-route, group, and global
- **Composable route groups** with prefix mounting
- **Built-in filters** -- Logger, Recovery, Cors, Auth
- **Thin Go bridge** -- ~120 lines of Go, everything else is pure GALA
- **Future-based handlers** -- sync handlers auto-wrapped in `Future[Response]`

## Installation

### Prerequisites

- [GALA](https://github.com/martianoff/gala) toolchain
- Go 1.25+

### Using `gala build` (recommended for simple projects)

```bash
# Create a new project
mkdir myapp && cd myapp
gala mod init github.com/user/myapp

# Add gala-server dependency
gala mod add github.com/martianoff/gala-server@v0.1.0

# Create main.gala (see Quick Start below)

# Build and run
gala build
./myapp
```

Your project structure:

```
myapp/
  gala.mod       # Module manifest
  gala.sum       # Dependency checksums (auto-generated)
  main.gala      # Your GALA code
```

### Using Bazel

For larger projects using Bazel:

**1. Add `gala-server` to `gala.mod`:**

```
module github.com/user/myapp

gala 1.0

require github.com/martianoff/gala-server v0.1.0
```

**2. Sync dependencies:**

```bash
gala mod tidy    # generates go.mod and go.sum for Bazel
```

**3. Configure `MODULE.bazel`:**

```python
bazel_dep(name = "gala", version = "1.0.0")
bazel_dep(name = "gala-server", version = "0.1.0")
bazel_dep(name = "rules_go", version = "0.59.0")
bazel_dep(name = "gazelle", version = "0.47.0")

go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//:go.mod")

# GALA dependencies
gala = use_extension("@gala//:extensions.bzl", "gala")
gala.from_file(gala_mod = "//:gala.mod")
```

For local development, add a path override:

```python
local_path_override(
    module_name = "gala-server",
    path = "../gala-server",
)
```

**4. Configure `BUILD.bazel`:**

```python
load("@gala//:gala.bzl", "gala_binary")

gala_binary(
    name = "myapp",
    src = "main.gala",
    gala_deps = [
        "@gala-server//:gala-server",
    ],
)
```

**5. Build:**

```bash
bazel build //:myapp
```

### Import in your GALA code

```gala
import . "martianoff/gala-server"
```

## Quick Start

### Minimal server

```gala
package main

import . "martianoff/gala-server"

func main() {
    NewServer().
        GET("/", (req) => Ok("Hello!")).
        Listen()
}
```

Default port is `8080`. Override with `WithPort(port)` or use `ListenOn(addr)` for a full address.

### Route parameters

Patterns use Go 1.22+ syntax: `/users/{id}`, `/files/{path...}`

```gala
GET("/users/{id}", (req) => {
    val id = req.Param("id").GetOrElse("0")
    return Json(s"{\"id\": \"$id\"}")
})
```

### Request API

```gala
req.Method()                  // Method sealed type (GET, POST, etc.)
req.Path()                    // URL path
req.Body()                    // Request body as string
req.Param("id")               // Path parameter -> Option[string]
req.QueryParam("page")        // Query parameter -> Option[string]
req.Header("Authorization")   // Header value -> Option[string]
req.ContentType()             // Content-Type header -> Option[string]
```

### Response helpers

```gala
Ok("body")                           // 200
Created("body")                      // 201
NoContent()                          // 204
BadRequest("body")                   // 400
Unauthorized("body")                 // 401
NotFound("body")                     // 404
InternalError("body")                // 500
Json("{\"key\": \"value\"}")         // 200 + application/json
Text("plain text")                   // 200 + text/plain
Html("<h1>Hello</h1>")               // 200 + text/html
Redirect("/new-url")                 // 302 + Location header
JsonWithStatus(StatusCreated(), body) // custom status + json
```

Chain `.WithHeader()`, `.WithBody()`, `.WithStatus()` for customization:

```gala
Ok("hello").
    WithHeader("X-Custom", "value").
    WithStatus(StatusAccepted())
```

## Filters

Filters wrap handlers with cross-cutting behavior. They are applied at listen time, not at route registration.

### Built-in filters

```gala
Logger()                      // Logs: GET /path -> 200 (1.2ms)
Recovery()                    // Catches panics, returns 500
Cors()                        // CORS headers with origin *
CorsWithOrigins("example.com") // CORS with specific origin
Auth()                        // Requires Authorization header
AuthWithValidator(fn)         // Custom auth validation
```

### Three-level scoping

```gala
// Per-route: Auth only on /secret
GET("/secret", handler, Auth())

// Group-level: Auth on all /api/* routes
val api = NewGroup().
    GET("/users", listUsers).
    POST("/users", createUser).
    WithFilter(Auth())

// Global: Logger on ALL routes
NewServer().
    Group("/api", api).
    WithFilter(Logger()).
    Listen()
```

### Custom filters

A filter is `func(Request, Handler) Future[Response]`:

```gala
func RateLimit(max int) Filter =
    (req Request, next Handler) => {
        // check rate limit...
        return next(req)
    }
```

## Route Groups

Groups compose routes under a prefix with scoped filters:

```gala
val v1 = NewGroup().
    GET("/users", listUsers).
    POST("/users", createUser).
    WithFilter(Auth())

val v2 = NewGroup().
    GET("/users", listUsersV2)

NewServer().
    GET("/health", (req) => Ok("ok")).
    Group("/api/v1", v1).
    Group("/api/v2", v2).
    WithFilter(Logger()).
    Listen()
```

Group filters apply only to routes within that group. They don't leak to the parent server or other groups.

## Server Configuration

```gala
NewServer().                  // defaults: port 8080, name "GALA Server"
    WithPort(9090).           // custom port
    WithName("My API").       // shown in startup message
    Listen()                  // uses configured port

// Or use ListenOn for full address control
server.ListenOn("0.0.0.0:443")
```

## Architecture

```
gala-server/
  types.gala         -- Method, StatusCode sealed types + Handler/Filter aliases
  request.gala       -- Request wrapping Go bridge
  response.gala      -- Response + helpers (Ok, Json, etc.)
  router.gala        -- Route struct, filter application (FoldRight)
  group.gala         -- Group composition (FoldLeft)
  server.gala        -- Server builder, Listen
  filter.gala        -- Logger, Recovery, Cors, Auth
  httpcore/
    httpcore.go      -- Thin Go bridge (~120 lines, only net/http)
```

The Go bridge (`httpcore`) handles only `net/http` conversion. All types, routing, filters, and the builder API are pure GALA.

## License

MIT
