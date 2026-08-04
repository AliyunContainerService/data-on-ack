VERSION ?= v2.0.0
IMAGE ?= ai-dashboard
TAG ?= $(VERSION)-$(shell git rev-parse --short HEAD)
REGISTRY ?= registry.cn-beijing.aliyuncs.com/acs

.PHONY: build-frontend build-backend docker-build clean test vet

# Build frontend
build-frontend:
	cd frontend && npm ci --legacy-peer-deps && npm run build

# Build Go backend
build-backend:
	go build -o bin/ai-dashboard ./backend/cmd/server/

# Build everything locally (no Docker)
build: build-frontend build-backend

# Run Go tests
test:
	go test ./backend/internal/... -count=1 -v

# Run go vet
vet:
	go vet ./backend/...

# Build Docker image
docker-build: build-frontend
	docker buildx build -t $(REGISTRY)/$(IMAGE):$(TAG) -f Dockerfile .

# Clean build artifacts
clean:
	rm -rf frontend/dist bin/
