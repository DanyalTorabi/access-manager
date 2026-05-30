# T21 — Kubernetes Manifests + Helm Chart

## Ticket

**T21** — Kubernetes Manifests + Helm Chart (GitHub [#32](https://github.com/DanyalTorabi/access-manager/issues/32))

## Phase

**Phase 6** — P3 scale, prod, product options

## Goal

Ship production-ready Kubernetes deployment for access-manager: raw manifests **and** a Helm chart, both wiring liveness/readiness probes to `/health`, enforcing non-root pod security, and aligning config/secret names with the tier matrix in `docs/environments.md`. An optional in-cluster Postgres StatefulSet is included for Minikube/dev clusters; production users substitute a managed database.

## Deliverables

- `deploy/k8s/` — raw manifests (Namespace, ConfigMap, Deployment, Service, Ingress) + `postgres/` overlay (StatefulSet, Service)
- `deploy/k8s/kustomization.yaml` — Kustomize resource list tying everything together
- `charts/access-manager/` — Helm chart mirroring the raw manifests; Postgres is conditional on `.Values.postgres.enabled`
- `docs/kubernetes.md` — prerequisites, secret creation, `kubectl apply`, Helm install, rolling-update and rollback procedures

## Steps

1. **Branch** from latest `main`: `danyal/feature/t21-kubernetes`.
2. **Raw manifests — app:** Namespace, ConfigMap (non-secret env), `secret.example.yaml` (placeholder, not committed with real values), Deployment (2 replicas, RollingUpdate maxSurge:1/maxUnavailable:0, `/health` liveness + readiness, `securityContext` matching Dockerfile's distroless `nonroot` UID 65532, resource requests/limits mirroring `compose.prod.yml`), ClusterIP Service, nginx Ingress.
3. **Raw manifests — Postgres:** `postgres/` sub-directory with StatefulSet (postgres:15-alpine, PVC 10Gi, `pg_isready` readiness), headless ClusterIP Service.
4. **Kustomize base + `.gitignore`** patch to prevent accidental real-secret commits.
5. **Helm chart:** `Chart.yaml`, `values.yaml` (image.tag, replicaCount, resources, ingress.host, postgres.enabled), `templates/_helpers.tpl`, templates for all resources.
6. **Docs:** `docs/kubernetes.md` + root `README.md` K8s section + `CHANGELOG.md` Unreleased entry.
7. **Minikube smoke test:** build local image → load into Minikube → create secrets → apply manifests → rollout status → `curl /health` 200 OK → rolling-update test → `helm lint` + `helm template | kubectl apply --dry-run=client`.

## Files / paths

- **Create:** `deploy/k8s/*.yaml`, `deploy/k8s/postgres/*.yaml`, `deploy/k8s/kustomization.yaml`
- **Create:** `charts/access-manager/Chart.yaml`, `charts/access-manager/values.yaml`, `charts/access-manager/templates/`
- **Create:** `docs/kubernetes.md`
- **Update:** `README.md` (root), `CHANGELOG.md`

## Acceptance criteria

- `kubectl apply --dry-run=client -f deploy/k8s/` succeeds with no warnings.
- `helm lint charts/access-manager/` passes.
- `helm template access-manager charts/access-manager/ | kubectl apply --dry-run=client -f -` succeeds.
- Minikube deploy: `GET /health` → `{"status":"ok"}` 200 after rollout.
- Rolling update with `maxUnavailable:0` completes without dropping traffic (readiness probe gates pod inclusion).
- Secret names match `docs/environments.md` exactly: `access-manager-db` (key `url`) and `access-manager-auth` (key `token`).
- No real secrets committed; `secret.example.yaml` contains only placeholder values.
- `make test` and `make lint` pass (no Go code changed).

## Out of scope

- Service mesh (Istio/Linkerd).
- Argo CD / Terraform — defer to a future ticket when org infrastructure is defined.
- HPA, NetworkPolicy, ServiceAccount/RBAC — deferred; `// TODO(T21): ...` comments left at relevant locations.
- TLS termination in-cluster — handled by ingress controller or reverse proxy (per `docs/environments.md`).
- Dedicated `/readyz` endpoint with DB ping — deferred; `// TODO(T21): ...` comment in handler.

## Dependencies

- **T19** Docker image + GHCR publish; **T22** env/secrets tier matrix (`docs/environments.md`); **T26** config keys.

## Curriculum link

**Theme 7** — Minikube-style path adapted to your cluster.

**Suggested P3 order:** after **T19** and Docker proven in CI.
