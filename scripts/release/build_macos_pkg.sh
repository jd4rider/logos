#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WAILS_DIR="$ROOT_DIR/wails"
BIN_DIR="$WAILS_DIR/bin"
VERSION="${VERSION:-0.0.0-dev}"
APP_BUNDLE_NAME="${APP_BUNDLE_NAME:-Logos AI.app}"
PKG_IDENTIFIER="${PKG_IDENTIFIER:-online.logos-ai.installer}"
PKG_OUTPUT="${PKG_OUTPUT:-$BIN_DIR/logos-ai-macos-universal.pkg}"
APP_INSTALL_DIR="${APP_INSTALL_DIR:-/usr/local/share/logos-ai}"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script must run on macOS." >&2
  exit 1
fi

mkdir -p "$BIN_DIR"

"$ROOT_DIR/scripts/release/build_cli.sh" "$BIN_DIR/logos-amd64" darwin amd64
"$ROOT_DIR/scripts/release/build_cli.sh" "$BIN_DIR/logos-arm64" darwin arm64
lipo -create -output "$BIN_DIR/logos" "$BIN_DIR/logos-amd64" "$BIN_DIR/logos-arm64"
rm -f "$BIN_DIR/logos-amd64" "$BIN_DIR/logos-arm64"

(
  cd "$WAILS_DIR"
  wails3 task darwin:package:universal
)

PKG_ROOT="$BIN_DIR/pkgroot"
rm -rf "$PKG_ROOT"
mkdir -p "$PKG_ROOT$APP_INSTALL_DIR" "$PKG_ROOT/usr/local/bin"

cp -R "$BIN_DIR/logos-ai.app" "$PKG_ROOT$APP_INSTALL_DIR/$APP_BUNDLE_NAME"
cp "$BIN_DIR/logos" "$PKG_ROOT/usr/local/bin/logos"
chmod 755 "$PKG_ROOT/usr/local/bin/logos"
cat >"$PKG_ROOT/usr/local/bin/logos-ai" <<EOF
#!/usr/bin/env bash
exec open "$APP_INSTALL_DIR/$APP_BUNDLE_NAME" --args "\$@"
EOF
chmod 755 "$PKG_ROOT/usr/local/bin/logos-ai"

pkgbuild \
  --root "$PKG_ROOT" \
  --identifier "$PKG_IDENTIFIER" \
  --version "$VERSION" \
  --install-location "/" \
  "$PKG_OUTPUT"

echo "Created $PKG_OUTPUT"
