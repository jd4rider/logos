package tts

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const directSpeechStartDelay = 200 * time.Millisecond

func isEstimatedDurationEngine(engine string) bool {
	switch engine {
	case "say", "windows", "espeak", "speechd":
		return true
	default:
		return false
	}
}

func displayEngineName(engine string) string {
	switch engine {
	case "speechd":
		return "Speech Dispatcher"
	case "espeak":
		return "eSpeak"
	default:
		return strings.Title(engine) //nolint:staticcheck
	}
}

func (e *Engine) listBasicVoices() []VoiceEntry {
	switch runtime.GOOS {
	case "windows":
		return e.listWindowsVoices()
	case "linux":
		return e.listLinuxVoices()
	default:
		return nil
	}
}

func (e *Engine) listWindowsVoices() []VoiceEntry {
	shell := firstExecutable("powershell", "pwsh")
	if shell == "" {
		return nil
	}

	script := "Add-Type -AssemblyName System.Speech; " +
		"$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; " +
		"$s.GetInstalledVoices() | ForEach-Object { $_.VoiceInfo.Name }"
	out, err := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil
	}

	var voices []VoiceEntry
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		voices = append(voices, VoiceEntry{
			Name:   "Windows: " + name,
			ID:     name,
			Engine: "windows",
		})
	}

	if len(voices) == 0 {
		voices = append(voices, VoiceEntry{
			Name:   "Windows: Default Voice",
			ID:     "",
			Engine: "windows",
		})
	}

	return voices
}

func (e *Engine) listLinuxVoices() []VoiceEntry {
	if cmd := firstExecutable("espeak-ng", "espeak"); cmd != "" {
		label := "eSpeak"
		if strings.Contains(cmd, "espeak-ng") {
			label = "eSpeak NG"
		}
		return []VoiceEntry{
			{Name: label + ": English (US)", ID: "en-us", Engine: "espeak"},
			{Name: label + ": English (UK)", ID: "en-gb", Engine: "espeak"},
		}
	}

	if firstExecutable("spd-say") != "" {
		return []VoiceEntry{
			{Name: "Speech Dispatcher: Default", ID: "default", Engine: "speechd"},
		}
	}

	return nil
}

func (e *Engine) speakBasic(text string, started chan struct{}) error {
	cmd, err := e.commandForEstimatedEngine(text)
	if err != nil {
		return err
	}
	return e.startDirectSpeech(cmd, started)
}

func (e *Engine) commandForEstimatedEngine(text string) (*exec.Cmd, error) {
	switch e.activeEngine {
	case "say":
		args := []string{"-r", strconv.Itoa(e.rate)}
		if e.activeVoice.ID != "" {
			args = append(args, "-v", e.activeVoice.ID)
		}
		args = append(args, text)
		return exec.Command("say", args...), nil
	case "windows":
		return windowsSpeechCommand(e.activeVoice.ID, e.rate, text)
	case "espeak":
		return eSpeakCommand(e.activeVoice.ID, e.rate, text)
	case "speechd":
		return speechDispatcherCommand(e.rate, text)
	default:
		return nil, fmt.Errorf("no direct TTS engine available")
	}
}

func (e *Engine) startDirectSpeech(cmd *exec.Cmd, started chan struct{}) error {
	if err := cmd.Start(); err != nil {
		return err
	}

	e.cmd = cmd
	e.playerCmd = nil
	e.playing = true
	e.paused = false

	go func() {
		time.Sleep(directSpeechStartDelay)
		safeClose(started)
	}()

	go func() {
		_ = cmd.Wait()
		e.mu.Lock()
		e.playing = false
		if e.cmd == cmd {
			e.cmd = nil
		}
		e.mu.Unlock()
	}()

	return nil
}

func (e *Engine) runEstimatedChunk(text string, gen int) error {
	e.mu.Lock()
	cmd, err := e.commandForEstimatedEngine(text)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	e.cmd = cmd
	e.playing = true
	e.paused = false
	e.mu.Unlock()

	if err := cmd.Start(); err != nil {
		e.mu.Lock()
		e.cmd = nil
		e.playing = false
		e.mu.Unlock()
		return err
	}

	_ = cmd.Wait()

	e.mu.Lock()
	curGen := e.generation
	e.cmd = nil
	e.playing = false
	e.mu.Unlock()

	if curGen != gen {
		return fmt.Errorf("stopped")
	}
	return nil
}

func windowsSpeechCommand(voice string, wpm int, text string) (*exec.Cmd, error) {
	shell := firstExecutable("powershell", "pwsh")
	if shell == "" {
		return nil, fmt.Errorf("powershell is not available")
	}

	script := strings.Join([]string{
		"Add-Type -AssemblyName System.Speech",
		"$text = [Console]::In.ReadToEnd()",
		"$s = New-Object System.Speech.Synthesis.SpeechSynthesizer",
		"if ($env:LOGOS_TTS_VOICE) { try { $s.SelectVoice($env:LOGOS_TTS_VOICE) } catch {} }",
		"$s.Rate = [int]$env:LOGOS_TTS_RATE",
		"$s.Speak($text)",
	}, "; ")

	cmd := exec.Command(shell, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(text)
	cmd.Env = append(os.Environ(),
		"LOGOS_TTS_VOICE="+voice,
		fmt.Sprintf("LOGOS_TTS_RATE=%d", clamp((wpm-150)/10, -10, 10)),
	)
	return cmd, nil
}

func eSpeakCommand(voice string, wpm int, text string) (*exec.Cmd, error) {
	bin := firstExecutable("espeak-ng", "espeak")
	if bin == "" {
		return nil, fmt.Errorf("espeak-ng or espeak is not available")
	}
	if strings.TrimSpace(voice) == "" {
		voice = "en-us"
	}
	return exec.Command(bin, "-s", strconv.Itoa(wpm), "-v", voice, text), nil
}

func speechDispatcherCommand(wpm int, text string) (*exec.Cmd, error) {
	if firstExecutable("spd-say") == "" {
		return nil, fmt.Errorf("spd-say is not available")
	}
	rate := clamp(((wpm-150)*100)/110, -100, 100)
	return exec.Command("spd-say", "-w", "-r", strconv.Itoa(rate), text), nil
}

func firstExecutable(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func safeClose(ch chan struct{}) {
	defer func() {
		_ = recover()
	}()
	close(ch)
}
