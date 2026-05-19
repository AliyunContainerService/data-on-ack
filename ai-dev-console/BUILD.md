# AI Dev Console Build Guide

This guide provides instructions for building and deploying the AI Dev Console.

## Prerequisites

### Backend Requirements
- Go 1.22 or later
- Make
- Docker (for building images)

### Frontend Requirements
- Node.js 16 or later
- npm or yarn

## Quick Start

### Build All Components

```bash
# Build everything (backend, frontend, and Docker images)
make docker-build
```

### Build Individual Components

#### Backend Only
```bash
# Build backend Go binary
make build-backend
```

#### Frontend Only
```bash
# Build frontend React application
make build-frontend

# Verify frontend build
make verify-frontend
```

#### Docker Images

```bash
# Build console Docker image (includes frontend build)
make console-build

# Build operator Docker image
make operator-build

# Build all Docker images
make docker-build
```

## Build Process Details

### Backend Build

The backend is built using Go:

```bash
cd console/backend/cmd/backend-server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o backend-server main.go
```

### Frontend Build

The frontend is built using npm:

```bash
cd console/frontend
npm install --legacy-peer-deps
npm run build
```

The build output is placed in `console/frontend/dist/`.

### Docker Build

The Docker build process:

1. Builds the backend Go binary in a Go container
2. Copies the frontend dist directory
3. Installs Arena CLI tools
4. Creates the final runtime image

## Environment Variables

### Build-time Variables

- `VERSION`: Version tag (default: v1.2.2)
- `GIT_COMMIT`: Git commit hash (auto-detected)
- `IMAGE_REGISTRY`: Docker registry (default: registry.cn-beijing.aliyuncs.com)
- `IMAGE_TAG`: Image tag (default: ${VERSION}-${GIT_COMMIT}-aliyun)
- `ARENA_TAR`: Arena installer tar file name (default: arena-installer-0.9.16-6c2373d-linux-amd64.tar.gz)

### Example with Custom Variables

```bash
VERSION=v1.3.0 IMAGE_REGISTRY=my-registry.com make console-build
```

## Troubleshooting

### Frontend Build Fails

If the frontend build fails:

1. Check Node.js version: `node --version` (should be 16+)
2. Clear node_modules and reinstall:
   ```bash
   cd console/frontend
   rm -rf node_modules package-lock.json
   npm install --legacy-peer-deps
   ```
3. Check for dependency conflicts:
   ```bash
   npm audit
   ```

### Backend Build Fails

If the backend build fails:

1. Check Go version: `go version` (should be 1.22+)
2. Update dependencies:
   ```bash
   go mod tidy
   go mod vendor
   ```
3. Check for compilation errors:
   ```bash
   go build ./console/backend/cmd/backend-server
   ```

### Docker Build Fails

If Docker build fails:

1. Ensure Docker is running: `docker ps`
2. Check Dockerfile syntax
3. Verify frontend dist exists: `ls -la console/frontend/dist/`
4. Check network connectivity for Arena download

### Missing Dependencies

If you encounter missing dependencies:

```bash
# Backend
go mod download
go mod vendor

# Frontend
cd console/frontend
npm install --legacy-peer-deps
```

## Clean Build Artifacts

```bash
# Remove all build artifacts
make clean
```

This removes:
- Backend binaries (`backend-server`, `manager`)
- Frontend dist directory
- Vendor directory
- Git commit file

## Development Workflow

### Local Development

1. **Backend Development**:
   ```bash
   cd console/backend/cmd/backend-server
   go run main.go
   ```

2. **Frontend Development**:
   ```bash
   cd console/frontend
   npm start
   ```

### Testing Builds

Before pushing, test the build:

```bash
# Build and verify
make build-backend
make build-frontend
make verify-frontend

# Test Docker build locally
make console-build
docker run -p 9090:9090 ${CONSOLE_IMG}
```

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Build Console
  run: |
    make build-frontend
    make console-build
    make console-push
```

### GitLab CI Example

```yaml
build:
  script:
    - make build-frontend
    - make console-build
    - make console-push
```

## Available Make Targets

- `build-backend`: Build backend Go binary
- `build-frontend`: Build frontend React application
- `verify-frontend`: Verify frontend build artifacts
- `console-build`: Build console Docker image
- `console-push`: Build and push console Docker image
- `operator-build`: Build operator Docker image
- `operator-push`: Build and push operator Docker image
- `docker-build`: Build all Docker images
- `docker-push`: Build and push all Docker images
- `clean`: Clean build artifacts
- `help`: Show help message

## Best Practices

1. **Always verify frontend build** before Docker build
2. **Use vendor mode** for reproducible builds
3. **Tag images** with version and commit hash
4. **Test locally** before pushing to registry
5. **Clean artifacts** between builds in CI/CD

## Support

For issues or questions:
- Check component-specific README files
- Review build logs for detailed error messages
- Ensure all prerequisites are installed

