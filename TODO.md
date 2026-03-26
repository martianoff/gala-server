# GALA Server — Feature Roadmap

## Tier 1: Table Stakes
- [x] TLS/HTTPS Support (`ListenTLS`, `ListenTLSOn`, `ListenGracefulTLS`, `ListenGracefulTLSOn`)
- [x] Base Path / API Prefix (`WithBasePath`, prepend to all route patterns)
- [x] Content Negotiation (`Accepts`, `AcceptsJSON`, `AcceptsHTML`, `AcceptsXML`, `Negotiate`)
- [x] Extended StatusCode sealed type (`StatusUnprocessableEntity`, `StatusTooManyRequests`, `StatusRequestTimeout`, `StatusBadGateway`)
- [x] Context Propagation (`req.Deadline()`, `req.IsCancelled()`)

## Tier 2: Power Features
- [x] Type-Safe Extractors (`PathParam`, `PathParamInt`, `QueryRequired`, `QueryInt`, `HeaderRequired`, `BodyAs`)
- [x] Server-Sent Events (`SSEEvent`, `SSE`, `SSEStream`)
- [x] Filter Algebra (`ComposeFilters`, `When`, `Skip`, `Use`)
- [x] Health Check & Readiness (`WithHealthCheck`, `WithReadiness`)
- [x] Error Mapping (`ErrorMapper` type, `WithErrorMapper`)
- [x] ETag / Caching Filter (`ETag()`, `CacheControl()`, `NoCacheFilter()`)

## Tier 3: Resilience (Finagle-inspired)
- [x] Circuit Breaker Filter (`CircuitBreaker` with closed/open/half-open state machine)
- [x] Retry Filter (`Retry`, `RetryWithBackoff` with exponential backoff)
- [x] Bulkhead / Concurrency Limiter (`Bulkhead` with semaphore)
- [x] Warmup Handler (`WithWarmup` — runs before accepting traffic)

## Tier 4: Additional Improvements
- [x] CUSTOM Method case for sealed type (`CUSTOM(MethodName string)`)
- [x] Response.copy() private method (refactored builder methods to use it)
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
