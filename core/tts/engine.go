package tts

import (
"os/exec"
"strings"
"sync"
)

// Engine is a TTS engine that auto-detects available backends
type Engine struct {
mu      sync.Mutex
cmd     *exec.Cmd
playing bool
model   string
engine  string
}

// New creates a new TTS engine, auto-detecting the available backend
func New(piperModel string) *Engine {
return &Engine{
model:  piperModel,
engine: detectEngine(piperModel),
}
}

func detectEngine(piperModel string) string {
if piperModel != "" {
if _, err := exec.LookPath("piper"); err == nil {
return "piper"
}
}
if _, err := exec.LookPath("kokoro"); err == nil {
return "kokoro"
}
if _, err := exec.LookPath("say"); err == nil {
return "say"
}
if _, err := exec.LookPath("espeak-ng"); err == nil {
return "espeak-ng"
}
if _, err := exec.LookPath("espeak"); err == nil {
return "espeak"
}
return "none"
}

// EngineName returns the name of the detected TTS engine
func (e *Engine) EngineName() string {
return e.engine
}

// Available returns true if a TTS engine is available
func (e *Engine) Available() bool {
return e.engine != "none"
}

// Speak starts speaking the given text asynchronously
func (e *Engine) Speak(text string) error {
e.mu.Lock()
defer e.mu.Unlock()

if e.cmd != nil && e.playing {
_ = e.cmd.Process.Kill()
e.cmd = nil
e.playing = false
}

var cmd *exec.Cmd
switch e.engine {
case "say":
cmd = exec.Command("say", text)
case "espeak-ng":
cmd = exec.Command("espeak-ng", "-s", "160", text)
case "espeak":
cmd = exec.Command("espeak", "-s", "160", text)
case "piper":
pipeline := "echo " + shellQuote(text) + " | piper --output-raw"
if e.model != "" {
pipeline += " --model " + shellQuote(e.model)
}
if _, err := exec.LookPath("afplay"); err == nil {
pipeline += " | afplay -f RAW -r 22050 -b 16 -c 1 -"
} else {
pipeline += " | aplay -r 22050 -f S16_LE -c 1"
}
cmd = exec.Command("sh", "-c", pipeline)
case "kokoro":
cmd = exec.Command("kokoro", text)
default:
return nil
}

if err := cmd.Start(); err != nil {
return err
}

e.cmd = cmd
e.playing = true

go func() {
_ = cmd.Wait()
e.mu.Lock()
e.playing = false
e.mu.Unlock()
}()

return nil
}

// Stop stops any currently playing speech
func (e *Engine) Stop() {
e.mu.Lock()
defer e.mu.Unlock()
if e.cmd != nil && e.playing {
_ = e.cmd.Process.Kill()
e.playing = false
e.cmd = nil
}
}

// IsPlaying returns true if TTS is currently active
func (e *Engine) IsPlaying() bool {
e.mu.Lock()
defer e.mu.Unlock()
return e.playing
}

func shellQuote(s string) string {
return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
