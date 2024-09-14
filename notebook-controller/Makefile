
IMG ?= notebook-controller
VERSION ?= v1.0.1
GIT_SHORT_COMMIT=$(shell git rev-parse --short HEAD)
TAG ?= ${VERSION}-${GIT_SHORT_COMMIT}-aliyun
SHELL := /bin/bash
GOLANG_VERSION ?= 1.15

USE_IMAGE_CACHE ?= true

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
