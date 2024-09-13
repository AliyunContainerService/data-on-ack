IMG ?= commit-agent
VERSION ?= v0.1.1
GIT_SHORT_COMMIT=$(shell git rev-parse --short HEAD)
TAG=${VERSION}-${GIT_SHORT_COMMIT}-aliyun

.PHONY: build
build:
	@docker build  --rm --no-cache --build-arg VERSION=${VERSION} --build-arg GIT_SHORT_COMMIT=${GIT_SHORT_COMMIT} -t ${IMG}:${TAG} .

build-client:
	GOARCH=amd64 GOOS=linux CGO_ENABLED=0 go build -ldflags "-X main.version=${TAG} -s -w" -o bin/ack-commit-ctl cmd/commit-ctl/main.go
