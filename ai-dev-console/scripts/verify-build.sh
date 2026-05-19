#!/bin/bash

# Build verification script for AI Dev Console
# This script verifies that all build prerequisites are met and components can be built

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    echo_info "Checking prerequisites..."
    
    # Check Go
    if ! command -v go &> /dev/null; then
        echo_error "Go is not installed. Please install Go 1.22+"
        exit 1
    fi
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    echo_info "Go version: $GO_VERSION"
    
    # Check Node.js
    if ! command -v node &> /dev/null; then
        echo_error "Node.js is not installed. Please install Node.js 16+"
        exit 1
    fi
    NODE_VERSION=$(node --version | sed 's/v//')
    echo_info "Node.js version: $NODE_VERSION"
    
    # Check npm
    if ! command -v npm &> /dev/null; then
        echo_error "npm is not installed"
        exit 1
    fi
    NPM_VERSION=$(npm --version)
    echo_info "npm version: $NPM_VERSION"
    
    # Check Docker (optional)
    if command -v docker &> /dev/null; then
        DOCKER_VERSION=$(docker --version)
        echo_info "Docker: $DOCKER_VERSION"
    else
        echo_warn "Docker is not installed (optional for local builds)"
    fi
    
    # Check Make
    if ! command -v make &> /dev/null; then
        echo_error "Make is not installed"
        exit 1
    fi
    echo_info "Make is available"
}

# Verify Go modules
verify_go_modules() {
    echo_info "Verifying Go modules..."
    cd "$(dirname "$0")/.."
    
    if [ ! -f "go.mod" ]; then
        echo_error "go.mod not found"
        exit 1
    fi
    
    echo_info "Running go mod verify..."
    go mod verify || {
        echo_error "Go module verification failed"
        exit 1
    }
    
    echo_info "Go modules verified"
}

# Verify frontend dependencies
verify_frontend() {
    echo_info "Verifying frontend setup..."
    cd "$(dirname "$0")/../console/frontend"
    
    if [ ! -f "package.json" ]; then
        echo_error "package.json not found"
        exit 1
    fi
    
    if [ ! -d "node_modules" ]; then
        echo_warn "node_modules not found. Run 'npm install --legacy-peer-deps' first"
    else
        echo_info "Frontend dependencies found"
    fi
}

# Verify build structure
verify_structure() {
    echo_info "Verifying project structure..."
    cd "$(dirname "$0")/.."
    
    REQUIRED_DIRS=(
        "console/backend/cmd/backend-server"
        "console/frontend"
        "pkg"
    )
    
    for dir in "${REQUIRED_DIRS[@]}"; do
        if [ ! -d "$dir" ]; then
            echo_error "Required directory not found: $dir"
            exit 1
        fi
    done
    
    echo_info "Project structure verified"
}

# Main execution
main() {
    echo_info "Starting build verification..."
    echo ""
    
    check_prerequisites
    echo ""
    
    verify_structure
    echo ""
    
    verify_go_modules
    echo ""
    
    verify_frontend
    echo ""
    
    echo_info "Build verification completed successfully!"
    echo_info "You can now run 'make build-backend' and 'make build-frontend'"
}

main "$@"

