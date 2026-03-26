# GALA Server Benchmarks

Comparative benchmarks between GALA Server, Go net/http, Echo, and Gin.

## Setup

```bash
cd benchmark
go mod tidy
```

## Running

```bash
# Full benchmark suite (5 runs, 3s per benchmark)
go test -bench=. -benchmem -count=5 -benchtime=3s | tee results.txt

# Quick run (single count)
go test -bench=. -benchmem

# Specific framework
go test -bench=BenchmarkGALA -benchmem
go test -bench=BenchmarkNetHTTP -benchmem
go test -bench=BenchmarkEcho -benchmem
go test -bench=BenchmarkGin -benchmem

# Compare results with benchstat
go install golang.org/x/perf/cmd/benchstat@latest
benchstat results.txt
```

## What's Measured

All benchmarks use `httptest.NewRecorder` + `ServeHTTP` — pure handler throughput with no TCP overhead.

| Benchmark | Description |
|-----------|-------------|
| HelloWorld | Simple text response |
| JSON | JSON response with Content-Type header |
| PathParam | Path parameter extraction (/users/{id}) |
| PostEcho | POST request body reading and echo |
| Headers | Multiple response headers |

## GALA Bridge Overhead

GALA transpiles to Go and adds a thin bridge layer:
1. `BridgeRequest` struct construction with lazy closure accessors
2. `BridgeResponse` struct + header map allocation
3. Response header copy-back loop

The benchmark simulates this exact code path. The overhead is typically 1-2 allocations per request beyond what raw net/http does — the bridge is intentionally thin.

## Expected Results

GALA Server performance should be very close to raw net/http since:
- It uses Go's `http.ServeMux` for routing (same as net/http)
- The bridge adds only a struct allocation + map for headers
- No reflection, no middleware framework overhead at the HTTP level
- Filters (middleware) add their own allocations but are opt-in

Echo and Gin use custom routers (radix tree) which may be faster for route matching with many routes, but for simple cases net/http's ServeMux is competitive.
