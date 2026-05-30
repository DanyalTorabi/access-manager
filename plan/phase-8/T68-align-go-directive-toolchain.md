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

This message is the authoritative signal for the correct minimum toolchain: golangci-lint's own dependency graph requires `go1.25.10`. The correct fix target was therefore `go 1.25.10`, not `go 1.25.9`.

**Discovered during:** PR #118 (T22) review — pre-existing, not introduced by T22.

## Fix

Raise the `go` directive in `go/go.mod` to `1.25.10` (the minimum required by the dependency graph):

```
go 1.25.10
```

Run `go mod tidy` after the change. The `toolchain` line is removed automatically by `go mod tidy` because it becomes redundant once the `go` directive equals or exceeds the previously declared toolchain. As a secondary benefit, `go1.25.10` also resolves stdlib vulnerability GO-2026-4971 (Windows-only panic in `net.Dial`/`LookupPort` on NUL byte).

## Steps

1. Edit `go/go.mod`: change `go 1.25.0` → `go 1.25.9`.
2. Run `go mod tidy` from `go/` to update `go.sum` if needed.
3. Verify `make test` and `make lint` pass.
4. Check if the `Set GOTOOLCHAIN from go.mod` step in `.github/workflows/ci.yml` is still needed — it may be simplified or removed once `go` and `toolchain` are aligned.

## Files / paths

- **Modify:** `go/go.mod`
- **Modify:** `.github/workflows/ci.yml` — removed the `Set GOTOOLCHAIN from go.mod` workaround steps from all three jobs (`go`, `docker`, `integration`); `actions/setup-go@v5` installs the correct version directly from the `go` directive now that `go` and `toolchain` are aligned

## Acceptance criteria

- `go/go.mod` has a single `go` directive set to `1.25.10`; no separate `toolchain` directive.
- `make test` and `make lint` pass locally.
- CI no longer logs a toolchain download or version-switch message.

## Dependencies

None.

## Curriculum link

**Theme 3** — CI/CD and build hygiene.
