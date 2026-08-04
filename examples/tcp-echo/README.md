# tcp-echo

A minimal non-HTTP protocol server built on `Server.ServeTCP`.

`Server` speaks HTTP; this is the other entry point, for protocols that need a
raw byte stream. See [../../docs/tcp.md](../../docs/tcp.md) for the full API and
how the two relate.

## Run

```
bazel run //examples/tcp-echo
```

## Protocol

| Command | Reply |
|---|---|
| `ECHO <text>` | `<text>` |
| `LEN <n>` | reads exactly `n` more bytes, replies `GOT <n> <payload>` |
| `TIME` | `COMMANDS <n>` — a per-connection counter |
| `QUIT` | `BYE`, then closes |
| anything else | `ERR unknown command` |

## Try it

```
$ nc localhost 7070
ECHO hello
hello
TIME
COMMANDS 2
LEN 5
abcde
GOT 5 abcde
QUIT
BYE
```

## What it demonstrates

- **`ReadLine`** for delimiter-framed commands, with a byte cap so a peer cannot
  make the server buffer without bound.
- **`ReadExactly(n, skip)`** for length-prefixed binary payloads — `LEN` will
  round-trip a payload containing `\r\n` or NUL bytes, which a delimiter-based
  read would truncate.
- **`Buffered()`** to flush only when nothing further is pending, so a pipelined
  batch costs one syscall rather than one per reply.
- **Per-connection state** (the `TIME` counter) held in the handler, which is a
  top-level function taking the `Conn` rather than a closure over enclosing
  state.
