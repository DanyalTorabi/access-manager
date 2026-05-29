# T64 — Add LICENSE file (MIT)

## Ticket

**T64** — Add `LICENSE` (MIT) (GitHub [#100](https://github.com/DanyalTorabi/access-manager/issues/100))

## Phase

**Phase 8** — Repository hygiene

## Goal

Add a top-level `LICENSE` file using the **MIT License**, copyrighted to **Danyal Torabi**, and reference it from the README so the repository is unambiguously open-source.

## Why

GitHub treats a repo without a `LICENSE` file as "all rights reserved" — nobody can legally reuse the code. Adding the file:
- Makes the project legally usable and open-source (matching the OpenClaw project's licensing posture).
- Lets GitHub display the license badge in the repo header.
- Lets `go` tooling and SBOM generators detect the license correctly.

## Deliverables

- `LICENSE` at repository root containing the standard MIT license text with the copyright line:
  ```
  Copyright (c) 2026 Danyal Torabi
  ```
- A short `## License` section in [README.md](../../README.md) pointing at the file:
  > Licensed under the [MIT License](LICENSE).
- A short `## License` section in [go/README.md](../go/README.md) pointing at the root file.
- No `NOTICE` file needed (MIT does not require one).

## Steps

1. Create a new `LICENSE` file at the repo root with the standard MIT license text and copyright header: `Copyright (c) 2026 Danyal Torabi`.
2. Add a `## License` section near the bottom of `README.md`: `Licensed under the [MIT License](LICENSE).`
3. Add a `## License` section near the bottom of `go/README.md` pointing at the root file.
4. Add a changelog entry under `## [Unreleased]` → `### Added` in `CHANGELOG.md`.
5. Verify GitHub's license detector picks it up after the PR merges (the repo header will show "MIT" within a few minutes of merge).

## Acceptance criteria

- `LICENSE` exists at repository root with the standard MIT text and `Copyright (c) 2026 Danyal Torabi`.
- `README.md` and `go/README.md` reference the license.
- `CHANGELOG.md` has an Unreleased entry.
- `golangci-lint` and `make test` are unaffected.
- After merge, GitHub UI shows "MIT" as the repo license.

## Files / paths

- **Create:** `LICENSE`
- **Edit:** `README.md`
- **Edit:** `go/README.md`
- **Edit:** `CHANGELOG.md`

## Dependencies

None. Blocker for **T66 / #102** (release archives include `LICENSE`).

## Out of scope

- Per-file SPDX headers (`// SPDX-License-Identifier: MIT`) — can be added later in a separate sweep if desired.
- A `CONTRIBUTOR LICENSE AGREEMENT` (CLA) workflow.
- Third-party license aggregation (`go-licenses report ...`).
