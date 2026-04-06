#!/usr/bin/env bash
set -euo pipefail

LOGOS_DATA_DIR="${LOGOS_DATA_DIR:-$HOME/.local/share/logos}"
LOGOS_BIN_DIR="${LOGOS_BIN_DIR:-$LOGOS_DATA_DIR/bin}"
LOGOS_VENV_DIR="${LOGOS_VENV_DIR:-$LOGOS_DATA_DIR/venv}"
LOGOS_PIPER_DIR="${LOGOS_PIPER_DIR:-$LOGOS_DATA_DIR/piper}"
LOGOS_KOKORO_DIR="${LOGOS_KOKORO_DIR:-$LOGOS_DATA_DIR/kokoro}"
LOGOS_PYTHON_POINTER="${LOGOS_PYTHON_POINTER:-$LOGOS_DATA_DIR/python_interp}"
LOGOS_OLLAMA_MODEL="${LOGOS_OLLAMA_MODEL:-llama3.2:3b}"
LOGOS_OLLAMA_EMBED_MODEL="${LOGOS_OLLAMA_EMBED_MODEL:-embeddinggemma}"
LOGOS_KOKORO_SCRIPT_SOURCE="${LOGOS_KOKORO_SCRIPT_SOURCE:-$LOGOS_DATA_DIR/kokoro_speak.py}"
LOGOS_KOKORO_MODEL_URL="${LOGOS_KOKORO_MODEL_URL:-https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.onnx}"
LOGOS_KOKORO_VOICES_URL="${LOGOS_KOKORO_VOICES_URL:-https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin}"

info() { printf '[logos-setup] %s\n' "$*"; }
warn() { printf '[logos-setup] %s\n' "$*" >&2; }

find_python() {
  local candidate
  for candidate in python3.12 python3.11 python3.10 python3 python; do
    if ! command -v "$candidate" >/dev/null 2>&1; then
      continue
    fi
    if "$candidate" -c 'import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)' >/dev/null 2>&1; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

download_file() {
  local url="$1"
  local dest="$2"
  mkdir -p "$(dirname "$dest")"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 "$url" -o "$dest"
    return 0
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$dest" "$url"
    return 0
  fi
  warn "Neither curl nor wget is available to download $url"
  return 1
}

ensure_ollama() {
  if command -v ollama >/dev/null 2>&1; then
    return 0
  fi

  info "Installing Ollama from the official installer..."
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://ollama.com/install.sh | sh
  else
    warn "curl is required to install Ollama automatically."
    return 1
  fi
}

start_ollama_if_needed() {
  if ! command -v ollama >/dev/null 2>&1; then
    return 1
  fi

  if ollama list >/dev/null 2>&1; then
    return 0
  fi

  info "Starting Ollama in the background..."
  mkdir -p "$LOGOS_DATA_DIR"
  nohup ollama serve >"$LOGOS_DATA_DIR/ollama.log" 2>&1 &

  local attempt
  for attempt in $(seq 1 30); do
    if ollama list >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  warn "Ollama did not become ready in time."
  return 1
}

pull_model_if_missing() {
  local model="$1"
  if ollama list 2>/dev/null | awk 'NR > 1 { print $1 }' | grep -Fxq "$model"; then
    info "Model already present: $model"
    return 0
  fi
  info "Pulling Ollama model: $model"
  ollama pull "$model"
}

main() {
  info "Preparing local AI and voice runtime..."
  mkdir -p "$LOGOS_DATA_DIR" "$LOGOS_BIN_DIR" "$LOGOS_PIPER_DIR" "$LOGOS_KOKORO_DIR"

  local python_bin
  if ! python_bin="$(find_python)"; then
    warn "Python 3.10+ was not found."
    warn "Install Python from https://www.python.org/downloads/ and run this setup again."
    exit 1
  fi

  info "Using Python: $python_bin ($("$python_bin" --version 2>&1))"

  if [[ ! -x "$LOGOS_VENV_DIR/bin/python" ]]; then
    info "Creating virtual environment at $LOGOS_VENV_DIR"
    "$python_bin" -m venv "$LOGOS_VENV_DIR"
  else
    info "Virtual environment already exists"
  fi

  local py="$LOGOS_VENV_DIR/bin/python"
  local pip="$LOGOS_VENV_DIR/bin/pip"
  export PATH="$LOGOS_BIN_DIR:$LOGOS_VENV_DIR/bin:$PATH"

  info "Upgrading pip"
  "$py" -m pip install --upgrade pip

  info "Installing Kokoro and Piper dependencies"
  "$pip" install --upgrade \
    kokoro-onnx \
    onnxruntime \
    numpy \
    soundfile \
    piper-tts

  if [[ ! -f "$LOGOS_KOKORO_DIR/kokoro-v1.0.onnx" ]]; then
    info "Downloading Kokoro model"
    download_file "$LOGOS_KOKORO_MODEL_URL" "$LOGOS_KOKORO_DIR/kokoro-v1.0.onnx"
  else
    info "Kokoro model already present"
  fi

  if [[ ! -f "$LOGOS_KOKORO_DIR/voices-v1.0.bin" ]]; then
    info "Downloading Kokoro voices"
    download_file "$LOGOS_KOKORO_VOICES_URL" "$LOGOS_KOKORO_DIR/voices-v1.0.bin"
  else
    info "Kokoro voices already present"
  fi

  if [[ ! -f "$LOGOS_KOKORO_SCRIPT_SOURCE" ]]; then
    warn "Expected Kokoro wrapper source at $LOGOS_KOKORO_SCRIPT_SOURCE"
    exit 1
  fi

  install -m 0755 "$LOGOS_KOKORO_SCRIPT_SOURCE" "$LOGOS_BIN_DIR/kokoro-speak.py"
  cat >"$LOGOS_BIN_DIR/kokoro-speak" <<EOF
#!/usr/bin/env bash
export LOGOS_KOKORO_DIR="$LOGOS_KOKORO_DIR"
exec "$py" "$LOGOS_BIN_DIR/kokoro-speak.py" "\$@"
EOF
  chmod 0755 "$LOGOS_BIN_DIR/kokoro-speak"

  if ! command -v piper >/dev/null 2>&1; then
    cat >"$LOGOS_BIN_DIR/piper" <<EOF
#!/usr/bin/env bash
exec "$py" -m piper "\$@"
EOF
    chmod 0755 "$LOGOS_BIN_DIR/piper"
  fi

  printf '%s\n' "$py" >"$LOGOS_PYTHON_POINTER"
  info "Saved interpreter pointer to $LOGOS_PYTHON_POINTER"

  if ensure_ollama && start_ollama_if_needed; then
    pull_model_if_missing "$LOGOS_OLLAMA_MODEL"
    pull_model_if_missing "$LOGOS_OLLAMA_EMBED_MODEL"
  else
    warn "Ollama setup could not be completed automatically."
    warn "Install it from https://ollama.com/download and rerun this setup to pull models."
  fi

  info "Setup complete."
  info "Restart Logos AI if the voice picker or AI tools do not refresh automatically."
}

main "$@"
