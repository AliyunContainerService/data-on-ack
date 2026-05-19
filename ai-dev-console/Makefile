
# Image URL to use all building/pushing image targets
VERSION:=v1.2.2
GIT_USER:=aliyun
GIT_COMMIT:=$(shell git rev-parse --short HEAD)
IMAGE_REGISTRY ?= registry.cn-beijing.aliyuncs.com
IMAGE_TAG ?= $(VERSION)-$(GIT_COMMIT)-aliyun
OPERATOR_IMG ?= $(IMAGE_REGISTRY)/$(GIT_USER)/persist-controller:$(IMAGE_TAG)
CONSOLE_IMG ?= $(IMAGE_REGISTRY)/$(GIT_USER)/kubeai-dev-console:$(IMAGE_TAG)
DOCKERCMD := docker build
PLATFORMS := linux/amd64

# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Build manager binary
manager: generate fmt vet vendor
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -mod=vendor -a -o manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && kustomize edit set image controller=${OPERATOR_IMG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook \
	paths=./apis/... paths=./controllers/... \
	output:crd:artifacts:config=config/crd/bases

vendor:
	@echo "Vendoring dependencies..."
	@go mod vendor || (echo "Vendor failed!" && exit 1)
	@echo "Vendor completed successfully"

# Run go fmt against code
fmt:
	@echo "Formatting Go code..."
	@go fmt ./... || (echo "fmt failed!" && exit 1)
	@echo "Formatting completed"

# Run go vet against code
vet:
	@echo "Vetting Go code..."
	@go vet ./... || (echo "vet failed!" && exit 1)
	@echo "Vetting completed"

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt,year=2020 \
	paths=./apis/... paths=./controllers/...


# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Build backend binary
build-backend: vendor
	@echo "Building backend server..."
	@cd console/backend/cmd/backend-server && \
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on \
		go build -mod=../../../../vendor -a -o ../../../../backend-server main.go || \
		(echo "Backend build failed!" && exit 1)
	@echo "Backend build completed successfully"

# Build frontend
build-frontend:
	@echo "Building frontend..."
	@cd console/frontend && \
		if [ ! -d "node_modules" ] || [ "package.json" -nt "node_modules/.package-lock.json" ]; then \
			echo "Installing/updating npm dependencies..."; \
			npm install --legacy-peer-deps || (echo "npm install failed!" && exit 1); \
		else \
			echo "Dependencies are up to date, skipping npm install"; \
		fi && \
		echo "Running frontend build..." && \
		(npm run build || (echo "Frontend build failed!" && exit 1)) && \
		(npm run verify || (echo "Frontend build verification failed!" && exit 1))
	@if [ ! -d "console/frontend/dist" ]; then \
		echo "Error: Frontend dist directory not found after build!"; \
		exit 1; \
	fi
	@if [ ! -f "console/frontend/dist/index.html" ]; then \
		echo "Error: Frontend index.html not found after build!"; \
		exit 1; \
	fi
	@echo "Frontend build completed successfully"

# Verify frontend build
verify-frontend:
	@if [ ! -d "console/frontend/dist" ]; then \
		echo "Error: Frontend dist directory not found. Please run 'make build-frontend' first."; \
		exit 1; \
	fi
	@if [ ! -f "console/frontend/dist/index.html" ]; then \
		echo "Error: Frontend index.html not found. Build may be incomplete."; \
		exit 1; \
	fi
	@echo "Frontend build verified"

# Save git commit hash
save-git-commit:
	@echo "${GIT_COMMIT}" > .git-commit
	@echo "Git commit saved: ${GIT_COMMIT}"

# Build console Docker image
console-build: verify-frontend save-git-commit
	@echo "Building console Docker image: ${CONSOLE_IMG}"
	@$(DOCKERCMD) \
		-f Dockerfile.console \
		-t ${CONSOLE_IMG} \
		--build-arg ARENA_TAR=${ARENA_TAR:-arena-installer-0.9.16-6c2373d-linux-amd64.tar.gz} \
		. || (echo "Console Docker build failed!" && exit 1)
	@echo "Console Docker image built successfully: ${CONSOLE_IMG}"

# Push console Docker image
console-push: console-build
	@echo "Pushing console Docker image: ${CONSOLE_IMG}"
	@docker push ${CONSOLE_IMG} || (echo "Console Docker push failed!" && exit 1)
	@echo "Console Docker image pushed successfully"

# Build operator Docker image
operator-build: manager
	@echo "Building operator Docker image: ${OPERATOR_IMG}"
	@docker build --platform linux/amd64 . --no-cache -f Dockerfile -t ${OPERATOR_IMG} || \
		(echo "Operator Docker build failed!" && exit 1)
	@echo "Operator Docker image built successfully: ${OPERATOR_IMG}"

# Push operator Docker image
operator-push: operator-build
	@echo "Pushing operator Docker image: ${OPERATOR_IMG}"
	@docker push ${OPERATOR_IMG} || (echo "Operator Docker push failed!" && exit 1)
	@echo "Operator Docker image pushed successfully"

# Build all components (console and operator)
docker-build: console-build operator-build
	@echo "All Docker images built successfully"
	@echo "Console image: ${CONSOLE_IMG}"
	@echo "Operator image: ${OPERATOR_IMG}"

# Build and push all components
docker-push: console-push operator-push
	@echo "All Docker images pushed successfully"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f backend-server manager .git-commit
	@rm -rf console/frontend/dist
	@rm -rf vendor
	@echo "Clean completed"

# Clean frontend (including node_modules)
clean-frontend:
	@echo "Cleaning frontend artifacts..."
	@cd console/frontend && npm run clean:all || true
	@echo "Frontend clean completed"

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Backend:"
	@echo "  build-backend      - Build backend Go binary"
	@echo ""
	@echo "Frontend:"
	@echo "  build-frontend     - Build frontend React application"
	@echo "  verify-frontend    - Verify frontend build artifacts"
	@echo "  clean-frontend    - Clean frontend artifacts (including node_modules)"
	@echo ""
	@echo "Docker:"
	@echo "  console-build      - Build console Docker image"
	@echo "  console-push       - Build and push console Docker image"
	@echo "  operator-build     - Build operator Docker image"
	@echo "  operator-push      - Build and push operator Docker image"
	@echo "  docker-build       - Build all Docker images"
	@echo "  docker-push        - Build and push all Docker images"
	@echo ""
	@echo "Utilities:"
	@echo "  clean              - Clean build artifacts"
	@echo "  help               - Show this help message"
