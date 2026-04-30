# T64 — Add LICENSE file (Apache-2.0)

## Ticket

**T64** — Add `LICENSE` (Apache-2.0) (GitHub [#100](https://github.com/DanyalTorabi/access-manager/issues/100))

## Phase

**Phase 8** — Repository hygiene

## Goal

Add a top-level `LICENSE` file using the **Apache License, Version 2.0**, copyrighted to **Danyal Torabi**, and reference it from the README so the repository is unambiguously licensed.

## Why

GitHub treats a repo without a `LICENSE` file as "all rights reserved" — nobody can legally reuse the code. Adding the file:
- Makes the project legally usable.
- Lets GitHub display the license badge in the repo header.
- Lets `go` tooling and SBOM generators detect the license correctly.

## Deliverables

- `LICENSE` at repository root containing the verbatim Apache-2.0 license text and the copyright line:
  ```
  Copyright 2026 Danyal Torabi
  ```
- A short `## License` section in [README.md](../../README.md) pointing at the file:
  > Licensed under the [Apache License, Version 2.0](LICENSE).
- Optional: a `NOTICE` file if/when third-party code under Apache-2.0 with notices is bundled. Defer until needed.

## Steps

1. Copy the Apache-2.0 text from [https://www.apache.org/licenses/LICENSE-2.0.txt](https://www.apache.org/licenses/LICENSE-2.0.txt) into a new `LICENSE` file at the repo root.
2. At the bottom of the boilerplate (after the "END OF TERMS AND CONDITIONS" / "APPENDIX" block), include the recommended copyright line: `Copyright 2026 Danyal Torabi`.
3. Add a `## License` section near the bottom of `README.md`.
4. Verify GitHub's license detector picks it up after the PR merges (the repo header will show "Apache-2.0" within a few minutes of merge).

## Acceptance criteria

- `LICENSE` exists at repository root with the **unmodified** Apache-2.0 text.
- `README.md` references the license.
- `golangci-lint` and `make test` are unaffected.
- After merge, GitHub UI shows "Apache-2.0" as the repo license.

## Files / paths

- **Create:** `LICENSE`
- **Edit:** `README.md`

## Dependencies

None. Blocker for **T66 / #102** (release archives include `LICENSE`).

## Out of scope

- Per-file SPDX headers (`// SPDX-License-Identifier: Apache-2.0`) — can be added later in a separate sweep if desired.
- A `CONTRIBUTOR LICENSE AGREEMENT` (CLA) workflow.
- Third-party license aggregation (`go-licenses report ...`).
