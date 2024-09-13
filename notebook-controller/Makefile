
IMG ?= notebook-controller
VERSION ?= v1.0.1
GIT_SHORT_COMMIT=$(shell git rev-parse --short HEAD)
TAG ?= ${VERSION}-${GIT_SHORT_COMMIT}-aliyun
SHELL := /bin/bash
GOLANG_VERSION ?= 1.15

USE_IMAGE_CACHE ?= true

CRD_OPTIONS ?= "crd"

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

check-license:
	./third_party/check-license.sh

test: generate fmt vet manifests
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/a9bd9117a77a2f84bbc546e28991136fe0000dc0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR);
	go test ./api/... ./controllers/... -coverprofile cover.out

manager: generate fmt vet
	go build -o bin/manager main.go

run: generate fmt vet
	go run ./main.go

install: manifests
	kubectl apply -f config/crd/bases

deploy: manifests
	kubectl apply -f config/crd/bases
	kustomize build config/default | kubectl apply -f -

manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

fmt:
	go fmt ./...

vet:
	go vet ./...

generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile=./hack/boilerplate.go.txt paths=./api/...

docker-build: test
	cd .. && docker build . -t ${IMG}:${TAG} -f ./notebook-controller/Dockerfile
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}:${TAG}"'@' ./config/default/manager_image_patch.yaml

docker-push:
	docker push ${IMG}:${TAG}

controller-gen:
ifeq (, $(shell which controller-gen))
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.0
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

build:
	docker build -t $(IMG):$(TAG) .
	@echo Built $(IMG):$(TAG)

push:
	docker push $(IMG):$(TAG)
	@echo Pushed $(IMG):$(TAG)
