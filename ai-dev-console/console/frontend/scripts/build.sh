#!/bin/bash

# Frontend build script with optimizations
# This script handles the frontend build process with proper error handling and caching

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    echo_step "Checking prerequisites..."
    
    if ! command -v node &> /dev/null; then
        echo_error "Node.js is not installed. Please install Node.js 16+"
        exit 1
    fi
    
    NODE_VERSION=$(node --version | sed 's/v//')
    echo_info "Node.js version: $NODE_VERSION"
    
    if ! command -v npm &> /dev/null; then
        echo_error "npm is not installed"
        exit 1
    fi
    
    NPM_VERSION=$(npm --version)
    echo_info "npm version: $NPM_VERSION"
    
    # Check Node.js version
    NODE_MAJOR=$(echo $NODE_VERSION | cut -d. -f1)
    if [ "$NODE_MAJOR" -lt 16 ]; then
        echo_error "Node.js 16+ is required. Current version: $NODE_VERSION"
        exit 1
    fi
}

# Check if dependencies need to be installed
check_dependencies() {
    echo_step "Checking dependencies..."
    
    cd "$FRONTEND_DIR"
    
    if [ ! -d "node_modules" ]; then
        echo_info "node_modules not found, installing dependencies..."
        npm install --legacy-peer-deps
    elif [ "package.json" -nt "node_modules/.package-lock.json" ] 2>/dev/null; then
        echo_warn "package.json is newer than node_modules, updating dependencies..."
        npm install --legacy-peer-deps
    else
        echo_info "Dependencies are up to date"
    fi
}

# Run linting
run_lint() {
    echo_step "Running linting..."
    cd "$FRONTEND_DIR"
    
    if npm run lint:js; then
        echo_info "Linting passed"
    else
        echo_warn "Linting found issues. Consider running 'npm run lint:fix'"
    fi
}

# Build frontend
build_frontend() {
    echo_step "Building frontend..."
    cd "$FRONTEND_DIR"
    
    # Clean previous build
    if [ -d "dist" ]; then
        echo_info "Cleaning previous build..."
        rm -rf dist
    fi
    
    # Run build
    echo_info "Starting production build..."
    if npm run build; then
        echo_info "Build completed successfully"
    else
        echo_error "Build failed!"
        exit 1
    fi
}

# Verify build
verify_build() {
    echo_step "Verifying build..."
    cd "$FRONTEND_DIR"
    
    if [ ! -d "dist" ]; then
        echo_error "dist directory not found!"
        exit 1
    fi
    
    if [ ! -f "dist/index.html" ]; then
        echo_error "dist/index.html not found!"
        exit 1
    fi
    
    echo_info "Build verification passed"
    echo_info "Build output size: $(du -sh dist | cut -f1)"
}

# Main execution
main() {
    echo_info "Starting frontend build process..."
    echo ""
    
    check_prerequisites
    echo ""
    
    check_dependencies
    echo ""
    
    # Optional: Run linting (can be skipped with --skip-lint)
    if [[ "$*" != *"--skip-lint"* ]]; then
        run_lint
        echo ""
    fi
    
    build_frontend
    echo ""
    
    verify_build
    echo ""
    
    echo_info "Frontend build completed successfully! ✓"
}

# Parse arguments
SKIP_LINT=false
for arg in "$@"; do
    case $arg in
        --skip-lint)
            SKIP_LINT=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --skip-lint    Skip linting before build"
            echo "  --help, -h     Show this help message"
            exit 0
            ;;
        *)
            echo_warn "Unknown option: $arg"
            ;;
    esac
done

main "$@"

