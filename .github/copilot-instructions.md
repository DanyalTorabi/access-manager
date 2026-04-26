# Code Review Standards — access-manager

## Architecture

- **Module root is `go/`**. Run `go` commands from there.
- `internal/access` and `internal/store` are library-oriented: **no** chi/HTTP imports.
- HTTP-only code belongs in `internal/api`; wiring in `cmd/server`.
- Future `go/pkg/...` for importable APIs; avoid coupling core logic to the router.

## Security (Critical)

- Never commit secrets, API keys, tokens, or credentials.
- Default HTTP bind to `127.0.0.1`. Flag any `0.0.0.0` without auth.
- Use parameterized queries; no string concatenation into SQL.
- Bearer tokens must not appear in process argv; use stdin or env.

## Error Handling

- Distinguish client errors (400) from internal errors (500) in handlers.
- Use `errors.Is(err, store.ErrNotFound)` for not-found → 404.
- Wrap errors with `fmt.Errorf("context: %w", err)`; avoid double-prefixing.

## Testing

- Non-trivial behavioral changes require automated tests in the same PR.
- Always assert HTTP status code **before** decoding JSON response body.
- Use `t.TempDir()` for temp paths; never hard-code `/tmp/` or `/dev/null`.
- Use `t.Setenv()` for env vars in tests; never mutate global state.
- Prefer polling with a deadline over `time.Sleep` in tests that wait for servers.
- Goroutines in production code must be synchronized; never silently drop errors.

## Go Style

- Match existing code style; avoid unrelated refactors.
- No comments that just narrate what code does ("increment counter").
- Deferred work must reference a ticket: `// TODO(T14): ...`.
- Keep functions focused; extract testable helpers from `main()`.

## Code Review Scope

When reviewing a PR, always review **all changed files** — not a partial subset. Cross-cutting concerns (shared helpers, type changes, API contracts) can only be caught by reading the full diff. Specifically:

- If a type or helper is added/changed in one file, verify every call site in the diff uses it correctly.
- If a query parameter or API contract is added, check that OpenAPI, Postman, tests, and handler code are all consistent.
- If a constant or enum is introduced, confirm exhaustive handling (switch cases, validation, docs).
- Do not skip files because they look "low risk" (e.g. docs, CHANGELOG, Postman JSON) — contract mismatches hide there.

## Pull Request Expectations

- `make test` and `make lint` must pass.
- No secrets or real `.env` values in the diff.
- Docs updated if behavior or setup changed.

## Repository workflow & Cursor-derived rules

- **No unused code:** Do not add functions, methods, or interfaces that have no callers in the current codebase. If a reviewer (human or AI) suggests speculative code (e.g. "consider implementing X for future compatibility"), defer it to a tracked ticket and reference that ticket in the review thread and plan files (e.g. `// TODO(T36): ...`).
- **Branching:** When starting work on a new ticket or task, create a topic branch from up-to-date `main` before making file changes. Follow the repo naming pattern (e.g. `author/prefix/description`, all lowercase). Verify with `git branch --show-current`; if on the wrong branch, stash, switch, create the branch, and pop.
- **Git (AI default):** Do not run `git commit`, `git push`, or `gh pr create` on behalf of a user unless they explicitly ask you to. When proposing changes, provide a proposed commit message (subject + optional body) and a PR description that follows `.github/pull_request_template.md` (Summary, Ticket, Checklist) so the human can run the git/gh commands locally.
- **Use `gh` for automation:** If the user asks you to automate GitHub operations, prefer the `gh` CLI over raw HTTP calls; when automation is not permitted, supply copy-paste snippets for maintainers.
- **After substantive changes:** Update `go/README.md` (and root `README.md` if layout/entrypoints changed). If the change is user-visible and `CHANGELOG.md` exists, add an Unreleased entry. Run `make test` from the repository root (or `go test -race ./...` from `go/`) and `make lint` / `make cover` when configured.
- **PR / review deferrals:** If a valid review comment (human or AI) will not be fixed in the current PR, reply on the thread that it will be addressed in `#… / Txx` and add a tracked note under the umbrella plan file (`plan/...`) so the follow-up is not lost.

## Code Review Mode (Cursor → Copilot guidance)

When asked to review a PR or diff, follow these steps:

- Review every changed file line-by-line; do not skim or skip files.
- Flag every issue you find; provide the concrete fix rather than marking it "acceptable".
- For each issue, state: the file and line reference, what is wrong, and a concrete fix to apply.
- Use this file (`.github/copilot-instructions.md`) and `.github/instructions/` as a checklist while reviewing.
- Verify tests assert the *intended* behavior (not only the current behavior). Check for TOCTOU races, correct error classification (400 vs 404 vs 500), test determinism (avoid `time.Sleep`), goroutine synchronization, and resource leaks.
- After the review, list any open Copilot/Cursor review comments from the PR and state whether each is addressed or still open.

## PR ↔ Issue linking

- In the PR description (or a commit on the branch), use `Fixes #123`, `Closes #123`, or `Resolves #123` to ensure the PR appears on the issue timeline and merging the PR auto-closes the issue when merged into the default branch. Use `Refs #123` or `See #123` for non-closing links. Place the reference in the Ticket section of the PR body when applicable.

