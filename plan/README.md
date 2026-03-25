# Implementation plans by phase

This directory holds **executable mini-specs** for umbrella tickets from [TICKETS.md](../TICKETS.md). Phases follow an **AI-friendly order** (tooling and docs first, then config, tests, CI, auth, polish, P3). [PLAN.md](../PLAN.md) describes product goals; these files describe **how** to implement each ticket.

**How to use:** Work through phases **0 → 6** unless a plan’s **Dependencies** say otherwise. Split a plan into sub-tasks when you start the ticket.

| Phase | Theme | Tickets | Folder |
|-------|--------|---------|--------|
| 0 | AI context and local ergonomics | T18, T8, T28, T9 | [phase-0](phase-0/) |
| 1 | Runtime configuration and safe shutdown | T26, T27 | [phase-1](phase-1/) |
| 2 | Tests and coverage | T10, T11, T12 | [phase-2](phase-2/) |
| 3 | GitHub, Docker, CI | T14, T6, T19, T13 | [phase-3](phase-3/) |
| 4 | API hardening | T7 | [phase-4](phase-4/) |
| 5 | P2 polish and multi-DB | T15, T17, T16, T1 | [phase-5](phase-5/) |
| 6 | P3 scale, prod, product options | T23, T20, T22, T21, T2, T3, T4, T5 | [phase-6](phase-6/) |

**Within phase 3**, prefer ticket order **T14 → T6 → T19 → T13** (Docker/compose before CI integration tests that need it).

[TICKETS.md](../TICKETS.md) P1/P2/P3 priority bands still apply for scheduling; this folder orders work for **solo + AI** flow.
