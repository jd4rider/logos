# Bible TUI

A professional Bible reader with three interfaces sharing a common Go core.

## Apps

### TUI (Terminal)
```bash
cd tui
go run . [command]
# or
go build -o bible-tui .
./bible-tui              # interactive TUI
./bible-tui read GEN.1   # read a chapter
./bible-tui read GEN.1.1 # read a verse
./bible-tui search "in the beginning"
./bible-tui versions
./bible-tui speak GEN.1.1
```

### Web App
```bash
cd web/frontend && npm install && npm run build && cd ..
go run .
# Opens at http://localhost:8484
```

### Wails Desktop App
```bash
# Requires Wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd wails
wails build
# or for development:
wails dev
```

## Setup
1. Copy `.env` and set your API.Bible key
2. Optionally set PIPER_MODEL path for Piper TTS

## TTS
- macOS: uses `say` command automatically
- Linux: uses espeak-ng or espeak
- Piper (high quality): set PIPER_MODEL env var
- Web/Wails: uses browser SpeechSynthesis API
