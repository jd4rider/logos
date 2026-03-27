#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_PATH="${1:-"$ROOT_DIR/wails/bin/logos"}"
TARGET_OS="${2:-${GOOS:-$(uname -s | tr '[:upper:]' '[:lower:]')}}"
TARGET_ARCH="${3:-${GOARCH:-$(uname -m)}}"

case "$TARGET_ARCH" in
  x86_64) TARGET_ARCH="amd64" ;;
  aarch64|arm64) TARGET_ARCH="arm64" ;;
esac

case "$TARGET_OS" in
  darwin|linux|windows) ;;
  *)
    echo "Unsupported GOOS: $TARGET_OS" >&2
    exit 1
    ;;
esac

mkdir -p "$(dirname "$OUTPUT_PATH")"

echo "Building Logos CLI for $TARGET_OS/$TARGET_ARCH -> $OUTPUT_PATH"
GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o "$OUTPUT_PATH" ./tui
