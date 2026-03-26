# GALA Server — Feature Roadmap

## Completed

### Tier 1: Table Stakes (Echo Parity)
- [x] Immutable Server builder with `NewServer()`, `WithPort`, `WithName`, `WithDebug`, `WithBanner`
- [x] Route registration: `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD`, `OPTIONS`, `Any`, `Handle`
- [x] Route groups with scoped filters (`Group`, `WithRoutes`, `WithFilter`)
- [x] Route naming and URL generation (`Named`, `URL`)
- [x] Path parameters (`Param`, `ParamInt`, `ParamInt64`)
- [x] Query parameters (`QueryParam`, `QueryParamInt`, `QueryParamDefault`, `QueryParams`)
- [x] Headers, cookies, form data, JSON body binding
- [x] Full response constructor suite (16 status codes, content-type constructors, blob variants, JSONP, redirects)
- [x] JSON serialization with zero-reflection codecs (`JsonFrom`, `JsonFromWithStatus`, `JsonPretty`)
- [x] File serving (`File`, `Attachment`, `Inline`, `Static`)
- [x] HTTPError sealed type with `ToResponse()` and `ToJsonResponse()`
- [x] Validator and Renderer interfaces
- [x] Graceful shutdown (`ListenGraceful`, `ListenGracefulOn`)
- [x] Custom error handler (`WithErrorHandler`, `WithNotFound`)
- [x] Connection info (`RealIP`, `Host`, `Scheme`, `IsTLS`, `IsWebSocket`, `RemoteAddr`)
- [x] Context values (`CtxGet`, `CtxGetInt`, `CtxSet`)
- [x] Request cloning (`WithMethod`, `WithBody`)
- [x] 27+ built-in filters (Logger, Recovery, CORS, Auth, Bearer, BasicAuth, KeyAuth, JWT, CSRF, RateLimit, RateLimitPerIP, Timeout, Gzip, Decompress, RequestId, Secure, BodyLimit, BodyDump, MethodOverride, HTTPSRedirect, WWWRedirect, NonWWWRedirect, TrailingSlash, RemoveTrailingSlash, Rewrite, Proxy, ProxyWithTimeout)
- [x] TLS/HTTPS support (`ListenTLS`, `ListenTLSOn`, `ListenGracefulTLS`, `ListenGracefulTLSOn`)
- [x] Base path / API prefix (`WithBasePath` — prepends to all route patterns)
- [x] Content negotiation (`Accepts`, `AcceptsJSON`, `AcceptsHTML`, `AcceptsXML`, `Negotiate`)
- [x] Extended StatusCode sealed type (`StatusUnprocessableEntity`, `StatusTooManyRequests`, `StatusRequestTimeout`, `StatusBadGateway`)
- [x] Context propagation (`req.Deadline()`, `req.IsCancelled()`)

### Tier 2: Power Features (Beyond Echo)
- [x] Type-safe extractors (`PathParam`, `PathParamInt`, `QueryRequired`, `QueryInt`, `HeaderRequired`, `BodyAs[T]`)
- [x] Server-Sent Events (`SSEEvent`, `SSE`, `SSEStream`)
- [x] Filter algebra (`ComposeFilters`, `When`, `Skip`, `Use`)
- [x] Health check and readiness endpoints (`WithHealthCheck`, `WithReadiness`)
- [x] Error mapping (`ErrorMapper` type, `WithErrorMapper`)
- [x] ETag / caching filters (`ETag()`, `CacheControl()`, `NoCacheFilter()`)

### Tier 3: Resilience (Finagle-inspired)
- [x] Circuit breaker filter (`CircuitBreaker` with closed/open/half-open state machine)
- [x] Retry filters (`Retry` with fixed backoff, `RetryWithBackoff` with exponential backoff)
- [x] Bulkhead / concurrency limiter (`Bulkhead` with semaphore)
- [x] Warmup handler (`WithWarmup` — runs before accepting traffic)

### Tier 4: Additional Improvements
- [x] `CUSTOM(MethodName)` case for Method sealed type
- [x] `Response.copy()` private method (refactored builder methods to use it)
- [x] Improved `toMethod` default (uses `CUSTOM(name)` instead of silently returning `GET()`)

## Future Ideas
- [ ] WebSocket support (upgrade handler)
- [ ] HTTP/2 push
- [ ] OpenTelemetry / tracing integration
- [ ] Metrics endpoint (Prometheus format)
- [ ] Request validation via struct tags
- [ ] Multipart file upload streaming
- [ ] Session management (cookie-based, Redis-backed)
- [ ] GraphQL handler integration
- [ ] gRPC bridge
- [ ] Hot reload in development mode
- [ ] OpenAPI / Swagger spec generation
- [ ] Rate limiter with sliding window algorithm
- [ ] Distributed circuit breaker (Redis-backed)
