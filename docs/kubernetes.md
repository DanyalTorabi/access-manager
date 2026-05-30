# Kubernetes Deployment — access-manager

This guide covers deploying access-manager to a Kubernetes cluster using either **raw manifests** (`deploy/k8s/`) or the **Helm chart** (`charts/access-manager/`). Both approaches are fully equivalent; pick whichever fits your workflow.

See [docs/environments.md](environments.md) for the complete environment tier matrix and config/secrets reference.

---

## Prerequisites

| Tool | Minimum version | Notes |
|------|----------------|-------|
| `kubectl` | 1.27+ | [Install guide](https://kubernetes.io/docs/tasks/tools/) |
| `helm` | 3.12+ | [Install guide](https://helm.sh/docs/intro/install/) — required for Helm path only |
| `minikube` | 1.33+ | For local testing; optional for production clusters |
| A running cluster | — | Minikube, kind, EKS, GKE, AKS, etc. |

The image is published to **GHCR** by CI on every push to `main`:
```
ghcr.io/danyaltorabi/access-manager:<sha-tag>
```
Find the latest tag on the [GHCR package page](https://github.com/DanyalTorabi/access-manager/pkgs/container/access-manager) or in the CI `publish` job output.

---

## Create Secrets (required for all approaches)

**Never commit real secrets.** Create them manually with `kubectl` before deploying.

### App secrets

Secret names and keys must match exactly — they are referenced by both the raw manifests and the Helm chart:

```bash
# Database DSN (matches docs/environments.md: Secret access-manager-db, key url)
kubectl create secret generic access-manager-db \
  --from-literal=url='postgres://user:password@host:5432/access_manager?sslmode=disable' \
  -n access-manager

# Bearer token for /api/v1/* (matches docs/environments.md: Secret access-manager-auth, key token)
kubectl create secret generic access-manager-auth \
  --from-literal=token='<strong-random-bearer-token>' \
  -n access-manager
```

For **production**, use a managed Postgres instance (RDS, Cloud SQL, etc.) and supply its DSN in `access-manager-db`.

### In-cluster Postgres secrets (dev/Minikube only)

```bash
kubectl create secret generic access-manager-postgres \
  --from-literal=POSTGRES_USER=access \
  --from-literal=POSTGRES_PASSWORD='<strong-random-password>' \
  --from-literal=POSTGRES_DB=access_manager \
  -n access-manager
```

The `DATABASE_URL` must use the same credentials and point to the in-cluster service:
```
postgres://access:<password>@postgres.access-manager.svc.cluster.local:5432/access_manager?sslmode=disable
```

---

## Raw Manifests (`deploy/k8s/`)

### Deploy

```bash
# 1. (Minikube/dev) Deploy in-cluster Postgres first
kubectl apply -f deploy/k8s/postgres/

# 2. Wait for Postgres to be ready
kubectl rollout status statefulset/postgres -n access-manager

# 3. Deploy the app
kubectl apply -f deploy/k8s/

# 4. Wait for app rollout
kubectl rollout status deployment/access-manager -n access-manager

# 5. Verify
kubectl get pods -n access-manager
```

### Smoke test (port-forward)

```bash
kubectl port-forward svc/access-manager 8080:8080 -n access-manager &
curl -s http://127.0.0.1:8080/health
# Expected: {"status":"ok"}
```

### Rolling update

```bash
# Edit deploy/k8s/deployment.yaml: update image tag to the new SHA
# e.g.: image: ghcr.io/danyaltorabi/access-manager:sha-abc1234

kubectl apply -f deploy/k8s/deployment.yaml

# Watch rollout (maxUnavailable:0 means no traffic loss)
kubectl rollout status deployment/access-manager -n access-manager
```

### Kustomize

A `kustomization.yaml` ties all resources together. Uncomment the postgres overlay for dev clusters:

```bash
# Dry-run
kubectl apply --dry-run=client -k deploy/k8s/

# Apply
kubectl apply -k deploy/k8s/
```

---

## Helm Chart (`charts/access-manager/`)

### Lint and dry-run

```bash
helm lint charts/access-manager/

# Preview rendered manifests
helm template access-manager charts/access-manager/ --debug

# Kubernetes dry-run
helm template access-manager charts/access-manager/ | kubectl apply --dry-run=client -f -
```

### Install

```bash
# With default values (postgres.enabled=true, image.tag=latest)
helm install access-manager charts/access-manager/ \
  --namespace access-manager \
  --create-namespace

# Production: external Postgres, pinned image tag
helm install access-manager charts/access-manager/ \
  --set postgres.enabled=false \
  --set image.tag=sha-abc1234 \
  --set ingress.host=access-manager.example.com \
  --namespace access-manager \
  --create-namespace
```

### Upgrade

```bash
helm upgrade access-manager charts/access-manager/ \
  --set image.tag=sha-newsha \
  --namespace access-manager
```

### Uninstall

```bash
helm uninstall access-manager --namespace access-manager
```

### Key values

| Value | Default | Description |
|-------|---------|-------------|
| `image.tag` | `latest` | Image tag — use a pinned `sha-<sha>` in staging/production |
| `image.pullPolicy` | `Always` | Set to `Never` when using a locally loaded Minikube image |
| `replicaCount` | `2` | Number of app replicas |
| `ingress.enabled` | `true` | Toggle Ingress resource |
| `ingress.host` | `access-manager.local` | Hostname — replace with your actual domain |
| `postgres.enabled` | `true` | Deploy in-cluster Postgres StatefulSet |
| `postgres.storage` | `10Gi` | PVC size for in-cluster Postgres |
| `secrets.db.name` | `access-manager-db` | Name of the K8s Secret with `DATABASE_URL` |
| `secrets.auth.name` | `access-manager-auth` | Name of the K8s Secret with `API_BEARER_TOKEN` |

---

## Minikube Local Testing

```bash
# Start cluster
minikube start --driver=docker

# Build and load image locally (avoids GHCR pull)
docker build -t ghcr.io/danyaltorabi/access-manager:local .
minikube image load ghcr.io/danyaltorabi/access-manager:local

# Create namespace and secrets
kubectl create namespace access-manager
kubectl create secret generic access-manager-postgres \
  --from-literal=POSTGRES_USER=access \
  --from-literal=POSTGRES_PASSWORD=devpassword \
  --from-literal=POSTGRES_DB=access_manager \
  -n access-manager
kubectl create secret generic access-manager-db \
  --from-literal=url='postgres://access:devpassword@postgres.access-manager.svc.cluster.local:5432/access_manager?sslmode=disable' \
  -n access-manager
kubectl create secret generic access-manager-auth \
  --from-literal=token='devtoken' \
  -n access-manager

# Deploy Postgres, then app
kubectl apply -f deploy/k8s/postgres/
kubectl rollout status statefulset/postgres -n access-manager
kubectl apply -f deploy/k8s/

# Use local image (not from GHCR)
kubectl set image deployment/access-manager \
  server=ghcr.io/danyaltorabi/access-manager:local \
  -n access-manager
kubectl rollout status deployment/access-manager -n access-manager

# Verify
kubectl port-forward svc/access-manager 8080:8080 -n access-manager &
curl -s http://127.0.0.1:8080/health
# Expected: {"status":"ok"}

# Test rolling update: reload image, trigger rollout
minikube image load ghcr.io/danyaltorabi/access-manager:local
kubectl rollout restart deployment/access-manager -n access-manager
kubectl rollout status deployment/access-manager -n access-manager
```

---

## Configuration Reference

All non-secret configuration is in the `ConfigMap`. Secret values are injected from K8s Secrets. See [go/README.md](../go/README.md#environment-variables) for the full config reference.

| Env var | Source | Value in K8s |
|---------|--------|--------------|
| `DATABASE_URL` | Secret `access-manager-db` key `url` | Postgres DSN |
| `API_BEARER_TOKEN` | Secret `access-manager-auth` key `token` | Strong random token |
| `DATABASE_DRIVER` | ConfigMap | `postgres` |
| `HTTP_ADDR` | ConfigMap | `0.0.0.0:8080` |
| `MIGRATIONS_DIR` | ConfigMap | `migrations/postgres` |
| `CORS_ALLOWED_ORIGINS` | ConfigMap | `none` (reverse proxy handles CORS) |
| `SHUTDOWN_TIMEOUT_SECONDS` | ConfigMap | `60` |

---

## Rollback

For application rollback instructions, see [docs/environments.md — Rollback procedure](environments.md#rollback-procedure).

The short form:
```bash
kubectl set image deployment/access-manager \
  server=ghcr.io/danyaltorabi/access-manager:sha-lastgoodsha \
  -n access-manager
kubectl rollout status deployment/access-manager -n access-manager
```

---

## Out of Scope (deferred)

The following are tracked in the codebase with `// TODO(T21): ...` comments and will be addressed in future tickets:

- Dedicated `/readyz` endpoint with database ping for readiness (currently both probes use `/health`)
- ServiceAccount with minimal RBAC (`automountServiceAccountToken: false` is already set)
- NetworkPolicy to restrict ingress/egress between pods
- HPA (Horizontal Pod Autoscaler)
- TLS termination in-cluster — use your ingress controller's TLS configuration or a reverse proxy
- Service mesh (Istio, Linkerd)
- Argo CD / Terraform — defer to when org infrastructure is defined
