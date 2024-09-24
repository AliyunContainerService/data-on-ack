
# Image URL to use all building/pushing image targets
VERSION:=v1.1.2
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
	go mod vendor

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

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

build-backend: vendor
	cd console/ && env GOOS=linux GOARCH=amd64 go build -mod=vendor -o backend-server && cd -

build-frontend:
	cd console/frontend/ && npm install --legacy-peer-deps && npm run build && cd -

save-git-commit:
	echo "${GIT_COMMIT}" > .git-commit

console-build: build-frontend save-git-commit
	$(DOCKERCMD) \
		-f Dockerfile.console \
		-t ${CONSOLE_IMG} \
		.

console-push:
	docker push ${CONSOLE_IMG}

operator-build: manager
	docker build --platform linux/amd64 . --no-cache -f Dockerfile -t ${OPERATOR_IMG}

operator-push:
	docker push ${OPERATOR_IMG}

# Build the docker image
docker-build: console-build operator-build cron-build
