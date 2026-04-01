# T21 — Kubernetes

## Ticket

**T21** — Kubernetes

## Phase

**Phase 6** — P3 scale, prod, product options

## Goal

Deploy the server (and dependencies or externalize them) with **Deployments**, **Services**, **ConfigMaps/Secrets**, **liveness/readiness** probes—config values aligned with **T26**.

## Deliverables

- `deploy/k8s/` manifests or a **Helm chart** with values for image, env, resources.
- Health checks wired to `/health` and optionally DB ping for readiness.

## Steps

1. Start from image produced in **T13**.
2. Externalize DB: use managed SQL or in-cluster Postgres with persistence (team choice).
3. Add resource requests/limits; run as non-root (match Dockerfile).
4. Document `kubectl apply` or Helm install in README/docs.
5. Optional: Argo CD / Terraform when org defines (TBC in GitHub / product planning).

## Files / paths

- **Create:** `deploy/k8s/*.yaml` or `charts/access-manager/`

## Acceptance criteria

- Rolling update completes with zero unbounded downtime given readiness probe.

## Out of scope

- Service mesh unless required later.

## Dependencies

- **T19** image; **T22** env strategy; **T26** config keys.

## Curriculum link

**Theme 7** — Minikube-style path adapted to your cluster.

**Suggested P3 order:** after **T19** and Docker proven in CI.
