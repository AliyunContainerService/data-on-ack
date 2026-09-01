# AI Dev Console Build Guide

The dev console ships as a single image built from the **new stack**:

- Backend: `console/backend/internal/**` (Gin + K8s client + IMS OAuth), entry point `console/backend/cmd/server/main.go`
- Frontend: `console/frontend-new` (React + Vite + antd)
- Dockerfile: `Dockerfile.console-new`

The legacy stack (`console/backend/pkg/**` + `console/frontend`, built via
`Dockerfile.console`) is **deprecated** but kept buildable for reference.

The standalone operator (persist-controller) was removed from this repo in
2024-09 (commit ee663c8). Notebook lifecycle is reconciled by the
`notebook-controller` module at the repo root; training jobs are reconciled by
the in-cluster training operators. The old `manager` / `operator-build` targets
and the root operator Dockerfile have been retired.

## Prerequisites

- Go 1.22+
- Make
- Docker (for images)
- Node.js 20+ (frontends)

## Quick Start

```bash
# Build the console Docker image (new stack, builds frontend + backend inside)
make console-build

# Push
make console-push
```

## Individual Components

```bash
# New backend binary -> bin/server
make build-backend-new

# New frontend -> console/frontend-new/dist
make build-frontend-new

# Go checks
make vet
make test
```

## Deprecated (legacy stack)

```bash
make build-backend        # legacy backend binary
make build-frontend       # legacy frontend (umi) -> console/frontend/dist
make console-build-legacy # legacy image via Dockerfile.console
```

## Deployment

Install with Helm (see `charts/ack-ai-dev-console`):

```bash
helm install ai-dev-console charts/ack-ai-dev-console -n kube-ai \
  --set 'dev-console.console.ingress.hosts[0].host=<YOUR_DOMAIN>' \
  --set 'dev-console.console.ingress.hosts[0].paths[0]=/'
```

Configuration notes:

- Static credentials: create Secret `ai-dev-console-credentials` with
  `accessKeyId` / `accessKeySecret`.
- RRSA: set `console.credentialMode=rrsa`, `console.rrsa.oidcProviderArn` and
  `console.rrsa.roleArn` (the RAM role ARN to assume).
- Session persistence is automatic: the chart generates a `<release>-session`
  Secret (stable across upgrades) unless `auth.existingSecret` is provided.
