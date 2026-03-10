# Route Groups

Groups let you organize routes under a common prefix and apply shared filters.

## Creating a Group

Use `NewGroup()` to create a route group (not `NewServer()` — groups cannot listen):

```gala
val users = NewGroup().
    GET("/", listUsers).
    GET("/{id}", getUser).
    POST("/", createUser).
    DELETE("/{id}", deleteUser)
```

## Mounting Groups

Use `Group(prefix, group)` to mount a group under a prefix:

```gala
val server = NewServer().
    Group("/api/v1/users", users).
    Listen(":8080")
```

This registers:
- `GET /api/v1/users/`
- `GET /api/v1/users/{id}`
- `POST /api/v1/users/`
- `DELETE /api/v1/users/{id}`

## Groups with Filters

Filters on a group apply only to that group's routes:

```gala
val publicRoutes = NewGroup().
    GET("/health", health).
    GET("/docs", docs)

val protectedApi = NewGroup().
    GET("/users", listUsers).
    POST("/users", createUser).
    WithFilter(Auth())

val server = NewServer().
    Group("", publicRoutes).
    Group("/api", protectedApi).
    WithFilter(Logger())
```

- `/health` and `/docs` get Logger only
- `/api/users` gets Logger + Auth

## Nested Groups

Groups can be composed by mounting groups into other groups, then mounting into a server:

```gala
val users = NewGroup().
    GET("/", listUsers).
    POST("/", createUser)

val posts = NewGroup().
    GET("/", listPosts).
    POST("/", createPost)

val v1 = NewGroup().
    Group("/users", users).
    Group("/posts", posts).
    WithFilter(Auth())

val v2 = NewGroup().
    Group("/users", users)

val server = NewServer().
    Group("/api/v1", v1).
    Group("/api/v2", v2).
    WithFilter(Logger())
```

## WithRoutes

`WithRoutes(group)` is shorthand for `Group("", group)` — merges routes without a prefix:

```gala
val common = NewGroup().
    GET("/health", health).
    GET("/version", version)

val server = NewServer().
    WithRoutes(common).
    GET("/", home)
```

## Server vs Group

| | `Server` | `Group` |
|---|---|---|
| Constructor | `NewServer()` | `NewGroup()` |
| Has `Listen()` | Yes | No |
| Has `GET/POST/...` | Yes | Yes |
| Has `WithFilter()` | Yes (global) | Yes (scoped) |
| Has `Group()` | Yes | Yes |
| Purpose | Top-level app | Composable route set |

Both are immutable — every method returns a new value.
