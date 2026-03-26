# API Reference

Complete reference for gala-server types, functions, and filters.

## Response Constructors

### Status Code Constructors

| Function | Status | Description |
|----------|--------|-------------|
| `Ok(body)` | 200 | Success |
| `Created(body)` | 201 | Resource created |
| `Accepted(body)` | 202 | Accepted for processing |
| `NoContent()` | 204 | No content |
| `BadRequest(body)` | 400 | Client error |
| `Unauthorized(body)` | 401 | Authentication required |
| `Forbidden(body)` | 403 | Access denied |
| `NotFound(body)` | 404 | Resource not found |
| `MethodNotAllowed()` | 405 | Method not allowed |
| `Conflict(body)` | 409 | Conflict |
| `UnprocessableEntity(body)` | 422 | Unprocessable |
| `TooManyRequests(body)` | 429 | Rate limited |
| `RequestTimeout()` | 408 | Timeout |
| `InternalError(body)` | 500 | Server error |
| `BadGateway(body)` | 502 | Bad gateway |
| `ServiceUnavailable(body)` | 503 | Unavailable |

### Content-Type Constructors

| Function | Content-Type | Description |
|----------|-------------|-------------|
| `JsonResponse(body)` | application/json | JSON response |
| `Text(body)` | text/plain | Plain text |
| `Html(body)` | text/html | HTML |
| `Xml(body)` | application/xml | XML |
| `JSONP(callback, body)` | application/javascript | JSONP with callback |
| `Blob(status, ct, data)` | custom | Binary response |
| `File(path)` | auto-detect | Serve file |
| `Attachment(path, name)` | auto-detect | File download |
| `Inline(path, name)` | auto-detect | File inline |
| `Stream(status, ct, body)` | custom | Streaming response |
| `String(status, body)` | text/plain | Text with custom status |

### JSON Serialization (zero-reflection)

```gala
func JsonFrom[T any](value T, codec JsonEncoder[T]) Response
func JsonFromWithStatus[T any](status StatusCode, value T, codec JsonEncoder[T]) Response
func JsonPretty[T any](value T, codec JsonEncoder[T]) Response
```

### Blob Variants

| Function | Description |
|----------|-------------|
| `HTMLBlob(data)` | HTML from bytes |
| `JSONBlob(data)` | JSON from bytes |
| `XMLBlob(data)` | XML from bytes |
| `HTMLBlobWithStatus(status, data)` | HTML blob with custom status |
| `JSONBlobWithStatus(status, data)` | JSON blob with custom status |
| `XMLBlobWithStatus(status, data)` | XML blob with custom status |

### Redirect Helpers

| Function | Status | Description |
|----------|--------|-------------|
| `Redirect(url)` | 302 | Temporary redirect |
| `RedirectPermanent(url)` | 301 | Permanent redirect |

### Response Builders

All response builder methods return a new Response (immutable):

```gala
resp.WithHeader("X-Custom", "value")
resp.WithBody("new body")
resp.WithBodyBytes([]byte("bytes"))
resp.WithStatus(StatusCreated())
resp.WithCookie(cookie)
```

### Cookie Helpers

```gala
resp.SetCookie("name", "value", "/", 3600)
resp.SetSecureCookie("name", "value", "/", 3600)
resp.DeleteCookie("name")
```

## Request API

### Path Parameters

```gala
req.Param("id")         // Option[string]
req.ParamInt("id")      // Option[int]
req.ParamInt64("id")    // Option[int64]
```

### Query Parameters

```gala
req.QueryParam("q")               // Option[string]
req.QueryParamInt("page")         // Option[int]
req.QueryParamDefault("q", "*")   // string
req.QueryParams("tag")            // Array[string] (multi-value)
req.RawQuery()                    // string
```

### Headers & Cookies

```gala
req.Header("Authorization")       // Option[string]
req.ContentType()                  // Option[string]
req.Cookie("session")              // Option[string]
```

### Body & Form

```gala
req.Body()                         // string
req.FormValue("username")          // Option[string]
req.FormParams("tags")             // Array[string]
req.FormFile("avatar")             // ([]byte, string, error)
BindJson[T](req, codec)           // Try[T]
```

### Connection Info

```gala
req.Method()       // Method (sealed type: GET(), POST(), etc.)
req.Path()         // string
req.Host()         // string
req.Scheme()       // "http" or "https"
req.RealIP()       // string (checks X-Real-IP, X-Forwarded-For)
req.RemoteAddr()   // string
req.IsTLS()        // bool
req.IsWebSocket()  // bool
```

### Context Values

Per-request key-value store for passing data between filters and handlers:

```gala
req.CtxSet("user", "alice")
req.CtxGet("user")        // Option[string]
req.CtxGetInt("count")    // Option[int]
```

### Request Cloning

Used internally by filters like MethodOverride and Decompress:

```gala
req.WithMethod("PUT")       // Request with overridden method
req.WithBody("new body")    // Request with replaced body
```

