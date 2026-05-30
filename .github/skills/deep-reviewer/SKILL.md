---
name: deep-reviewer
description: "Deep PR code reviewer for access-manager. Use when: reviewing a pull request, doing a PR review, audit PR changes, find issues in PR, deep review, code review PR. Asks for a PR number, fetches the branch, reads plan files and CHANGELOG to understand scope, then produces a numbered issue list written to comments-PR#[PRN].md."
argument-hint: "PR number to review (e.g. 122)"
---

# Deep PR Reviewer

## Purpose

Produce a thorough, issue-focused review of a pull request in the `DanyalTorabi/access-manager` repository. Output is a numbered list of **concerns only** — logic errors, structural problems, security issues, missing tests, contract mismatches — written to `comments-PR#[PRN].md` in the repo root.

## When to Use

- User asks to review PR `#N`, audit changes, or do a deep review.
- Triggered by phrases: "review PR", "deep review", "audit PR", "find issues in PR".

---

## Procedure

### Step 0 — Collect PR Number

If the user has not provided a PR number, ask:

> "Which PR number should I review?"

Store it as **PRN**. The PR URL will be:
`https://github.com/DanyalTorabi/access-manager/pull/[PRN]`

---

### Step 1 — Fetch PR Metadata and Diff

1. Run `gh pr view [PRN] --json number,title,body,headRefName,baseRefName,files` to get the PR title, description, branch names, and changed-file list.
2. Run `gh pr diff [PRN]` to get the full unified diff of every changed file.
3. Note the **head branch** (e.g. `danyal/feature/t21-kubernetes`) — this is the branch under review.

> The local checkout may not be on the PR branch. Fetch via `git fetch origin [HEAD_BRANCH]` and `git show origin/[HEAD_BRANCH]:[FILE_PATH]` to read files at the PR's HEAD without switching branches.

---

### Step 2 — Understand Scope and Expectations

Before reviewing code, read context files to understand what the PR is **supposed** to do and what is **out of scope**:

1. **PR body** — Summary, linked ticket, checklist (from `gh pr view` output).
2. **Plan file** — Find the matching plan file under `plan/` by ticket number from the PR body (e.g. `plan/phase-6/T21-kubernetes.md`). Read it fully.
3. **CHANGELOG.md** — Check the Unreleased section for the PR's declared deliverables.
4. **Referenced issues** — If the PR body contains `Fixes #N` or `Closes #N`, read the GitHub issue with `gh issue view N`.
5. **Existing review comments** — Run `gh pr review [PRN] --comments` or check `comments-PR#[PRN].md` if it already exists, to avoid duplicating known issues.

Use this context to calibrate:
- What the PR intends to deliver.
- What is explicitly deferred (TODOs referencing future tickets).
- What contracts (OpenAPI, env vars, config keys) the PR touches.

---

### Step 3 — Deep Review

Review **all changed files** line by line. Do NOT skip files because they appear low-risk (docs, CHANGELOG, Postman JSON often hide contract mismatches).

For each changed file, check:

#### Logic & Correctness
- Off-by-one errors, incorrect conditional logic, unhandled edge cases.
- Race conditions or non-deterministic behavior.
- Incorrect error classification (400 vs 404 vs 500).

#### Security (OWASP Top 10 + repo policy)
- Secrets or credentials hard-coded or committed.
- Parameterized queries; no string-concatenated SQL.
- HTTP bind address defaulting to `0.0.0.0` without auth.
- Bearer tokens in process argv.
- Missing `securityContext`, excessive container privileges.

#### Architecture & Structure
- HTTP/chi imports leaking into `internal/access` or `internal/store`.
- Business logic in `internal/api` handlers instead of domain layer.
- Unused functions, methods, or interfaces added speculatively.
- Deferred work without a ticket reference (`// TODO(Txx): ...`).

#### Testing
- Non-trivial behavioral changes lack automated tests.
- Tests assert current behavior rather than intended behavior.
- Hard-coded `/tmp/` paths, `time.Sleep` polling, global state mutation.
- HTTP status code asserted before JSON decoding.

#### Cross-cutting Consistency
- If a type or helper is added in one file, verify all call sites in the diff use it correctly.
- If a query parameter or API contract is added, check OpenAPI, Postman, tests, and handler are consistent.
- If a constant or enum is introduced, verify exhaustive handling (switch cases, validation, docs).

#### Kubernetes / Infrastructure (if applicable)
- `terminationGracePeriodSeconds` vs app shutdown timeout.
- Helm template values vs hardcoded strings.
- Resource names including `.Release.Name` to prevent multi-instance collisions.
- `namespace:` field using `{{ .Release.Namespace }}` not a string literal.
- Headless governing service for StatefulSets.
- Probe `initialDelaySeconds` accounting for migration startup time.
- Default values (`image.tag: latest`, `postgres.enabled: true`) that are footguns for production.

#### Documentation
- README, docs, and CHANGELOG updated if behavior or setup changed.
- No placeholder text left where factual content existed before.
- Security warnings present before copy-pasteable credential examples.

---

### Step 4 — Write Issues to File

1. Determine the output file path: `[REPO_ROOT]/comments-PR#[PRN].md`.
2. If the file does not exist, create it with this header:

```markdown
# PR #[PRN] Review — [PR TITLE]

Branch: `[HEAD_BRANCH]` → `[BASE_BRANCH]`
Reviewed: [TODAY'S DATE]
Scope: all [N] changed files ([comma-separated file list or areas])

---

## Issues

---
```

3. For each issue found, append a section using this format:

```markdown
### LCM-[PRN]-[N] — [Short imperative title of the problem]

**Files:** `path/to/file.go` (line N or lines N–M if helpful)

[One paragraph explaining what the problem is and why it matters. Include code snippets where they help clarify the issue.]

**Fix:** [Concrete, actionable fix. Include corrected code snippet where applicable.]

---
```

4. **Do not stop at 10 issues.** Continue until all concerns found in Step 3 are documented.
5. **Issues only** — do not list praise, positive observations, or notes about correct code. If the PR is clean, state that explicitly at the end.

---

### Step 5 — Summary

After writing all issues to the file, reply to the user with:

- Total issue count.
- A one-line summary per issue (issue ID + title).
- The path to the output file.
- Any issues that are borderline or context-dependent (flag, don't suppress).

---

## Output Format Rules

- Issue IDs are sequential: `LCM-[PRN]-1`, `LCM-[PRN]-2`, …, `LCM-[PRN]-N`.
- Each issue is self-contained: a reader unfamiliar with the PR can understand it from the issue text alone.
- Code snippets in issues use fenced blocks with the language tag.
- "Fix" is always concrete — avoid "consider adding X". Say what to add and where.
- Do not reference praise ("the rest of the file is fine"). Issue list = problems only.

---

## Reference: Repo Conventions

See also:
- [`.github/copilot-instructions.md`](../../copilot-instructions.md) — full review checklist
- [`AGENTS.md`](../../../AGENTS.md) — architecture, library vs service boundaries, unused code policy
- [`CONTRIBUTING.md`](../../../CONTRIBUTING.md) — self-review checklist, branching, PR template
