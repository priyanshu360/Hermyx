#!/bin/bash

set -e

# Project metadata
APP_NAME="hermyx"
MAIN_PATH="./cmd/main.go"
OUTPUT_DIR="./bin"

# Target platform (defaults to host if not set)
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}

# Optional Windows extension
EXT=""
if [ "$GOOS" == "windows" ]; then
  EXT=".exe"
fi

OUTPUT_PATH="$OUTPUT_DIR/${APP_NAME}${EXT}"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

echo "🔧 Building $APP_NAME for $GOOS/$GOARCH..."
GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$OUTPUT_PATH" "$MAIN_PATH"

# Make executable if not Windows
if [ "$GOOS" != "windows" ]; then
  chmod +x "$OUTPUT_PATH"
fi

echo "✅ Build successful: $OUTPUT_PATH"
