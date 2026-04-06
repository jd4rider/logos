package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/ai"
	"github.com/jd4rider/logos/internal/appenv"
)

type LocalSetupStatus struct {
	NeedsSetup      bool   `json:"needsSetup"`
	PythonReady     bool   `json:"pythonReady"`
	PiperReady      bool   `json:"piperReady"`
	KokoroReady     bool   `json:"kokoroReady"`
	OllamaInstalled bool   `json:"ollamaInstalled"`
	OllamaRunning   bool   `json:"ollamaRunning"`
	ChatModel       string `json:"chatModel"`
	EmbedModel      string `json:"embedModel"`
}

func (s *LogosService) GetLocalSetupStatus() LocalSetupStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client := ai.NewClient()
	voices := s.ttsEngine.RefreshVoices()
	piperReady := false
	kokoroReady := false
	for _, voice := range voices {
		if voice.Engine == "piper" {
			piperReady = true
		}
		if voice.Engine == "kokoro" {
			kokoroReady = true
		}
	}
	status := LocalSetupStatus{
		PythonReady:     fileExists(appenv.PythonPointerPath()) || fileExists(appenv.VenvPythonPath()),
		PiperReady:      piperReady,
		KokoroReady:     kokoroReady,
		OllamaInstalled: client.IsInstalled(),
		OllamaRunning:   client.IsAvailable(ctx),
		ChatModel:       client.Model(),
		EmbedModel:      client.EmbedModel(),
	}
	status.NeedsSetup = !status.PythonReady || !status.KokoroReady || !status.OllamaRunning
	return status
}

func (s *LogosService) RunSetupScript() error {
	setupDir := filepath.Join(appenv.DataDir(), "setup")
	if err := os.MkdirAll(setupDir, 0o755); err != nil {
		return err
	}

	kokoroSource := filepath.Join(setupDir, "kokoro_speak.py")
	if err := os.WriteFile(kokoroSource, []byte(kokoroSpeakScript), 0o755); err != nil {
		return err
	}

	switch runtime.GOOS {
	case "windows":
		scriptPath := filepath.Join(setupDir, "setup-runtime.ps1")
		if err := os.WriteFile(scriptPath, []byte(setupRuntimePowerShell), 0o755); err != nil {
			return err
		}

		shell := resolveExecutable("powershell", "pwsh")
		if shell == "" {
			return fmt.Errorf("PowerShell is not available on PATH")
		}

		cmd := exec.Command(shell, "-NoExit", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
		cmd.Env = append(os.Environ(), setupEnvironment(kokoroSource)...)
		return cmd.Start()
	default:
		scriptPath := filepath.Join(setupDir, "setup-runtime.sh")
		if err := os.WriteFile(scriptPath, []byte(setupRuntimeShell), 0o755); err != nil {
			return err
		}

		launcherPath := filepath.Join(setupDir, "launch-setup.sh")
		if err := os.WriteFile(launcherPath, []byte(unixSetupLauncher(scriptPath, kokoroSource)), 0o755); err != nil {
			return err
		}

		switch runtime.GOOS {
		case "darwin":
			return exec.Command("open", "-a", "Terminal", launcherPath).Start()
		case "linux":
			return startLinuxSetupLauncher(launcherPath)
		default:
			return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}
	}
}

func unixSetupLauncher(scriptPath, kokoroSource string) string {
	var builder strings.Builder
	builder.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n")
	for _, envEntry := range setupEnvironment(kokoroSource) {
		parts := strings.SplitN(envEntry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		builder.WriteString("export ")
		builder.WriteString(parts[0])
		builder.WriteString("=")
		builder.WriteString(shellQuote(parts[1]))
		builder.WriteString("\n")
	}
	builder.WriteString("exec bash ")
	builder.WriteString(shellQuote(scriptPath))
	builder.WriteString("\n")
	return builder.String()
}

func setupEnvironment(kokoroSource string) []string {
	client := ai.NewClient()
	return []string{
		"LOGOS_DATA_DIR=" + appenv.DataDir(),
		"LOGOS_BIN_DIR=" + appenv.BinDir(),
		"LOGOS_VENV_DIR=" + appenv.VenvDir(),
		"LOGOS_PIPER_DIR=" + appenv.PiperDir(),
		"LOGOS_KOKORO_DIR=" + appenv.KokoroDir(),
		"LOGOS_PYTHON_POINTER=" + appenv.PythonPointerPath(),
		"LOGOS_OLLAMA_MODEL=" + client.Model(),
		"LOGOS_OLLAMA_EMBED_MODEL=" + client.EmbedModel(),
		"LOGOS_KOKORO_SCRIPT_SOURCE=" + kokoroSource,
	}
}

func startLinuxSetupLauncher(launcherPath string) error {
	launchers := []struct {
		command string
		args    []string
	}{
		{command: "x-terminal-emulator", args: []string{"-e", launcherPath}},
		{command: "gnome-terminal", args: []string{"--", launcherPath}},
		{command: "konsole", args: []string{"-e", launcherPath}},
		{command: "xfce4-terminal", args: []string{"-e", launcherPath}},
		{command: "xterm", args: []string{"-e", launcherPath}},
	}

	for _, launcher := range launchers {
		path, err := exec.LookPath(launcher.command)
		if err != nil {
			continue
		}
		return exec.Command(path, launcher.args...).Start()
	}

	return fmt.Errorf("no supported Linux terminal launcher found")
}

func resolveExecutable(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
