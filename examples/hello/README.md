# GALA Server — Full Feature Demo

Demonstrates the complete gala-server feature set in a single runnable example.

## Features Shown

- Immutable server builder with fluent API
- Route groups with scoped filters (public, authenticated, admin)
- 10+ built-in filters (Logger, Recovery, CORS, Auth, RateLimit, RequestId, ETag, etc.)
- Multiple auth strategies (header, Bearer, BasicAuth, API key)
- Content negotiation (JSON/HTML/text based on Accept header)
- Server-Sent Events (SSE)
- Type-safe extractors (PathParam, QueryInt)
- Health check and readiness endpoints
- Error mapping
- Base path / API versioning (`/api` prefix)
- Named routes and URL generation
- Cookie handling
- Warmup handler
- Graceful shutdown

## Run

```bash
bazel run //examples/hello
```

## Test

With the server running, open another terminal:

```bash
# Basic
curl http://localhost:8080/api/
curl http://localhost:8080/api/health
curl http://localhost:8080/api/ready

# Content negotiation
curl -H "Accept: application/json" http://localhost:8080/api/public/negotiate
curl -H "Accept: text/html" http://localhost:8080/api/public/negotiate

# Search with query params
curl "http://localhost:8080/api/public/search?q=gala&page=2"

# SSE stream
curl http://localhost:8080/api/public/sse

# Auth required
curl http://localhost:8080/api/v1/users                          # 401
curl -H "Authorization: Bearer x" http://localhost:8080/api/v1/users  # 200

# Basic auth
curl -u admin:password http://localhost:8080/api/basic

# API key
curl -H "X-API-Key: my-secret-key" http://localhost:8080/api/api-key

# Admin (bearer)
curl -H "Authorization: Bearer admin-secret" http://localhost:8080/api/admin/stats

# 404
curl http://localhost:8080/api/nonexistent
```
