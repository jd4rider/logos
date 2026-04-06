package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jd4rider/logos/internal/appenv"
	"github.com/joho/godotenv"
)

func prepareDesktopRuntime() {
	loadRuntimeEnvFiles()
	extendRuntimePath()
	ensureTTSPython()
	maybeStartOllama()
}

func loadRuntimeEnvFiles() {
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(appenv.ConfigDir(), ".env"),
		filepath.Join(home, ".logos.env"),
		".env",
	}

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			_ = godotenv.Load(candidate)
		}
	}
}

func extendRuntimePath() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	entries := []string{
		appenv.BinDir(),
		appenv.VenvBinDir(),
		appenv.PiperDir(),
		filepath.Join(home, "go", "bin"),
	}
	if runtime.GOOS != "windows" {
		entries = append(entries,
			"/usr/local/bin",
			"/usr/local/sbin",
			"/opt/homebrew/bin",
			"/opt/homebrew/sbin",
			"/usr/bin",
			"/bin",
			"/usr/sbin",
			"/sbin",
		)
	}

	sep := string(os.PathListSeparator)
	current := strings.Split(os.Getenv("PATH"), sep)
	seen := make(map[string]bool, len(current)+len(entries))
	merged := make([]string, 0, len(current)+len(entries))

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, err := os.Stat(entry); err != nil {
			continue
		}
		if !seen[entry] {
			seen[entry] = true
			merged = append(merged, entry)
		}
	}

	for _, entry := range current {
		entry = strings.TrimSpace(entry)
		if entry == "" || seen[entry] {
			continue
		}
		seen[entry] = true
		merged = append(merged, entry)
	}

	_ = os.Setenv("PATH", strings.Join(merged, sep))
}

// ensureTTSPython finds the Python interpreter that has the piper module
// installed and prepends its directory to PATH.  This guarantees that
// #!/usr/bin/env python3 scripts (piper, kokoro-speak) use the right
// interpreter even when multiple Python versions are present.
func ensureTTSPython() {
	candidates := []string{
		strings.TrimSpace(runtimeEnv("LOGOS_PYTHON")),
		appenv.VenvPythonPath(),
	}
	if pointerPath := appenv.PythonPointerPath(); pointerPath != "" {
		if raw, err := os.ReadFile(pointerPath); err == nil {
			candidates = append(candidates, strings.TrimSpace(string(raw)))
		}
	}
	candidates = append(candidates,
		"python3",
		"python",
		"/usr/local/bin/python3",
		"/usr/local/bin/python3.11",
		"/opt/homebrew/opt/python@3.11/bin/python3",
	)

	for _, py := range candidates {
		py = strings.TrimSpace(py)
		if py == "" {
			continue
		}
		resolved := py
		if !strings.Contains(py, string(filepath.Separator)) {
			found, err := exec.LookPath(py)
			if err != nil {
				continue
			}
			resolved = found
		} else if _, err := os.Stat(py); err != nil {
			continue
		}
		if pythonHasModule(resolved, "piper") || pythonHasModule(resolved, "kokoro_onnx") {
			prependPath(filepath.Dir(resolved))
			return
		}
	}
}

func pythonHasModule(interpreter, module string) bool {
	cmd := exec.Command(interpreter, "-c", "import "+module)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func prependPath(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	current := os.Getenv("PATH")
	sep := string(os.PathListSeparator)
	if current == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	if strings.HasPrefix(current, dir+sep) || current == dir {
		return
	}
	_ = os.Setenv("PATH", dir+sep+current)
}

func maybeStartOllama() {
	if strings.EqualFold(runtimeEnv("LOGOS_DISABLE_OLLAMA_AUTOSTART"), "1") ||
		strings.EqualFold(runtimeEnv("LOGOS_DISABLE_OLLAMA_AUTOSTART"), "true") {
		return
	}
	if strings.TrimSpace(runtimeEnv("OLLAMA_HOST")) != "" &&
		!strings.HasPrefix(strings.TrimSpace(runtimeEnv("OLLAMA_HOST")), "http://localhost") &&
		!strings.HasPrefix(strings.TrimSpace(runtimeEnv("OLLAMA_HOST")), "https://localhost") &&
		!strings.HasPrefix(strings.TrimSpace(runtimeEnv("OLLAMA_HOST")), "http://127.0.0.1") {
		return
	}
	if _, err := exec.LookPath("ollama"); err != nil {
		return
	}
	if exec.Command("ollama", "list").Run() == nil {
		return
	}

	cmd := exec.Command("ollama", "serve")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Start()
}

func runtimeEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
