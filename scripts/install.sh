#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-jd4rider/logos}"
INSTALL_DIR="${LOGOS_INSTALL_DIR:-$HOME/.local/bin}"
APP_DIR="${LOGOS_APP_DIR:-$HOME/.local/share/logos-ai}"

info() { printf '[logos] %s\n' "$*"; }
fail() { printf '[logos] %s\n' "$*" >&2; exit 1; }

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *) fail "Unsupported OS for shell install. Use the native installer from the release page." ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) fail "Unsupported architecture." ;;
  esac
}

latest_tag() {
  curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n 1
}

main() {
  local os arch tag asset tmpdir
  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(latest_tag)"

  [[ -n "$tag" ]] || fail "Could not resolve the latest release tag."

  if [[ "$os" == "darwin" ]]; then
    asset="logos-ai-macos-universal.tar.gz"
  else
    asset="logos-ai-linux-${arch}-bundle.tar.gz"
  fi

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  info "Downloading $asset from $tag"
  curl -fsSL "https://github.com/$REPO/releases/download/$tag/$asset" -o "$tmpdir/archive.tar.gz"
  tar -xzf "$tmpdir/archive.tar.gz" -C "$tmpdir"

  mkdir -p "$INSTALL_DIR"

  if [[ "$os" == "darwin" ]]; then
    mkdir -p "$APP_DIR"
    rm -rf "$APP_DIR/logos-ai.app"
    cp -R "$tmpdir/logos-ai.app" "$APP_DIR/logos-ai.app"
    if [[ -f "$tmpdir/logos" ]]; then
      install -m 0755 "$tmpdir/logos" "$INSTALL_DIR/logos"
    elif [[ -f "$APP_DIR/logos-ai.app/Contents/Resources/logos" ]]; then
      install -m 0755 "$APP_DIR/logos-ai.app/Contents/Resources/logos" "$INSTALL_DIR/logos"
    else
      fail "Bundled CLI was not found in the macOS archive."
    fi
    cat >"$INSTALL_DIR/logos-ai" <<EOF
#!/usr/bin/env bash
exec open "$APP_DIR/logos-ai.app" --args "\$@"
EOF
    chmod 0755 "$INSTALL_DIR/logos-ai"
    info "Installed app to $APP_DIR/logos-ai.app"
    info "Launcher installed to $INSTALL_DIR/logos-ai"
  else
    install -m 0755 "$tmpdir/logos-ai" "$INSTALL_DIR/logos-ai"
    install -m 0755 "$tmpdir/logos" "$INSTALL_DIR/logos"
    info "Installed binaries to $INSTALL_DIR"
  fi

  info "If $INSTALL_DIR is not on PATH, add it to your shell profile."
}

main "$@"
