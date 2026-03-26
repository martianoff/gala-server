# Transpiler Notes — gala-server v2 Development

Found during implementation of the full framework features. These are potential transpiler issues or patterns that need verification.

**Last triaged:** 2026-03-24 by GALA Fix Team
**Last fix cycle:** 2026-03-24 — 15 PRs merged (#81–#95), 15 bugs fixed
**GALA version:** dev (post-fix, 216 bazel tests passing)
**Test results:** 216 bazel tests passing

> **Fix cycle 2 (2026-03-24):** 7 additional PRs (#89–#95) fixing all remaining codegen bugs:
> - BUG-039 → **FIXED** [PR #89](https://github.com/martianoff/gala/pull/89) — named param defaults for methods
> - BUG-044 → **FIXED** [PR #90](https://github.com/martianoff/gala/pull/90) — missing return before IIFE
> - BUG-040 → **FIXED** [PR #91](https://github.com/martianoff/gala/pull/91) — dot import consistency
> - BUG-041 → **FIXED** [PR #92](https://github.com/martianoff/gala/pull/92) — compiler rejects nil for val fields (use Option[T])
> - BUG-045 → **FIXED** [PR #93](https://github.com/martianoff/gala/pull/93) — unused import removal
> - BUG-032 → **DOC_GAP** — use Tuple/TupleN with destructuring (PR #95 closed)
> - BUG-046 → **FIXED** [PR #94](https://github.com/martianoff/gala/pull/94) — gala_go_test lib_srcs
> - BUG-043 → **VERIFIED** already fixed by BUG-036 (PR #86)

---

## Fixed Bugs (2026-03-24 fix cycle)

### BUG-028: Multi-line function parameters cause parse failure

**Status:** FIXED — [PR #81](https://github.com/martianoff/gala/pull/81)
**Severity:** High
**Root cause:** Grammar rules `parameterList` and `sealedCaseFieldList` did not support trailing commas. Added `','?` to both rules.

Multi-line parameter lists with trailing commas now work:
```gala
func SecureWithConfig(
    xssProtection string = "1; mode=block",
    contentTypeNosniff string = "nosniff",
    xFrameOptions string = "SAMEORIGIN",
) Filter = ...

struct Cookie(
    Name string,
    Value string,
    Path string,
)
```

**Action:** Remove single-line workarounds in `filter.gala`, `response.gala`, `server.gala`.

---

### BUG-029: Sealed type composite literal in `if` condition — ambiguous `{}`

**Status:** FIXED — [PR #82](https://github.com/martianoff/gala/pull/82)
**Severity:** High
**Root cause:** Transpiler did not parenthesize composite literals in if/for/switch conditions. Added `parenthesizeCompositeLits()` walker applied to all condition contexts.

Now generates `(OPTIONS{}).Apply()` instead of `OPTIONS{}.Apply()` in conditions.

**Action:** Remove `sed` post-processing for composite literal parenthesization.

---

### BUG-030: Match expression type mismatch — `Future[server.Response]` vs `Future[Response]`

**Status:** FIXED — [PR #83](https://github.com/martianoff/gala/pull/83)
**Severity:** High
**Root cause:** `typesCompatible` only checked dot-imports for qualified/unqualified comparison. Extended to check current package name and known imports, plus recursive base type comparison for generic type params.

**Action:** Remove if/else workarounds in `filter.gala` — match expressions should work now.

---

### BUG-031: `if`-expression not supported as `val` initializer

**Status:** FIXED — [PR #84](https://github.com/martianoff/gala/pull/84)
**Severity:** Medium
**Root cause:** Grammar's `ifExpression` only accepted bare expressions as branches. Added `ifExprBranch: block | expression` rule. Transformer extracts last expression from blocks as return value.

Both forms now work:
```gala
val status = if (score > 50) "pass" else "fail"
val url = if (q != "") { s"$base?$q" } else { base }
```

**Action:** Remove `var` + mutation workarounds — use if-expressions directly.

---

### BUG-033: `[]byte(...)` type conversion not supported

**Status:** RESOLVED — Not a bug, use `go_interop.ToBytes()`
**Severity:** Medium

`[]byte(...)` Go syntax is not supported in GALA's grammar. Use the existing `go_interop` helper:
```gala
import . "martianoff/gala/go_interop"
val data = ToBytes("hello")      // instead of []byte("hello")
val str = ToString(data)          // instead of string(data)
val runes = ToRunes("hello")     // instead of []rune("hello")
```

**Action:** Already using `httpcore.ToBytes()` — consider switching to `go_interop.ToBytes()` to remove the httpcore bridge function.

---

### BUG-034: Escaped quotes inside `${}` string interpolation

**Status:** FIXED — [PR #85](https://github.com/martianoff/gala/pull/85)
**Severity:** Low
**Root cause:** Two issues: (1) ANTLR `BRACE_CONTENT` fragment didn't handle escape sequences. (2) Go interpolation parser didn't unescape `\"` before re-parsing.

Now works:
```gala
Ok(s"${r.CtxGet(\"jwt-sub\").GetOrElse(\"none\")}")
```

**Action:** Remove variable extraction workarounds for escaped quotes in `filter_test.gala`.

---

### BUG-036: Standalone `gala transpile` generates wrong types without stdlib resolution

**Status:** FIXED — [PR #86](https://github.com/martianoff/gala/pull/86)
**Severity:** High
**Root cause:** Analyzer collected package resolution failures as warnings but transformer proceeded with incomplete type information. Now fails hard with a `SemanticError` listing all unresolved packages with a hint to use `--search` or `gala.mod`.

**Note:** This also partially addresses BUG-043 — the transpiler will no longer silently generate `any` types when packages can't be resolved.

---

### BUG-037 / BUG-042: `gala build` cannot build/test library packages

**Status:** FIXED — [PR #87](https://github.com/martianoff/gala/pull/87)
**Severity:** High (both)
**Root cause:** `gala build` called `checkIsExecutable()` which rejected non-main packages. Replaced with `isLibraryPackage()` and added `goCompileCheck()` method that runs `go build ./gen/...` for libraries.

`gala build` now:
- For `package main`: builds executable (unchanged)
- For library packages: runs `go build` compile check, prints "ok (library compiled successfully)"
- `gala run` rejects libraries with a clear error

**Action:** Use `gala build` to compile-check the server package. This also addresses BUG-042 (generated Go is now validated).

---

### BUG-038: `martianoff/gala/test` module missing from stdlib

**Status:** FIXED — [PR #88](https://github.com/martianoff/gala/pull/88)
**Severity:** High
**Root cause:** Test framework existed in repo at `test/` but was not in the stdlib embedding pipeline. Added `test` to `cmd/stdlib_gen`, `internal/build/gomod.go`, `internal/stdlib/BUILD.bazel`, and `internal/stdlib/stdlib.go`.

**Action:** Remove manually created `~/.gala/stdlib/v0.20.0/test/` — it will now be distributed automatically with `gala install`.

---

## Still Open Bugs

### BUG-032: Multi-value return types not supported in function signatures

**Status:** DOC_GAP — Not a transpiler bug. Use `Tuple[A, B]` or `TupleN[A, B, C]` with tuple destructuring.
**Severity:** Low (documentation)

Go-style multi-value returns `(A, B, error)` are not supported in GALA. Use Tuple types instead:

```gala
// WRONG: Go-style multi-return (not valid GALA)
// func FormFile(name string) ([]byte, string, error) = ...

// CORRECT: Use Tuple for multiple return values
func FormFile(name string) Tuple3[[]byte, string, string] = ...

// Destructure at call site (note parens)
val (data, contentType, err) = FormFile("upload")
```

For Go interop with functions that already return `(T, error)`, use `var`:
```gala
var data, err = goFuncReturningTwoValues()
```

---

### BUG-035: `defer` not supported (intentional design decision)

**Status:** WONTFIX — Intentional
**Severity:** Low

`defer` is intentionally not supported in GALA. Code requiring Go cleanup patterns (HTTP clients, file I/O) must use `.go` bridge files.

---

### BUG-039: Named parameter defaults not expanded in generated Go

**Status:** FIXED — [PR #89](https://github.com/martianoff/gala/pull/89)
**Severity:** ~~Critical~~

When a GALA function uses named parameters with defaults, calls that supply only some arguments transpile with only the supplied argument. Default values for omitted parameters are not expanded.

```gala
func (s Server) copy(root Group = s.root, port int = s.Port, ...) Server = ...
func (s Server) WithPort(port int) Server = s.copy(port = port)
```

```go
// Generated (wrong):
func (s Server) WithPort(port int) Server {
    return s.copy(port)  // ERROR: not enough arguments (need 11, got 1)
}

// Expected:
func (s Server) WithPort(port int) Server {
    return s.copy(s.root.Get(), port, s.Name.Get(), s.ErrorHandler.Get(), ...)
}
```

**Root cause:** Transpiler emits named-argument call as positional with only the supplied argument.

---

### BUG-040: Dot imports not preserved — unqualified references without import

**Status:** FIXED — [PR #91](https://github.com/martianoff/gala/pull/91)
**Severity:** ~~High~~

When GALA source uses `. "martianoff/gala/collection_immutable"`, the transpiler generates mixed qualified and unqualified references, and may omit the import entirely.

```go
// server.gen.go: no collection_immutable import at all, but uses both:
EmptyArray[std.Tuple[string, string]]()         // unqualified — ERROR: undefined
collection_immutable.Array[std.Tuple[...]]       // qualified — would work IF imported
```

**Root cause:** Inconsistent handling of dot imports in codegen.

---

### BUG-041: `std.NewImmutable(nil)` — compiler now rejects nil for val fields

**Status:** FIXED — [PR #92](https://github.com/martianoff/gala/pull/92)
**Severity:** ~~Medium~~

Compiler now emits `[SemanticError] cannot assign nil to immutable field 'X' — use Option[T] with None() for optional values`. Use `Option[T]` with `None()` instead of `nil`.

When a struct field is initialized to `nil`, the transpiler generates `std.NewImmutable(nil)`. Go cannot infer `T` from `nil`.

```go
// Generated (wrong):
renderer: std.NewImmutable(nil),      // ERROR: cannot infer T

// Expected:
renderer: std.NewImmutable[Renderer](nil),
```

---

### BUG-043: Standalone `gala transpile` infers lambda parameter/return types as `any`

**Status:** VERIFIED FIXED — BUG-036 fix (PR #86) fully resolves this
**Severity:** ~~High~~

The BUG-036 fix ([PR #86](https://github.com/martianoff/gala/pull/86)) makes the transpiler fail hard when packages can't be resolved, preventing silent `any` fallback. However, if resolution succeeds but type inference still falls through to `any` in edge cases, this could still occur.

**Action:** Re-test after upgrading to the fixed GALA version. If the transpiler now fails hard instead of generating `any`, this is effectively resolved.

---

### BUG-044: Missing `return` before anonymous function call in match-to-IIFE codegen

**Status:** FIXED — [PR #90](https://github.com/martianoff/gala/pull/90)
**Severity:** ~~High~~

When a `match` expression is the last expression in a function body, the transpiler generates an IIFE but omits the `return` keyword.

```go
// Generated (wrong):
func (s Server) URL(name string, params ...string) std.Option[string] {
    var route = ...
    func(obj std.Option[Route]) std.Option[string] { ... }(route.Get())  // missing return!
}

// Correct:
    return func(obj std.Option[Route]) std.Option[string] { ... }(route.Get())
```

---

### BUG-045: Unused import emitted in generated Go

**Status:** FIXED — [PR #93](https://github.com/martianoff/gala/pull/93)
**Severity:** ~~Low~~

The transpiler sometimes emits an import for `collection_immutable` in files that don't use it, causing a Go compilation error.

**Workaround:** Remove the unused import line from the generated file.

---

### BUG-047: Lambda return type inference fails for complex multi-return bodies

**Status:** OPEN — Type inference limitation
**Severity:** High

When a lambda has multiple return paths returning different expressions (e.g., `FutureOf(...)` on one path and `next(req)` on another), the transpiler may infer the wrong return type if the explicit type parameter is omitted from `FutureOf`/`FutureApply`.

```gala
// FAILS: transpiler infers lambda return type as `string` instead of `Future[Response]`
func JWTAuthWithConfig(secret string, checkExpiry bool) Filter =
    (req Request, next Handler) => {
        if ... { return FutureOf(Unauthorized("...")) }   // FutureOf without [Response]
        return next(req)
    }

// WORKS: explicit type param helps transpiler
    (req Request, next Handler) => {
        if ... { return FutureOf[Response](Unauthorized("...")) }
        return next(req)
    }
```

Also affects `FutureApply` in complex bodies (e.g., `proxyFilter` with `FoldLeft`):
```gala
// FAILS: transpiler infers return type as Future[[]string]
(req, next) => FutureApply(() => { ... ArrayOf(keys...).FoldLeft(...) })

// WORKS: explicit type param
(req, next) => FutureApply[Response](() => { ... ArrayOf(keys...).FoldLeft(...) })
```

**Root cause:** Lambda return type inference uses the first return expression's type, which may be incomplete when `FutureOf`/`FutureApply` type params are omitted and the body has complex control flow.

**Workaround:** Keep explicit `[Response]` type params on `FutureOf`/`FutureApply` in complex lambda bodies (multi-branch, FoldLeft). Simple single-return lambdas infer correctly without explicit params.

**Impact on lint:** GALA lint rule "Type Inference: redundant generic params" must have an exception for these cases. The type params are NOT redundant when the transpiler needs them for lambda return type inference.

---

### BUG-046: Bazel `gala_go_test` fails — missing library target

**Status:** FIXED — [PR #94](https://github.com/martianoff/gala/pull/94)
**Severity:** ~~High~~

`bazel test //:server_test` fails because the `gala_go_test` macro expects a `go_library` target `//:gala-server` which does not exist. The BUILD.bazel only defines `gala_transpile` rules.

**Root cause:** Either a `go_library` bundling all `*.gen.go` + `httpcore` is needed, or the `gala_go_test` macro should generate it.

---

## Usability Issues

### USABILITY-001: `gala test` fails — package conflict and local subpackage resolution

**Status:** FIXED — Issue A fixed by [PR #97](https://github.com/martianoff/gala/pull/97) (testgen/ separation), Issue B fixed by [PR #108](https://github.com/martianoff/gala/pull/108) (import rewriting)
**Severity:** ~~High~~

~~`gala test` has two issues:~~

**Issue A: conflicting packages in workspace.** ~~FIXED~~ — `gala test` now puts test files in `testgen/` with package rewriting to avoid the `server` vs `main` package conflict.

**Issue B: same as USABILITY-006** — ~~FIXED~~ — workspace now rewrites project module imports to workspace module paths (PR #108).

### USABILITY-002: Long single-line parameter lists

**Status:** RESOLVED — Fixed by BUG-028 fix
Trailing commas now supported. Use multi-line formatting.

### USABILITY-003: `match` expressions unusable for common filter patterns

**Status:** RESOLVED — Fixed by BUG-030 fix
Qualified vs unqualified type comparison now works in generic params.

### USABILITY-004: No way to compile-check `.gala` library files standalone

**Status:** RESOLVED — Fixed by BUG-037/042 fix
`gala build` now runs `go build` for library packages.

### USABILITY-005: GALA test functions require Go test harness wrapper

**Status:** WONTFIX — By design. `gala test` generates the harness automatically (PR #97). Manual wrappers only needed for `go test` without `gala test`.

### USABILITY-006: `gala build` workspace cannot resolve local subpackages

**Status:** FIXED — [PR #107](https://github.com/martianoff/gala/pull/107) (Bazel symlink crash) + [PR #108](https://github.com/martianoff/gala/pull/108) (import rewriting)
**Severity:** ~~High~~

~~`gala build` creates an isolated workspace with its own `go.mod` and downloads dependencies from the remote registry.~~ **Fixed:** After transpiling and copying local Go subpackages to the workspace `gen/` directory, the builder now rewrites import paths in all `.go` files: `<project-module>/subpkg` → `gala-build-workspace/gen/subpkg`. This makes Go resolve local subpackages from the workspace instead of the remote registry.

PR #107 also fixed the Bazel symlink crash (USABILITY-008) that prevented `copyNonGalaFiles` from copying subpackages in the first place.

### USABILITY-007: `gala transpile` requires manual stdlib path

**Status:** FIXED — [PR #105](https://github.com/martianoff/gala/pull/105)
**Severity:** ~~Medium~~

~~When using `gala transpile` (per-file mode), the stdlib path must be provided manually via `-s`.~~ **Fixed:** `gala transpile` now auto-resolves stdlib and deps from `gala.mod` without requiring the `-s` flag.

---

## Resolved Issues (prior sessions)

### ISSUE-018–027: Various — RESOLVED

See git history for details.

---

## General Notes

1. **Cross-package `ForEach` lambda inference** (BUG-014, FIXED): Relies on PR #46 fix.
2. **`.Copy()` same-package usage** (BUG-017, FIXED): Server uses explicit constructors as a defensive measure.
3. **Block lambda return types** (BUG-006, FIXED): Works correctly after the fix.

---

## Bug Severity Matrix

| Bug | Type | Status | Severity | Workaround | Blocks |
|-----|------|--------|----------|------------|--------|
| BUG-028 | Parser | **FIXED** PR #81 | ~~High~~ | ~~Single-line params~~ | ~~Readability~~ |
| BUG-029 | Codegen | **FIXED** PR #82 | ~~High~~ | ~~`sed` post-process~~ | ~~Go compilation~~ |
| BUG-030 | Type inference | **FIXED** PR #83 | ~~High~~ | ~~if/else rewrite~~ | ~~Pattern matching~~ |
| BUG-031 | Parser | **FIXED** PR #84 | ~~Medium~~ | ~~var + mutation~~ | ~~Expressiveness~~ |
| BUG-032 | DOC_GAP | **DOC_GAP** | Low | Use Tuple/TupleN | Documentation |
| BUG-033 | Parser | **RESOLVED** | ~~Medium~~ | `go_interop.ToBytes()` | ~~Type conversions~~ |
| BUG-034 | Parser | **FIXED** PR #85 | ~~Low~~ | ~~Extract to vars~~ | ~~String interpolation~~ |
| BUG-035 | Design | WONTFIX | Low | Go bridge file | Resource cleanup |
| BUG-036 | Tooling | **FIXED** PR #86 | ~~High~~ | ~~None clean~~ | ~~Standalone transpile~~ |
| BUG-037 | Tooling | **FIXED** PR #87 | ~~High~~ | ~~Bazel only~~ | ~~Library build/test~~ |
| BUG-038 | Tooling | **FIXED** PR #88 | ~~High~~ | ~~Manual creation~~ | ~~Test framework~~ |
| BUG-039 | Codegen | **FIXED** PR #89 | ~~Critical~~ | ~~Manual gen fix~~ | ~~Named param defaults~~ |
| BUG-040 | Codegen | **FIXED** PR #91 | ~~High~~ | ~~Manual gen fix~~ | ~~Dot import refs~~ |
| BUG-041 | Codegen | **FIXED** PR #92 | ~~Medium~~ | ~~Manual gen fix~~ | ~~nil → use Option[T]~~ |
| BUG-042 | Tooling | **FIXED** PR #87 | ~~Critical~~ | ~~None~~ | ~~Go compile validation~~ |
| BUG-043 | Type inference | **VERIFIED** PR #86 | ~~High~~ | ~~Fail-hard now~~ | ~~Test type accuracy~~ |
| BUG-044 | Codegen | **FIXED** PR #90 | ~~High~~ | ~~Manual gen fix~~ | ~~IIFE return~~ |
| BUG-045 | Codegen | **FIXED** PR #93 | ~~Low~~ | ~~Remove import line~~ | ~~Unused import~~ |
| BUG-046 | Build system | **FIXED** PR #94 | ~~High~~ | ~~`go test` workaround~~ | ~~Bazel test pipeline~~ |
| BUG-047 | Type inference | OPEN | High | Explicit `[Response]` | Lambda return types |

### Usability Issues Status

| Issue | Type | Status | Severity | Blocks |
|-------|------|--------|----------|--------|
| USABILITY-001 | Build/Test | **FIXED** PR #97, #108 | ~~High~~ | ~~gala test~~ |
| USABILITY-002 | Parser | **RESOLVED** | ~~Low~~ | ~~Readability~~ |
| USABILITY-003 | Type inference | **RESOLVED** | ~~Low~~ | ~~Match expressions~~ |
| USABILITY-004 | Tooling | **RESOLVED** | ~~Low~~ | ~~Library compile check~~ |
| USABILITY-005 | Design | WONTFIX | Low | `gala test` handles it |
| USABILITY-006 | Build | **FIXED** PR #107, #108 | ~~High~~ | ~~Local subpackages~~ |
| USABILITY-007 | Tooling | **FIXED** PR #105 | ~~Medium~~ | ~~Standalone transpile~~ |
| USABILITY-008 | Build | **FIXED** PR #107 | ~~High~~ | ~~Bazel symlink crash~~ |
| USABILITY-009 | Build/Test | OPEN | Low | None needed (cosmetic) | Slow test startup |

### Critical path to compilable Go output (updated 2026-03-24, fix cycle 2)

**ALL codegen post-processing fixes are now resolved:**

~~1. **BUG-029** — Parenthesize `{}.Apply()` in `if` conditions~~ **FIXED** PR #82
~~2. **BUG-039** — Expand named parameter defaults to positional args~~ **FIXED** PR #89
~~3. **BUG-040** — Add missing `collection_immutable` import / fix dot import handling~~ **FIXED** PR #91
~~4. **BUG-041** — Reject nil for val fields (use Option[T])~~ **FIXED** PR #92
~~5. **BUG-044** — Add `return` before IIFE match expressions~~ **FIXED** PR #90
~~6. **BUG-045** — Remove unused imports~~ **FIXED** PR #93
~~7. **BUG-043** — Lambda type fallback to `any`~~ **VERIFIED** already fixed by PR #86

**No post-processing of `.gen.go` files should be needed for these issues.**

---

## Summary of Workarounds (updated)

Workarounds that can now be **removed** (bugs fixed):

| File | Workaround Applied | Bug | Action |
|------|-------------------|-----|--------|
| `filter.gala` | `SecureWithConfig` params → single line | BUG-028 | **Remove** — use multi-line with trailing commas |
| `filter.gala` | All match expressions → if/else | BUG-030 | **Remove** — match expressions should work now |
| `filter.gala` | `if` expressions → `var` + mutation | BUG-031 | **Remove** — use if-expressions with block branches |
| `filter.gala` | Extract `${req.RawQuery()}` to variables | BUG-034 | **Remove** — escaped quotes in interpolation work now |
| `response.gala` | `struct Cookie(...)` → single line | BUG-028 | **Remove** — use multi-line with trailing commas |
| `server.gala` | `copy()` params → single line | BUG-028 | **Remove** — use multi-line with trailing commas |
| `~/.gala/stdlib/v0.20.0/test/` | Manually created test module | BUG-038 | **Remove** — distributed automatically now |
| `*.gen.go` | `sed` for `{}.Apply()` parenthesization | BUG-029 | **Remove** — transpiler handles this now |

Workarounds that can now be **removed** (fix cycle 5):

| File | Workaround Applied | Bug | Action |
|------|-------------------|-----|--------|
| `*.gen.go` (library) | `sed` fixes for BUG-039/040/041/044/045 | Post-process | **Remove** — all codegen bugs fixed in PRs #89–#93 |
| `gala transpile` script | Manual `-s` stdlib path | USABILITY-007 | **Remove** — auto-resolved from gala.mod (PR #105) |
| `gala transpile` script | Per-file transpile + `go test` workaround | USABILITY-006 | **Remove** — `gala build`/`gala test` now resolve local subpackages (PRs #107–#108) |

Workarounds that must **remain** (bugs still open):

| File | Workaround | Bug |
|------|-----------|-----|
| `request.gala` | Remove `FormFile` (multi-return) | BUG-032 |
| `server_test.gala` | `[]byte(...)` → `httpcore.ToBytes(...)` | BUG-033 (use `go_interop.ToBytes`) |
| `httpcore/testing.go` | Go bridge functions | BUG-032/035 |
| `*.gen.go` (tests) | `sed` fixes for BUG-043 type fallbacks | Post-process |
| `gala_test_harness_test.go` | Go test → GALA test adapter | USABILITY-005 |
| `gala_test_wrappers_test.go` | 220 `Test_X` wrappers | USABILITY-005 |

---

## Next Fix Cycle Priorities

**All codegen and usability issues are resolved.** Only test-specific type inference issues remain:

1. **BUG-043 partial** (High) — Test codegen: `Immutable[string]` wrapping, `any` type fallback, `Immutable[Array[Cookie]]` — 34 errors in `filter_test.gen.go`, 1 in `integration_test.gen.go`. **This is the only blocker for running tests.**
2. **BUG-047** (Medium) — Lambda return type inference for complex multi-return bodies — has workaround (explicit `[Response]` type params)
3. **USABILITY-009** (Low) — `gala test` downloads stale remote module before using rewritten local imports
4. **USABILITY-005** — WONTFIX — `gala test` generates harness automatically

**Remaining open issues** after fix cycle 5 (PRs #107–#108):

> Fix cycle 4 resolved: BUG-047 FoldLeft spread type (PR #102), BUG-048 slice type in generics (PR #99), BUG-045 unused dot imports (PR #106), cross-file dot imports (PR #98), test sibling analysis (PR #103), auto-stdlib (PR #105), local subpackages partial (PR #104).
> Fix cycle 5 resolved: USABILITY-008 Bazel symlink crash (PR #107), USABILITY-006 local subpackage import rewriting (PR #108).

**Library code compiles clean — 0 errors.** Only test codegen and tooling issues remain:

1. **BUG-043 partial** — `filter_test.gen.go` (34 errors): `Immutable[string]` wrapping on struct fields (`resp.Body`) accessed in test assertions, `any` type fallback for lambda params in local val declarations, `Immutable[Array[Cookie]]` wrapping prevents calling `.Exists()/.Find()` directly
2. **USABILITY-009 (NEW)** — `gala test` still downloads remote httpcore even though `gala build` rewrites imports correctly. The test path re-transpiles test files but the workspace `gen/` directory still has a `require github.com/martianoff/gala-server` in go.mod, causing `go get` to fetch the remote module. The import rewriting for test files occurs after `go.mod` generation, so Go downloads the remote dependency before discovering it's been rewritten.

```
# gala test -v output:
Rewriting project module imports: github.com/martianoff/gala-server → gala-build-workspace/gen
...
Downloading Go dependencies...
go: finding module for package github.com/martianoff/gala-server/httpcore  ← still fetches remote!
go: found github.com/martianoff/gala-server/httpcore in github.com/martianoff/gala-server v0.0.0-20260320051124-045839da4797
```

**Impact:** Slows down `gala test` by downloading unnecessary remote packages. The compiled code uses the local rewritten imports, but Go's module resolver still resolves the stale `require` entry. The remote httpcore is fetched but never used — the local copy in `gen/httpcore/` is what actually compiles.

**Expected:** After import rewriting, the stale `require github.com/martianoff/gala-server` entry should be removed from `go.mod` (or never added), or `go mod tidy` should be run after rewriting to clean up unused requires.

**Verified FIXED:**
- BUG-047 (FoldLeft spread type) — proxyFilter compiles clean
- BUG-048 (`Option[Array[byte]]`) — response.gen.go compiles clean
- BUG-045 (unused imports) — types.gen.go compiles clean
- BUG-040 (cross-file dot imports) — server.gen.go compiles clean
- USABILITY-007 (`gala transpile` auto-stdlib) — works without `-s` flag
- USABILITY-008 (Bazel symlink crash) — `copyNonGalaFiles` skips unreadable entries (PR #107)
- USABILITY-006 (local subpackage resolution) — import rewriting to workspace module (PR #108)
- USABILITY-001 (test package conflict + local subpackage) — testgen/ separation (PR #97) + import rewriting (PR #108)
