# T69 — Automate Go toolchain version tracking

## Ticket

**T69** — Automate Go toolchain version tracking (GitHub [#121](https://github.com/DanyalTorabi/access-manager/issues/121))

## Phase

**Phase 8** — maintenance and polish

## Problem

`go/go.mod` was bumped manually twice during T68 (PR #120): `1.25.0` → `1.25.9` → `1.25.10`. The final version was discovered only after `govulncheck` failed CI and the golangci-lint output flagged a version mismatch. There is no mechanism to proactively detect new Go patch releases.

**Discovered during:** PR #120 (T68) review.

## Fix

Add a `.github/dependabot.yml` `gomod` entry. Dependabot supports `go-modules` as an ecosystem and will open a PR automatically when the `go` directive in `go.mod` can be bumped to a newer patch or minor version.

Example `.github/dependabot.yml` addition:

```yaml
version: 2
updates:
  - package-ecosystem: gomod
    directory: /go
    schedule:
      interval: weekly
    labels:
      - dependencies
      - go
```

This is the simplest approach with no custom code. If finer control is needed (patch-only bumps, custom PR titles), a scheduled GHA workflow querying `go.dev/dl/?mode=json` is an alternative.

## Steps

1. Add or update `.github/dependabot.yml` with the `gomod` entry pointing at `/go`.
2. Verify Dependabot recognises the config via the GitHub Security → Dependabot tab.
3. Optionally configure `ignore` rules to skip major/minor bumps and allow only patch updates.

## Files / paths

- **Create or modify:** `.github/dependabot.yml`

## Acceptance criteria

- New Go patch releases trigger a Dependabot PR automatically without requiring a CI failure to discover the gap.
- `go/go.mod` `go` directive is kept current with the latest patch.

## Dependencies

None.

## Curriculum link

**Theme 3** — CI/CD and build hygiene.
