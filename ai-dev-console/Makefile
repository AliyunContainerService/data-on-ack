# Image URL to use for building/pushing image targets
VERSION:=v3.1.0
GIT_USER:=aliyun
GIT_COMMIT:=$(shell git rev-parse --short HEAD)
IMAGE_REGISTRY ?= registry.cn-beijing.aliyuncs.com
IMAGE_TAG ?= $(VERSION)-$(GIT_COMMIT)-aliyun
CONSOLE_IMG ?= $(IMAGE_REGISTRY)/$(GIT_USER)/kubeai-dev-console:$(IMAGE_TAG)
CONSOLE_LEGACY_IMG ?= $(IMAGE_REGISTRY)/$(GIT_USER)/kubeai-dev-console-legacy:$(IMAGE_TAG)
DOCKERCMD := docker build

# NOTE: The standalone operator (persist-controller) was removed from this repo
# (commit ee663c8, 2024-09). Notebook lifecycle is handled by the
# notebook-controller module at the repo root; training jobs are reconciled by
# the in-cluster training operators. The corresponding `manager`/`operator-build`
# targets and the root operator Dockerfile have been retired.

all: console-build

# ---------- Backend ----------

# Build the new backend binary (canonical, used by charts/Dockerfile.console-new)
build-backend-new:
	@echo "Building new backend server..."
	CGO_ENABLED=0 go build -o bin/server ./console/backend/cmd/server/
	@echo "New backend build completed: bin/server"

# Build the legacy backend binary (deprecated; kept for reference)
build-backend:
	@echo "Building LEGACY backend server (deprecated)..."
	@cd console/backend/cmd/backend-server && \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ../../../../backend-server main.go || \
		(echo "Backend build failed!" && exit 1)
	@echo "Legacy backend build completed"

# ---------- Frontend ----------

# Build the new frontend (canonical)
build-frontend-new:
	@echo "Building new frontend..."
	@cd console/frontend-new && \
		(npm ci || npm install) && \
		(npm run build || (echo "Frontend build failed!" && exit 1))
	@if [ ! -f "console/frontend-new/dist/index.html" ]; then \
		echo "Error: frontend-new dist/index.html not found after build!"; exit 1; \
	fi
	@echo "New frontend build completed"

# Build the legacy frontend (deprecated)
build-frontend:
	@echo "Building LEGACY frontend (deprecated)..."
	@cd console/frontend && \
		(npm ci --legacy-peer-deps || npm install --legacy-peer-deps) && \
		(npm run build || (echo "Frontend build failed!" && exit 1))
	@if [ ! -f "console/frontend/dist/index.html" ]; then \
		echo "Error: console/frontend dist/index.html not found after build!"; exit 1; \
	fi
	@echo "Legacy frontend build completed"

# ---------- Docker images ----------

# Build the console Docker image (new stack: internal backend + frontend-new)
console-build:
	@echo "Building console Docker image (new stack): ${CONSOLE_IMG}"
	@$(DOCKERCMD) -f Dockerfile.console-new -t ${CONSOLE_IMG} . || \
		(echo "Console Docker build failed!" && exit 1)
	@echo "Console Docker image built successfully: ${CONSOLE_IMG}"

console-push: console-build
	@docker push ${CONSOLE_IMG} || (echo "Console Docker push failed!" && exit 1)

# Build the LEGACY console image (deprecated; old backend + old frontend dist)
console-build-legacy:
	@echo "Building LEGACY console Docker image: ${CONSOLE_LEGACY_IMG}"
	@$(DOCKERCMD) \
		-f Dockerfile.console \
		-t ${CONSOLE_LEGACY_IMG} \
		--build-arg ARENA_TAR=${ARENA_TAR:-arena-installer-0.9.16-6c2373d-linux-amd64.tar.gz} \
		. || (echo "Legacy console Docker build failed!" && exit 1)

# Build everything (new stack)
docker-build: console-build
	@echo "All Docker images built successfully"

docker-push: console-push
	@echo "All Docker images pushed successfully"

# ---------- Utilities ----------

fmt:
	@gofmt -l -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	@go vet ./...

test:
	@go test ./pkg/... ./apis/... ./console/backend/... -count=1

clean:
	@rm -rf backend-server bin .git-commit console/frontend-new/dist
	@echo "Clean completed"

help:
	@echo "Available targets:"
	@echo "  build-backend-new  - Build the new backend binary (bin/server)"
	@echo "  build-frontend-new - Build the new frontend (frontend-new/dist)"
	@echo "  console-build      - Build console Docker image (new stack)"
	@echo "  console-push       - Push console Docker image"
	@echo "  docker-build       - Build all Docker images (new stack)"
	@echo "  build-backend      - [deprecated] Build legacy backend binary"
	@echo "  build-frontend     - [deprecated] Build legacy frontend"
	@echo "  console-build-legacy - [deprecated] Build legacy console image"
	@echo "  fmt / vet / test   - Go formatting / vetting / tests"
	@echo "  clean              - Clean build artifacts"
