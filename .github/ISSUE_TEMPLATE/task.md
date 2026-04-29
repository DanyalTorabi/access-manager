---
name: Task / Plan ticket
about: Track work that has (or needs) a plan file under `plan/`
title: "[T##] short imperative title"
labels: []
---

<!--
TITLE FORMAT (required): "[T##] short imperative title"
  • Use the next free Tnn (see existing files under plan/phase-*/).
  • Examples of good titles:
      [T50] Fix double increment of authz_checks_total
      [T48] Typed InvalidInputError and stable public messages

PRIORITY LABEL (required): set exactly one of P1 / P2 / P3 on this issue
before submitting. The repo's labels are:
  • P1  — During active development
  • P2  — Next (after core flows stabilize)
  • P3  — Later (scale, prod, hardening)

NOTE: Do NOT reuse a closed Tnn for a new ticket. T31 (#42) in particular
is closed; do not write `TODO(T31)` for unrelated follow-ups.
-->

## Problem / motivation

<!-- What is wrong, missing, or risky today, and why it matters. -->

## Proposed work

<!-- Bullet list of the deliverables this ticket covers. -->

-

## Plan

See [plan/phase-N/T##-short-name.md](../../plan/phase-N/T##-short-name.md).

## Acceptance criteria

<!-- Bullet list of testable outcomes that mean this ticket is done. -->

-

## Discovered in / related

<!-- Optional: PR or issue that surfaced this work, related Tnn dependencies. -->
