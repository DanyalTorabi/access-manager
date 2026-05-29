# T68 — Align go.mod `go` directive with `toolchain` to prevent CI runtime download

## Ticket

**T68** — Align go.mod `go` directive with `toolchain` (GitHub [#119](https://github.com/DanyalTorabi/access-manager/issues/119))

## Phase

**Phase 8** — maintenance and polish

## Problem

`go/go.mod` declares two different Go versions:

```
go 1.25.0
toolchain go1.25.9
```

`actions/setup-go@v5` with `go-version-file: go/go.mod` reads the `go` directive and installs **go1.25.0**. The CI script (`Set GOTOOLCHAIN from go.mod` step) then sets `GOTOOLCHAIN=go1.25.9` (parsed from the `toolchain` line). When go1.25.0 runs with `GOTOOLCHAIN=go1.25.9`, it detects the mismatch and downloads go1.25.9 at runtime — an unnecessary network call that can fail on a cold cache or a restricted network.

This also caused local lint runs to print:

```
go: github.com/golangci/golangci-lint/v2@v2.11.4 requires go >= 1.25.0; switching to go1.25.10
```

The drift to 1.25.10 was a secondary effect of golangci-lint's own `go.mod` pulling a newer patch, compounding the version confusion.

**Discovered during:** PR #118 (T22) review — pre-existing, not introduced by T22.

## Fix

Raise the `go` directive in `go/go.mod` to match the `toolchain` directive:

```
go 1.25.9
toolchain go1.25.9
```

Run `go mod tidy` after the change. The `toolchain` line becomes redundant once `go` matches it, but keeping it explicit is harmless and makes intent clear.

## Steps

1. Edit `go/go.mod`: change `go 1.25.0` → `go 1.25.9`.
2. Run `go mod tidy` from `go/` to update `go.sum` if needed.
3. Verify `make test` and `make lint` pass.
4. Check if the `Set GOTOOLCHAIN from go.mod` step in `.github/workflows/ci.yml` is still needed — it may be simplified or removed once `go` and `toolchain` are aligned.

## Files / paths

- **Modify:** `go/go.mod`
- **Possibly simplify:** `.github/workflows/ci.yml` (GOTOOLCHAIN step)

## Acceptance criteria

- `go/go.mod` `go` directive matches `toolchain` directive.
- `make test` and `make lint` pass locally.
- CI no longer logs a toolchain download or version-switch message.

## Dependencies

None.

## Curriculum link

**Theme 3** — CI/CD and build hygiene.
