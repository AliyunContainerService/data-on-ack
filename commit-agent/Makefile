IMG ?= commit-agent
VERSION ?= v0.1.2
GIT_SHORT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
TAG := $(VERSION)-$(GIT_SHORT_COMMIT)-aliyun
PLATFORMS ?= linux/amd64,linux/arm64

LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.GitCommit=$(GIT_SHORT_COMMIT) \
	-X main.version=$(TAG)

GOFLAGS ?= -mod=vendor

.PHONY: all
all: fmt vet test build build-client

.PHONY: fmt
fmt:
	gofmt -l -w $(shell git ls-files '*.go' | grep -v '^vendor/')

.PHONY: vet
vet:
	GOOS=linux GOARCH=amd64 go vet $(GOFLAGS) ./...

# test runs the unit suites portable across hosts (pkg/utils today). The full
# tree pulls Linux-only dependencies (nerdctl, containerd internals) so it
# only compiles on a Linux runner — see `test-linux`.
TEST_PKGS ?= ./pkg/utils/...

.PHONY: test
test:
	go test $(GOFLAGS) $(TEST_PKGS)

.PHONY: test-linux
test-linux:
	GOOS=linux GOARCH=amd64 go test $(GOFLAGS) ./...

.PHONY: build
build:
	docker build --rm \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHORT_COMMIT=$(GIT_SHORT_COMMIT) \
		-t $(IMG):$(TAG) .

.PHONY: buildx
buildx:
	docker buildx build --rm \
		--platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_SHORT_COMMIT=$(GIT_SHORT_COMMIT) \
		-t $(IMG):$(TAG) .

.PHONY: build-client
build-client:
	GOARCH=amd64 GOOS=linux CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/ack-commit-ctl ./cmd/commit-ctl

.PHONY: build-client-arm64
build-client-arm64:
	GOARCH=arm64 GOOS=linux CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/ack-commit-ctl-arm64 ./cmd/commit-ctl

.PHONY: build-agent
build-agent:
	GOARCH=amd64 GOOS=linux CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/commit-agent ./cmd/commit-agent

.PHONY: clean
clean:
	rm -rf bin
