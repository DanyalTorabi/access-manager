# End-to-end smoke (**T16**)

Two implementations of the **same** journey (health → domain → entities → group grant → `authz/check`):

| Path | How to run |
|------|------------|
| **Go (default)** | From repo root: `make e2e` (needs server on **`BASE_URL`**, default `http://127.0.0.1:8080`). Runs `go test -race -count=1 -tags=e2e ./e2e/...` inside **`go/`** (see **`go/Makefile`** `e2e`). |
| **Bash (optional)** | `bash test/e2e/bash/run.sh` or `make e2e-bash` — requires **curl** and **jq**. |

Set **`API_BEARER_TOKEN`** when the server enforces Bearer auth.

Pick one long-term; keep both until you decide.
