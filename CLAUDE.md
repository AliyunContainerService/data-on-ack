# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

**data-on-ack** is a cloud-native AI/ML platform built on Alibaba Cloud Container Service for Kubernetes (ACK). The repo is a monorepo containing five subprojects plus Helm charts for deployment.

## Project Structure

```
data-on-ack/
├── ai-dashboard/          # Cluster admin operations dashboard
│   ├── backend/           # Spring Boot (JDK 8/11, Maven, MySQL, MyBatis)
│   └── frontend/          # Vue 2 + Element UI (vue-admin-template)
├── ai-dev-console/        # Model dev/training console for engineers
│   ├── cmd/               # Go CLI entry points
│   ├── console/           # Gin-based Go web server + Vue frontend
│   ├── controllers/       # Kubernetes controllers
│   └── pkg/               # Shared Go packages
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

**Backend (Spring Boot):**
```bash
cd ai-dashboard/backend
mvn spring-boot:run          # Run dev server
mvn clean package -DskipTests # Build JAR
mvn test                     # Run tests
```

**Frontend (Vue 2):**
```bash
cd ai-dashboard/frontend
npm install
npm run dev                  # Dev server at http://localhost:9528
npm run build:prod           # Production build
npm run lint                 # Lint check
npm run test:unit            # Jest unit tests
```

**Full Docker image build (from ai-dashboard/):**
```bash
cd ai-dashboard
make docker-build            # Builds frontend, packages backend, creates Docker image
```

### ai-dev-console

```bash
cd ai-dev-console
make manager                 # Build operator binary
make build-backend           # Build console Go server
make build-frontend          # Build console Vue frontend (npm)
make console-build           # Docker build for console
make operator-build          # Docker build for operator
make docker-build            # Build all images
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

- **ai-dashboard**: Backend uses fabric8 kubernetes-client to interact with the cluster, OAuth2 for auth, MyBatis + MySQL for persistence, and Zuul as API gateway. Frontend is a vue-admin-template scaffold with i18n (zh/en), Element UI components, and Vue Router.
- **ai-dev-console**: Two binaries — an operator (controller-runtime based K8s controller) and a console server (Gin web framework with MySQL/GORM). Uses ack-arena SDK for job management.
- **commit-agent**: gRPC service supporting both Docker and containerd runtimes. Runs as sidecar in Jupyter pods for code sync.
- **notebook-controller**: kubeflow-style controller using controller-runtime. Manages `Notebook` CRDs (kubeflow.org/v1alpha1) and reconciles pods/services.
