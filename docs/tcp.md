# TCP — serving protocols that are not HTTP

A `Server` is protocol-agnostic: it owns identity, port, banner, warmup and
graceful shutdown. What it *speaks* is chosen by which method you call.

```gala
// HTTP
NewServer().WithPort(8080).ServeHTTP(NewHTTP().GET("/", handler))

// Anything else
NewServer().WithPort(7070).ServeTCP(handle)
```

Redis's RESP, memcached, SMTP and line-oriented control ports all need a byte
stream rather than a router. `ServeTCP` gives you one.

## How this relates to `ServeHTTP`

Both are `Server` methods, so port, name, banner, warmup and `ShutdownTimeout`
are configured identically. Only the protocol differs.

| | `ServeHTTP(HTTP)` | `ServeTCP(handler)` |
|---|---|---|
| Protocol | HTTP/1.1, HTTPS | anything over a TCP byte stream |
| Unit of work | `Request` → `Response` | a `Conn` you read and write yourself |
| Framing | done for you | yours to define |
| Routing, filters, sessions | on the `HTTP` value | not applicable |
| Graceful shutdown | yes | yes |

Routing and middleware live on `HTTP` rather than on `Server` precisely so this
works: a RESP server has no routes, and should not have to carry a routing table
to get a port and a shutdown timeout.

### Running both in one binary

Two ports, same configuration style:

```gala
func main() {
    // Admin/metrics over HTTP
    go_interop.Spawn(() =>
        NewServer().WithPort(8080).ServeHTTP(NewHTTP().
            WithHealthCheck("/health").
            WithMetricsEndpoint("/metrics", NewMetrics())))

    // The protocol itself over TCP
    NewServer().
        WithName("my-protocol").
        WithPort(7070).
        WithShutdownTimeout(15 * time.Second).
        ServeTCP(handle)
}
```

That is the shape a real protocol server wants: the protocol on its own port,
and HTTP for what HTTP is good at — health checks, Prometheus scraping, an
admin API.

## API

Nothing in a signature is a Go type. No `[]byte`, no `net.Conn`, no
`(value, error)` pairs. Callers see `string`, `int`, `Option` and `Try`.

### Starting

| Method | Returns | Notes |
|---|---|---|
| `Server.ServeTCP(handler func(Conn))` | `Try[bool]` | binds the Server's configured port |
| `Server.ServeTCPOn(addr string, handler func(Conn))` | `Try[bool]` | explicit address — a specific interface, or `":0"` for an OS-assigned port |

There is deliberately **no** free `ListenTCP` and no exported `Listener`. An
earlier draft had both. They duplicated `ServeTCP` while quietly skipping the
banner, the warmup hook and — most importantly — graceful shutdown. Two ways to
start a server, one of which silently lacks a drain, is worse than one way.

If something ever genuinely needs the raw accept loop (a protocol multiplexer,
say), that is a small additive API. It should be added when something needs it,
not kept on speculation.

### Conn

The handler receives one `Conn` per connection.

| Method | Returns | Notes |
|---|---|---|
| `ReadLine(maxBytes int)` | `Option[string]` | reads through `\n`, strips a trailing `\r\n` or `\n`. `None` on close, read error, or a line longer than `maxBytes` |
| `ReadExactly(n int, skip int)` | `Option[string]` | reads exactly `n` bytes, then discards `skip` more (a trailing delimiter). `None` on short read |
| `Write(s string)` | — | buffered; nothing reaches the socket until `Flush` |
| `WriteAll(parts Array[string])` | — | writes several strings in order |
| `Buffered()` | `int` | bytes already read and waiting |
| `Flush()` | — | |
| `Close()` | — | flushes, then closes |
| `RemoteAddr()` | `string` | |

## Three things the API is opinionated about

### Payloads are `string`, never `[]byte`

A byte slice is mutable, aliasable, and carries two plausible lengths. A Go
string is an immutable arbitrary byte sequence — which is what a binary-safe
protocol payload actually is. Every length in this API is **bytes**.

If you carry payloads as `string` in your own code, use `.ByteSize()` for
anything that touches the wire, never `.Size()`. `.Size()` counts runes; using
it for a length prefix silently truncates any non-ASCII payload and
desynchronises the connection several commands later, nowhere near the cause.

### `ReadExactly` is length-driven, not delimiter-driven

A binary payload may contain the delimiter, so reading to one would truncate
it. The natural repair — read to the delimiter in a loop until `n` bytes
accumulate — is a denial-of-service vector: a peer that declares a tiny length
and then streams gigabytes without a delimiter makes the first read allocate
all of it before any check runs.

The implementation is bounded by construction. The per-connection scratch
buffer is reused, so the steady-state cost of a bulk payload is one allocation
— the returned string, which is irreducible because the payload *is* that
string.

### `Buffered()` exists for pipelining

Flush only when nothing further is already buffered:

```gala
if !running || c.Buffered() == 0 {
    c.Flush()
}
```

A client that pipelines several commands in one write then gets one syscall
back instead of one per reply. For Redis-like protocols under
`redis-benchmark -P 16` this is the difference between one write and sixteen.

## Graceful shutdown

`ServeTCP` honours the Server's `ShutdownTimeout`. On SIGINT/SIGTERM it stops
accepting, lets in-flight connections finish, and force-closes whatever remains
once the timeout expires.

Go's standard library has `http.Server.Shutdown` but no equivalent for a bare
`net.Listener`, so the accounting is done in `httpcore.TCPServer`: a wait group
tracks live handlers and a registry of open connections provides the
force-close path. A forced close surfaces as a failed read or write in the
handler, so handlers unwind rather than being killed mid-operation.

## Concurrency, stated plainly

`ServeTCP` contains the **only** `go_interop.Spawn` in the package, so an
application built on it has none of its own.

It cannot be a `concurrent.Future`. `Future` is GALA's `Sendable`-checked
concurrency boundary: the compiler verifies that the closure crossing it
captures only deeply-immutable values (`GALA-E0037`). A live connection owns a
socket and mutable buffers, so it is not `Shareable`, and the check would
reject it — **correctly**. There is no way to express "run this connection
concurrently" that satisfies the checker today, because the thing being run is
genuinely mutable shared state.

So: this package does not make your protocol server data-race-free. It
concentrates the unchecked concurrency into one reviewed function in a
dependency, instead of leaving a raw `Spawn` in every application.

Write your handler as a **top-level function taking the `Conn`**, not a closure
reaching into enclosing state. That shape keeps each connection independently
readable and is what makes the unchecked zone reviewable.

## Complete example

See [`examples/tcp-echo`](../examples/tcp-echo) for a runnable line protocol
with `ECHO`, a length-prefixed `LEN` command that round-trips binary payloads,
per-connection state, and `QUIT`.

```
$ bazel run //examples/tcp-echo

   tcp-echo
   TCP listening on [::]:7070
   Graceful shutdown: 10s

$ nc localhost 7070
ECHO hello
hello
LEN 5
abcde
GOT 5 abcde
QUIT
BYE
```
