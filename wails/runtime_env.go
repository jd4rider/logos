package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

func prepareDesktopRuntime() {
	loadRuntimeEnvFiles()
	extendRuntimePath()
	ensureTTSPython()
}

func loadRuntimeEnvFiles() {
	home, err := os.UserHomeDir()
	if err != nil {
		_ = godotenv.Load(".env")
		return
	}

	candidates := []string{
		filepath.Join(home, ".config", "logos", ".env"),
		filepath.Join(home, ".logos.env"),
		".env",
	}

	for _, candidate := range candidates {
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
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".local", "share", "piper"),
		filepath.Join(home, "go", "bin"),
		// /usr/local/bin MUST come before /opt/homebrew/bin so that piper and
		// kokoro scripts (which use #!/usr/bin/env python3) resolve to the
		// Python 3.11 installation that has the piper/kokoro_onnx packages,
		// rather than a newer Homebrew Python that may not have them.
		"/usr/local/bin",
		"/usr/local/sbin",
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
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
		"/usr/local/bin/python3",
		"/usr/local/bin/python3.11",
		"/opt/homebrew/opt/python@3.11/bin/python3",
	}
	for _, py := range candidates {
		if _, err := os.Stat(py); err != nil {
			continue
		}
		_, err := exec.Command(py, "-c", "import piper").Output()
		if err == nil {
			dir := filepath.Dir(py)
			current := os.Getenv("PATH")
			sep := string(os.PathListSeparator)
			if !strings.HasPrefix(current, dir+sep) {
				_ = os.Setenv("PATH", dir+sep+current)
			}
			return
		}
	}
}

func runtimeEnv(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
