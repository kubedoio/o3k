#!/bin/bash
# Build O3K Docker image for current platform

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "  O3K Docker Build (Current Platform)"
echo "=========================================="
echo ""

# Detect current architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        PLATFORM="linux/amd64"
        ;;
    aarch64|arm64)
        PLATFORM="linux/arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

echo -e "${YELLOW}Building for:${NC} $PLATFORM ($ARCH)"
echo ""

# Navigate to project root
cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# Build
docker build \
    --platform $PLATFORM \
    -t lightstack-o3k:latest \
    -f deployments/docker/Dockerfile \
    .

echo ""
echo -e "${GREEN}✓ Build complete!${NC}"
echo ""
echo "Image: lightstack-o3k:latest"
echo "Platform: $PLATFORM"
echo ""
echo "To run:"
echo "  docker compose up -d"
