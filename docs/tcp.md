# TCP — serving protocols that are not HTTP

gala-server's `Server` type speaks HTTP: routes, filters, sessions, SSE,
content negotiation. Plenty of protocols worth writing in GALA are not HTTP —
Redis's RESP, memcached, SMTP, line-oriented control ports — and those need a
byte stream, not a router.

`ListenTCP` / `Listener` / `Conn` are that second entry point.

## How this relates to `Server`

**They are two independent entry points in the same package, and are
deliberately not composed.**

An HTTP server owns routing, middleware and request/response objects. A RESP
server has no routes, no headers, and no notion of a request that ends at a
blank line. Plumbing TCP into `Server` would mean either giving `Server` a
second mode that ignores most of its own configuration, or giving TCP users a
routing table they never asked for. Both are worse than two entry points that
each do one thing.

| | `Server` | `ListenTCP` |
|---|---|---|
| Protocol | HTTP/1.1, HTTPS | anything over a TCP byte stream |
| Unit of work | `Request` → `Response` | a `Conn` you read and write yourself |
| Framing | done for you | yours to define |
| Routing, filters, sessions | yes | not applicable |
| Entry point | `NewServer()…Listen()` | `ListenTCP(addr)` |

Naming note: the TCP entry point is `ListenTCP`, not `Listen`, because
`Server.Listen()` already exists for HTTP. The compiler can tell a package
function from a method; a reader scanning a dot-imported file should not have
to.

### Running both in one binary

They are independent, so this is just two ports:

```gala
func main() {
    // Admin/metrics over HTTP
    go_interop.Spawn(() => NewServer().
        WithPort(8080).
        WithHealthCheck("/health").
        WithMetricsEndpoint("/metrics", NewMetrics()).
        Listen())

    // The protocol itself over TCP
    ListenTCP(":7070") match {
        case Success(ln) => ln.Serve(handle)
        case Failure(e)  => Println(s"listen failed: ${e.Error()}")
    }
}
```

That is the shape a real protocol server wants: the protocol on its own port,
and HTTP for what HTTP is good at — health checks, Prometheus scraping, an
admin API.

## API

Nothing in a signature is a Go type. No `[]byte`, no `net.Conn`, no
`(value, error)` pairs. Callers see `string`, `int`, `Option` and `Try`.

### Listener

| Function | Returns | Notes |
|---|---|---|
| `ListenTCP(addr string)` | `Try[Listener]` | `Failure` carries the bind error |
| `Listener.Accept()` | `Try[Conn]` | `Failure` means the listener is done |
| `Listener.Serve(handler func(Conn))` | — | accepts until the listener fails, one goroutine per connection |
| `Listener.Addr()` | `string` | useful when the OS picked the port (`":0"`) |
| `Listener.Close()` | — | |

### Conn

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

## Concurrency, stated plainly

`Listener.Serve` contains the **only** `go_interop.Spawn` in the package, so an
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
tcp-echo listening on [::]:7070

$ nc localhost 7070
ECHO hello
hello
LEN 5
abcde
GOT 5 abcde
QUIT
BYE
```
