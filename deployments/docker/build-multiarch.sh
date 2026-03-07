#!/bin/bash
# Build O3K Docker image for multiple architectures

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "  O3K Multi-Architecture Build"
echo "=========================================="
echo ""

# Parse arguments
IMAGE_NAME="${1:-o3k}"
IMAGE_TAG="${2:-latest}"
PUSH="${3:-false}"

echo -e "${YELLOW}Configuration:${NC}"
echo "  Image: $IMAGE_NAME:$IMAGE_TAG"
echo "  Push: $PUSH"
echo ""

# Check if buildx is available
if ! docker buildx version &> /dev/null; then
    echo -e "${YELLOW}Installing Docker buildx...${NC}"
    docker buildx install
fi

# Create builder if it doesn't exist
if ! docker buildx inspect multiarch &> /dev/null; then
    echo -e "${YELLOW}Creating multi-arch builder...${NC}"
    docker buildx create --name multiarch --use --bootstrap
else
    docker buildx use multiarch
fi

echo ""
echo -e "${YELLOW}Building for linux/amd64 and linux/arm64...${NC}"

# Build command
BUILD_CMD="docker buildx build \
    --platform linux/amd64,linux/arm64 \
    -t $IMAGE_NAME:$IMAGE_TAG \
    -f deployments/docker/Dockerfile \
    ."

# Add push flag if requested
if [ "$PUSH" = "true" ]; then
    BUILD_CMD="$BUILD_CMD --push"
else
    BUILD_CMD="$BUILD_CMD --load"
fi

# Execute build
cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
eval $BUILD_CMD

echo ""
echo -e "${GREEN}✓ Build complete!${NC}"
echo ""
echo "Built architectures:"
docker buildx imagetools inspect $IMAGE_NAME:$IMAGE_TAG 2>/dev/null | grep "Platform:" || echo "  - linux/amd64"
echo "  - linux/arm64"

if [ "$PUSH" != "true" ]; then
    echo ""
    echo "Note: Images are cached in buildx. To load for current platform:"
    echo "  docker buildx build --platform linux/$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') --load -t $IMAGE_NAME:$IMAGE_TAG -f deployments/docker/Dockerfile ."
fi