## HTTPError

Structured errors using a sealed type for pattern matching:

```gala
sealed type HTTPError {
    case HTTPErr(Status StatusCode, Message string, Internal error)
}
```

### Constructors

```gala
NewHTTPError(StatusNotFound(), "user not found")
NewHTTPErrorWithInternal(StatusInternalError(), "db failed", dbErr)
```

### Methods

```gala
err.Code()           // int (e.g. 404)
err.Msg()            // string (e.g. "user not found")
err.Error()          // string (e.g. "code=404, message=user not found")
err.ToResponse()     // Response with status code and message body
err.ToJsonResponse() // JSON Response: {"error": "...", "code": 404}
```

## Server Configuration

### Builder Methods

```gala
NewServer().
    WithPort(8080).
    WithName("My API").
    WithDebug(true).
    WithBanner(false).
    WithShutdownTimeout(10 * time.Second).
    WithErrorHandler((err) => InternalError(err.Error())).
    WithNotFound((req) => NotFound("page not found")).
    WithRenderer(myRenderer).
    WithValidator(myValidator)
```

### Route Naming & URL Generation

```gala
val s = NewServer().
    GET("/users/{id}", handler).Named("user-detail")

s.URL("user-detail", "id", "42")  // Some("/users/42")
```

### Static File Serving

```gala
server.Static("/public/", "./static")
```

### Pluggable Interfaces

```gala
type Renderer interface { Render(name string, data any) string }
type Validator interface { Validate(i any) error }
```

### Listening

```gala
server.Listen(":8080")          // Start server
server.ListenGraceful()         // Start with graceful shutdown
```

## Filters (Middleware)

All filters have the type `func(Request, Handler) Future[Response]` and use `Future.Map`/`Future.Recover` for non-blocking composition.

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

### Security Headers

```gala
Secure()                                       // Default: XSS, HSTS, CSP, etc.
SecureWithConfig(xss, csp, hsts, frame, ct)    // Custom security header configuration
```

### CSRF Protection

```gala
CSRF("my-secret-key")   // HMAC-signed token validation
```

### CORS

```gala
Cors()                                                  // Allow all origins (*)
CorsWithOrigins("https://example.com")                  // Specific origin
CorsWithConfig(origins, methods, headers, maxAge)       // Full configuration
```

### Rate Limiting

```gala
RateLimit(1000.0, 100.0)      // Global token bucket (max tokens, refill/sec)
RateLimitPerIP(100.0, 10.0)   // Per-IP rate limiting
```

### Request Processing

```gala
BodyLimit("2M")                                 // Reject large requests (K, M, G suffixes)
BodyLimitBytes(2097152)                         // Reject by exact byte count
Gzip()                                          // Compress responses (Accept-Encoding: gzip)
Decompress()                                    // Decompress gzip request bodies
RequestId()                                     // Add X-Request-ID header + context value
Timeout(5 * time.Second)                        // Cancel slow handlers via Future racing
MethodOverride()                                // Override POST via X-HTTP-Method-Override or _method
BodyDump((reqBody, respBody) => ...)            // Debug: log request and response bodies
BodyDumpWithRequest((reqBody, respBody) => ...) // Same, different signature
```

### JWT Authentication

```gala
JWTAuth("my-secret-key")                     // Validate HS256 JWT with expiry check
JWTAuthWithConfig("my-secret-key", false)     // Validate HS256 JWT without expiry check
```

On success, stores claims in request context: `jwt-sub`, `jwt-iss`, `jwt-aud`, and `jwt-claims` (the full `*httpcore.JWTClaims` object).

### Reverse Proxy

```gala
Proxy("http://backend:3000")                           // Forward requests to backend
ProxyWithTimeout("http://backend:3000", 10 * time.Second) // With custom timeout
```

Terminal filter that replaces the handler response with the proxied backend response. Forwards headers, strips hop-by-hop headers, returns 502 on proxy failure.

### URL Manipulation

```gala
TrailingSlash()        // Add trailing slash to path
RemoveTrailingSlash()  // Remove trailing slash from path
HTTPSRedirect()        // Redirect HTTP requests to HTTPS
WWWRedirect()          // Redirect non-www to www
NonWWWRedirect()       // Redirect www to non-www
Rewrite(rules)         // URL path rewriting with wildcard matching
```

## Architecture

```
+---------------------------------------------+
|              GALA Server Layer               |
|  (server.gala, request.gala, response.gala,  |
|   filter.gala, group.gala, router.gala,      |
|   types.gala)                                |
+---------------------------------------------+
|           httpcore Bridge (Go)              |
|  (~400 lines - the ONLY Go code)            |
|  BridgeRequest, BridgeResponse,             |
|  ServerBuilder, TokenBucket, HMACSigner     |
+---------------------------------------------+
|              Go net/http                     |
+---------------------------------------------+
```
