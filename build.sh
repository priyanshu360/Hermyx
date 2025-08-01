#!/bin/bash

set -e

# Project metadata
APP_NAME="hermyx"
MAIN_PATH="./cmd/main.go"
OUTPUT_DIR="./bin"

# Target platform (defaults to host)
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}

# File extension for Windows
EXT=""
if [ "$GOOS" == "windows" ]; then
  EXT=".exe"
fi

OUTPUT_PATH="$OUTPUT_DIR/${APP_NAME}-${GOOS}-${GOARCH}${EXT}"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

echo "🔧 Building $APP_NAME for $GOOS/$GOARCH..."
GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$OUTPUT_PATH" "$MAIN_PATH"

if [ "$GOOS" != "windows" ]; then
  chmod +x "$OUTPUT_PATH"
fi

echo "✅ Build successful: $OUTPUT_PATH"
