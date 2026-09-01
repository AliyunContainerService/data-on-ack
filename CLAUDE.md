# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

**data-on-ack** is a cloud-native AI/ML platform built on Alibaba Cloud Container Service for Kubernetes (ACK). The repo is a monorepo containing five subprojects plus Helm charts for deployment.

## Project Structure

```
data-on-ack/
├── ai-dashboard/          # Cluster admin operations dashboard
│   ├── backend/           # Go (Gin) backend; entry: backend/cmd/server/main.go
│   └── frontend/          # React 18 + Vite + antd (TypeScript)
├── ai-dev-console/        # Model dev/training console for engineers
│   ├── console/backend/   # Go backends:
│   │   ├── cmd/server/    #   new stack entry point (canonical)
│   │   ├── internal/      #   new stack implementation (canonical)
│   │   ├── cmd/backend-server/ # legacy entry point (deprecated)
│   │   └── pkg/           #   legacy implementation (deprecated)
│   ├── console/frontend-new/ # React + Vite frontend (canonical)
│   ├── console/frontend/  # legacy umi frontend (deprecated)
│   ├── apis/              # CRD type definitions (training/notebook/data)
│   └── pkg/               # Shared Go packages (infra backends, job controller)
├── commit-agent/          # gRPC sidecar for Jupyter notebook pods
│   ├── cmd/               # CLI client (commit-ctl)
│   ├── pkg/               # gRPC server implementation
│   └── v1beta1/           # Protobuf definitions
├── notebook-controller/   # Kubernetes controller for Notebook CRDs
│   ├── api/               # CRD type definitions
│   ├── controllers/       # Reconciliation logic
│   └── pkg/               # Shared packages
└── charts/                # Helm charts
    ├── ack-ai-dashboard/
    └── ack-ai-dev-console/
```

## Build & Development Commands

### ai-dashboard

**Backend (Go/Gin):**
```bash
cd ai-dashboard
go build ./...               # Build all packages (entry: backend/cmd/server)
go test ./backend/internal/... -count=1
```

**Frontend (React + Vite):**
```bash
cd ai-dashboard/frontend
npm ci --legacy-peer-deps
npm run dev                  # Dev server (proxies /user /ops /k8s ... to :8080)
npm run build                # tsc -b && vite build
```

**Full Docker image build (from ai-dashboard/):**
```bash
cd ai-dashboard
make docker-build            # Builds frontend, packages backend, creates Docker image
```

### ai-dev-console

```bash
cd ai-dev-console
make build-backend-new       # Build new backend binary (bin/server)
make build-frontend-new      # Build new frontend (frontend-new/dist)
make console-build           # Docker build for console (new stack, Dockerfile.console-new)
make docker-build            # Build all images (new stack)
make vet && make test        # Go vet / tests
```

### commit-agent

```bash
cd commit-agent
make build                   # Docker build
make build-client            # Build standalone CLI client binary
# Generate gRPC code:
cd v1beta1 && protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative service.proto
```

### notebook-controller

```bash
cd notebook-controller
make build                   # Docker build
make push                    # Push image
make generate                # Generate code from CRDs
```

### Helm Charts

Charts in `charts/ack-ai-dashboard/` and `charts/ack-ai-dev-console/` use standard Helm workflows (`helm install`, `helm upgrade`, `helm template`).

## Key Architecture Notes

- **ai-dashboard**: Go backend (Gin) talking to the cluster via client-go and to Alibaba Cloud IMS for RAM OAuth2 login; session cookies (gorilla sessions). Frontend is React 18 + antd + zustand with i18n (zh/en). The backend entry point is `backend/cmd/server/main.go`; most management APIs require the admin role (`adminOnly` middleware).
- **ai-dev-console**: Canonical stack is the new backend (`console/backend/internal`, Gin + client-go + IMS OAuth, per-tenant clients via SA tokens) with the `frontend-new` React frontend, built by `Dockerfile.console-new`. The legacy backend (`console/backend/pkg`, Gin + MySQL/GORM + arena SDK) is deprecated but still buildable. The standalone operator was removed in 2024-09; Notebook CRs are reconciled by the root `notebook-controller` module and training jobs by in-cluster operators.
- **commit-agent**: gRPC service supporting both Docker and containerd runtimes. Runs as sidecar in Jupyter pods for code sync.
- **notebook-controller**: kubeflow-style controller using controller-runtime. Manages `Notebook` CRDs (kubeflow.org/v1alpha1) and reconciles pods/services.
