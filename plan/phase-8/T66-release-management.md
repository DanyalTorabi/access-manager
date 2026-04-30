# T66 — Release management: SemVer tags, GitHub Actions release & undo workflows

## Ticket

**T66** — Release management umbrella (GitHub [#102](https://github.com/DanyalTorabi/access-manager/issues/102))

## Phase

**Phase 8** — Release engineering

## Goal

Establish a repeatable release process for `access-manager`:

1. Tag a release with a SemVer git tag (`vX.Y.Z`).
2. CI builds **binaries** for linux/macOS/windows, **checksums** them, and attaches them to a GitHub Release.
3. CI publishes a **versioned Docker image** to GHCR (`ghcr.io/danyaltorabi/access-manager:vX.Y.Z` and `:latest`).
4. CHANGELOG `## [vX.Y.Z] - YYYY-MM-DD` block is generated/promoted from the `## [Unreleased]` section.
5. An **undo-release** workflow can roll back a botched release: delete the GitHub Release + tag, untag the GHCR image, and reopen the `Unreleased` block.

Automation is built around **`goreleaser`** (binaries + checksums + GitHub Release) plus a small custom workflow for the undo path. `release-please` is **not** adopted (CHANGELOG is hand-curated today; revisit if Conventional Commits are introduced later).

## Deliverables

### Configuration

- `.goreleaser.yaml` at repo root configured to:
  - Build `cmd/server` for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
  - Produce `tar.gz` (linux/macOS) and `zip` (windows) archives.
  - Generate `checksums.txt` (SHA-256).
  - Use the `LICENSE` (T64) and `README.md` in archives.
  - Strip CGO (project is pure Go via `modernc.org/sqlite`).

### Workflows

- `.github/workflows/release.yml` — triggered on **push of a tag matching `v*.*.*`**:
  - Lint + test (reuse the existing CI job via `workflow_call` if practical).
  - Run `goreleaser release --clean`.
  - Build and push the Docker image to GHCR with `:vX.Y.Z` and `:latest` tags using `docker/build-push-action`.
  - Verify CHANGELOG has a matching `## [vX.Y.Z]` block; fail the workflow if missing.

- `.github/workflows/release-undo.yml` — manual `workflow_dispatch` with a required `version` input (e.g. `v0.3.1`):
  - Step 1: Refuse to run unless caller has `maintain` permission (use the `permissions` check or branch protection).
  - Step 2: Delete the GitHub Release with `gh release delete <version> --yes`.
  - Step 3: Delete the git tag both remotely (`git push --delete origin <version>`) and the local cache.
  - Step 4: Untag the GHCR image (`gh api -X DELETE /user/packages/container/access-manager/versions/<id>`) — find the version by tag name first.
  - Step 5: Print a summary listing what was undone, and a reminder to manually move the CHANGELOG block back to `## [Unreleased]` (this step is not automated to avoid clobbering hand-edits).

### CHANGELOG conventions

- Keep a current `## [Unreleased]` block at the top with `Added`/`Changed`/`Fixed`/`Removed` subsections.
- At release time, the human (or a release helper script) renames `## [Unreleased]` to `## [vX.Y.Z] - YYYY-MM-DD` and starts a new empty `## [Unreleased]`.
- Add a one-line link reference at the bottom: `[vX.Y.Z]: https://github.com/DanyalTorabi/access-manager/compare/vX.Y.(Z-1)...vX.Y.Z`.

### Docs

- `docs/releasing.md` — checklist for cutting a release:
  1. Confirm `main` is green.
  2. Promote `## [Unreleased]` → `## [vX.Y.Z] - <date>`; add the compare link.
  3. Open PR titled `release: vX.Y.Z`. Merge after CI green.
  4. Tag the merge commit: `git tag -s vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
  5. Watch `.github/workflows/release.yml` complete; verify GitHub Release artifacts and `ghcr.io/.../access-manager:vX.Y.Z`.
  6. Smoke-test the published image: `docker run --rm ghcr.io/danyaltorabi/access-manager:vX.Y.Z --help`.
- `docs/releasing.md` — separate "Undoing a release" section for the rollback workflow.

## Steps

1. Add `LICENSE` first (depends on **T64 / #100**) so goreleaser archives include it.
2. Land `.goreleaser.yaml` and validate locally with `goreleaser release --snapshot --clean --skip=publish`.
3. Add `release.yml` workflow; test by pushing an annotated tag like `v0.0.1-rc1` against a fork or a `--draft` release.
4. Add `release-undo.yml` workflow; test by undoing the rc1 release.
5. Hand-write `docs/releasing.md` based on the actual workflow runs.
6. Cut the first real `v0.1.0` release once docs are accurate.

## Acceptance criteria

- Pushing `v0.0.1-rc1` produces a draft GitHub Release with attached binaries (5 archives + `checksums.txt`) and a matching `ghcr.io/danyaltorabi/access-manager:v0.0.1-rc1` image.
- Running the undo workflow with `version=v0.0.1-rc1` removes the GitHub Release, deletes the tag, and untags the GHCR image.
- `make test` and `make lint` still pass on `main`.
- `docs/releasing.md` is up to date and includes a worked example.

## Files / paths

- **Create:** `.goreleaser.yaml`, `.github/workflows/release.yml`, `.github/workflows/release-undo.yml`, `docs/releasing.md`
- **Edit:** `CHANGELOG.md` (link references at bottom; first `[v0.1.0]` block when shipping)
- **Edit:** `README.md` (badge / install snippet from a GitHub Release)

## Dependencies

- **T64 / #100** — `LICENSE` must exist before goreleaser bundles it into archives.
- **T13 / #24** — existing CI workflow structure used as a base.
- **T19 / #30** — existing Dockerfile reused for the GHCR publish step.
- **T15 / #26** — CHANGELOG conventions already in place.

## Out of scope

- Conventional Commits / `release-please` automation (revisit in a future ticket if commit discipline tightens).
- Signed releases (sigstore / cosign image signing) — defer to a follow-up security ticket.
- Homebrew tap / Linux package repos — defer until there is user demand.
