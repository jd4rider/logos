#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

fail() {
  printf '[release] %s\n' "$*" >&2
  exit 1
}

sha256_file() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    fail "No SHA256 tool found. Install shasum or sha256sum."
  fi
}

VERSION_INPUT="${1:-}"
MACOS_ARCHIVE="${2:-}"
WINDOWS_BUNDLE="${3:-}"

[[ -n "$VERSION_INPUT" && -n "$MACOS_ARCHIVE" && -n "$WINDOWS_BUNDLE" ]] || fail \
  "Usage: scripts/release/render_manifests.sh <version> <macos-tar.gz> <windows-bundle.zip>"

[[ -f "$MACOS_ARCHIVE" ]] || fail "Missing macOS archive: $MACOS_ARCHIVE"
[[ -f "$WINDOWS_BUNDLE" ]] || fail "Missing Windows bundle: $WINDOWS_BUNDLE"

VERSION="${VERSION_INPUT#v}"
MACOS_SHA="$(sha256_file "$MACOS_ARCHIVE")"
WINDOWS_SHA="$(sha256_file "$WINDOWS_BUNDLE")"

cat >"$ROOT_DIR/packaging/homebrew/Casks/logos-ai.rb" <<EOF
# typed: false
# frozen_string_literal: true

cask "logos-ai" do
  version "$VERSION"
  sha256 "$MACOS_SHA"

  url "https://github.com/jd4rider/logos/releases/download/v#{version}/logos-ai-macos-universal.tar.gz"
  name "Logos AI"
  desc "Bible study desktop app with bundled Logos CLI"
  homepage "https://logos-ai.online"

  app "logos-ai.app"
  binary "logos"
end
EOF

cat >"$ROOT_DIR/packaging/scoop/logos-ai.json" <<EOF
{
  "version": "$VERSION",
  "description": "Bible study desktop app with bundled Logos CLI.",
  "homepage": "https://logos-ai.online",
  "license": "MIT",
  "architecture": {
    "64bit": {
      "url": "https://github.com/jd4rider/logos/releases/download/v$VERSION/logos-ai-windows-amd64-bundle.zip",
      "hash": "$WINDOWS_SHA",
      "bin": [
        "logos.exe"
      ],
      "shortcuts": [
        [
          "logos-ai.exe",
          "Logos AI"
        ]
      ]
    }
  }
}
EOF

printf '[release] Updated Homebrew cask and Scoop manifest for %s\n' "$VERSION"
