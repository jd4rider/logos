package tts

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// VoiceEntry describes a single selectable voice.
type VoiceEntry struct {
	Name   string `json:"name"`   // display name shown in picker
	ID     string `json:"id"`     // engine-specific identifier (model path, voice id, etc.)
	Engine string `json:"engine"` // "piper" | "kokoro" | "say"
}

// Engine auto-detects available TTS backends and speaks text asynchronously.
type Engine struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	playerCmd  *exec.Cmd
	playing    bool
	paused     bool
	generation int // incremented on Stop(); used to detect stop between chunks

	activeEngine string
	activeVoice  VoiceEntry
	rate         int

	piperModel   string
	kokoroModel  string
	kokoroVoices string

	cache *AudioCache
}

type Config struct {
	PiperModel string
	Voice      string
	Rate       int
}

func New(piperModel string) *Engine {
	return NewWithConfig(Config{PiperModel: piperModel})
}

func NewWithConfig(cfg Config) *Engine {
	rate := cfg.Rate
	if rate == 0 {
		if v := os.Getenv("SPEECH_RATE"); v != "" {
			if r, err := strconv.Atoi(v); err == nil && r > 0 {
				rate = r
			}
		}
	}
	if rate == 0 {
		rate = 150
	}

	home, _ := os.UserHomeDir()
	e := &Engine{
		rate:         rate,
		piperModel:   cfg.PiperModel,
		kokoroModel:  home + "/.local/share/kokoro/kokoro-v1.0.onnx",
		kokoroVoices: home + "/.local/share/kokoro/voices-v1.0.bin",
		cache:        NewAudioCache(),
	}

	voices := e.ListVoices()
	if len(voices) > 0 {
		e.activeVoice = voices[0]
		e.activeEngine = voices[0].Engine
		if voices[0].Engine == "piper" {
			e.piperModel = voices[0].ID
		}
	} else {
		e.activeEngine = "none"
	}

	if cfg.Voice != "" {
		for _, v := range voices {
			if v.Name == cfg.Voice || v.ID == cfg.Voice {
				e.SetVoiceEntry(v)
				break
			}
		}
	}

	// Pre-warm piper in background so first real call starts quickly
	if e.activeEngine == "piper" {
		go e.Prewarm()
	}

	return e
}

// Prewarm runs a silent synthesis to load the ONNX model into memory.
// Call once at startup; subsequent Speak calls will start ~300ms faster.
func (e *Engine) Prewarm() {
	if e.piperModel == "" {
		return
	}
	cmd := exec.Command("piper", "--output-raw", "--model", e.piperModel)
	cmd.Stdin = strings.NewReader(" ")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Run()
}

// ── Voice listing ─────────────────────────────────────────────────────────────

func (e *Engine) ListVoices() []VoiceEntry {
	var voices []VoiceEntry
	voices = append(voices, e.listPiperVoices()...)
	voices = append(voices, e.listKokoroVoices()...)
	voices = append(voices, e.listSayVoices()...)
	return voices
}

func (e *Engine) listPiperVoices() []VoiceEntry {
	if _, err := exec.LookPath("piper"); err != nil {
		return nil
	}
	home, _ := os.UserHomeDir()
	piperDir := home + "/.local/share/piper"
	entries, err := os.ReadDir(piperDir)
	if err != nil {
		if e.piperModel != "" {
			return []VoiceEntry{{Name: "Piper: " + modelShortName(e.piperModel), ID: e.piperModel, Engine: "piper"}}
		}
		return nil
	}
	var voices []VoiceEntry
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".onnx") {
			path := piperDir + "/" + entry.Name()
			voices = append(voices, VoiceEntry{
				Name:   "Piper: " + strings.TrimSuffix(entry.Name(), ".onnx"),
				ID:     path,
				Engine: "piper",
			})
		}
	}
	return voices
}

func (e *Engine) listKokoroVoices() []VoiceEntry {
	if _, err := exec.LookPath("kokoro-speak"); err != nil {
		return nil
	}
	if _, err := os.Stat(e.kokoroModel); err != nil {
		return nil
	}
	kokoroEnglish := []struct{ id, label string }{
		{"am_michael", "Michael (US Male)"},
		{"am_adam", "Adam (US Male)"},
		{"am_echo", "Echo (US Male)"},
		{"am_eric", "Eric (US Male)"},
		{"am_liam", "Liam (US Male)"},
		{"af_heart", "Heart (US Female)"},
		{"af_bella", "Bella (US Female)"},
		{"af_sarah", "Sarah (US Female)"},
		{"af_nicole", "Nicole (US Female)"},
		{"af_sky", "Sky (US Female)"},
		{"bf_emma", "Emma (UK Female)"},
		{"bf_isabella", "Isabella (UK Female)"},
		{"bm_george", "George (UK Male)"},
		{"bm_lewis", "Lewis (UK Male)"},
	}
	var voices []VoiceEntry
	for _, v := range kokoroEnglish {
		voices = append(voices, VoiceEntry{Name: "Kokoro: " + v.label, ID: v.id, Engine: "kokoro"})
	}
	return voices
}

func (e *Engine) listSayVoices() []VoiceEntry {
	if runtime.GOOS != "darwin" {
		return nil
	}
	out, err := exec.Command("say", "-v", "?").Output()
	if err != nil {
		return nil
	}
	var voices []VoiceEntry
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "en_US") && !strings.Contains(line, "en_GB") {
			continue
		}
		if name := extractVoiceName(line); name != "" {
			voices = append(voices, VoiceEntry{Name: "Say: " + name, ID: name, Engine: "say"})
		}
	}
	return voices
}

func modelShortName(path string) string {
	base := path
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	return strings.TrimSuffix(base, ".onnx")
}

func extractVoiceName(line string) string {
	for _, loc := range []string{"en_US", "en_GB", "en_AU", "en_IE", "en_IN", "en-US"} {
		if idx := strings.Index(line, loc); idx > 0 {
			name := strings.TrimSpace(line[:idx])
			if p := strings.Index(name, "("); p > 0 {
				name = strings.TrimSpace(name[:p])
			}
			return name
		}
	}
	return ""
}

// ── Getters / setters ─────────────────────────────────────────────────────────

func (e *Engine) EngineName() string      { return e.activeEngine }
func (e *Engine) Available() bool         { return e.activeEngine != "none" }
func (e *Engine) ActiveVoice() VoiceEntry { return e.activeVoice }
func (e *Engine) Rate() int               { return e.rate }

func (e *Engine) IsPlaying() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.playing
}

func (e *Engine) SetVoiceEntry(v VoiceEntry) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.activeVoice = v
	e.activeEngine = v.Engine
	if v.Engine == "piper" {
		e.piperModel = v.ID
		go e.Prewarm()
	}
}

func (e *Engine) SetRate(wpm int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rate = wpm
}

// ── Speaking ──────────────────────────────────────────────────────────────────

// Speak starts speaking asynchronously. The returned channel is closed when
// audio actually begins (first byte produced). Use this to sync word highlighting.
func (e *Engine) Speak(text string) (<-chan struct{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()

	started := make(chan struct{})
	var err error
	switch e.activeEngine {
	case "piper":
		err = e.speakPiper(text, started)
	case "kokoro":
		err = e.speakKokoro(text, started)
	case "say":
		err = e.speakSay(text, started)
	default:
		close(started)
		return started, fmt.Errorf("no TTS engine available")
	}
	if err != nil {
		close(started)
		return started, err
	}
	return started, nil
}

// speakPiper: piper → sox pipeline. Closes started when first PCM byte arrives.
func (e *Engine) speakPiper(text string, started chan struct{}) error {
	piperArgs := []string{"--output-raw", "--model", e.piperModel}
	soxArgs := []string{"-t", "raw", "-r", "22050", "-e", "signed", "-b", "16", "-c", "1", "-", "-d"}
	if runtime.GOOS != "darwin" {
		soxArgs = []string{"-t", "raw", "-r", "22050", "-e", "signed", "-b", "16", "-c", "1", "-", "-t", "alsa", "default"}
	}

	piperCmd := exec.Command("piper", piperArgs...)
	piperCmd.Stdin = strings.NewReader(text)

	soxCmd := exec.Command("sox", soxArgs...)

	pipe, err := piperCmd.StdoutPipe()
	if err != nil {
		return err
	}

	// Wrap pipe with a first-byte notifier
	monitoredPipe := &firstByteReader{r: pipe, started: started}
	soxCmd.Stdin = monitoredPipe

	if err := piperCmd.Start(); err != nil {
		return fmt.Errorf("piper: %w", err)
	}
	if err := soxCmd.Start(); err != nil {
		piperCmd.Process.Kill()
		return fmt.Errorf("sox: %w", err)
	}

	e.cmd = piperCmd
	e.playerCmd = soxCmd
	e.playing = true
	go func() {
		piperCmd.Wait()
		soxCmd.Wait()
		e.mu.Lock()
		e.playing = false
		e.cmd = nil
		e.playerCmd = nil
		e.mu.Unlock()
	}()
	return nil
}

// speakKokoro: kokoro-speak → sox pipeline at 24000Hz.
func (e *Engine) speakKokoro(text string, started chan struct{}) error {
	speed := fmt.Sprintf("%.2f", float64(e.rate)/150.0)
	kokoroCmd := exec.Command("kokoro-speak", e.activeVoice.ID, speed)
	kokoroCmd.Stdin = strings.NewReader(text)

	soxArgs := []string{"-t", "raw", "-r", "24000", "-e", "signed", "-b", "16", "-c", "1", "-", "-d"}
	if runtime.GOOS != "darwin" {
		soxArgs = []string{"-t", "raw", "-r", "24000", "-e", "signed", "-b", "16", "-c", "1", "-", "-t", "alsa", "default"}
	}
	soxCmd := exec.Command("sox", soxArgs...)

	pipe, err := kokoroCmd.StdoutPipe()
	if err != nil {
		return err
	}
	soxCmd.Stdin = &firstByteReader{r: pipe, started: started}

	if err := kokoroCmd.Start(); err != nil {
		return fmt.Errorf("kokoro: %w", err)
	}
	if err := soxCmd.Start(); err != nil {
		kokoroCmd.Process.Kill()
		return fmt.Errorf("sox: %w", err)
	}

	e.cmd = kokoroCmd
	e.playerCmd = soxCmd
	e.playing = true
	go func() {
		kokoroCmd.Wait()
		soxCmd.Wait()
		e.mu.Lock()
		e.playing = false
		e.cmd = nil
		e.playerCmd = nil
		e.mu.Unlock()
	}()
	return nil
}

func (e *Engine) speakSay(text string, started chan struct{}) error {
	args := []string{"-r", strconv.Itoa(e.rate)}
	if e.activeVoice.ID != "" {
		args = append(args, "-v", e.activeVoice.ID)
	}
	args = append(args, text)
	cmd := exec.Command("say", args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	e.cmd = cmd
	e.playing = true
	// say starts immediately — signal after a short buffer
	go func() {
		time.Sleep(200 * time.Millisecond)
		close(started)
	}()
	go func() {
		cmd.Wait()
		e.mu.Lock()
		e.playing = false
		e.cmd = nil
		e.mu.Unlock()
	}()
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()
}

// Pause suspends audio playback without stopping the current session.
// The word-advance ticker must also be halted by the caller (set ttsPaused).
func (e *Engine) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.playing && !e.paused && e.playerCmd != nil && e.playerCmd.Process != nil {
		e.playerCmd.Process.Signal(syscall.SIGSTOP) //nolint:errcheck
		e.paused = true
	}
}

// Resume continues playback after a Pause.
func (e *Engine) Resume() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.paused && e.playerCmd != nil && e.playerCmd.Process != nil {
		e.playerCmd.Process.Signal(syscall.SIGCONT) //nolint:errcheck
		e.paused = false
	}
}

// IsPaused reports whether playback is currently suspended.
func (e *Engine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.paused
}

func (e *Engine) stopLocked() {
	e.generation++
	if e.playerCmd != nil && e.playerCmd.Process != nil {
		e.playerCmd.Process.Kill()
	}
	e.playerCmd = nil
	if e.cmd != nil && e.cmd.Process != nil {
		e.cmd.Process.Kill()
	}
	e.cmd = nil
	e.playing = false
	e.paused = false
}

// firstByteReader wraps an io.Reader and closes 'started' once the first byte is read.
type firstByteReader struct {
	r       io.Reader
	started chan struct{}
	once    sync.Once
}

func (f *firstByteReader) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if n > 0 {
		f.once.Do(func() { close(f.started) })
	}
	return n, err
}

// SyncedSpeech holds the result of a pre-synthesized speech run.
type SyncedSpeech struct {
	WordDurations []time.Duration // one per word, calibrated to actual audio length
	Started       <-chan struct{} // closed when audio playback begins
}

// SpeakSynced pre-synthesizes the text to measure its exact audio duration,
// then plays it back while returning per-word durations that are scaled to the
// real audio length. This gives accurate word-highlight synchronisation.
//
// For say/kokoro it falls back to estimated durations (no pre-synthesis needed).
func (e *Engine) SpeakSynced(text string, words []string) (*SyncedSpeech, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopLocked()

	started := make(chan struct{})

	switch e.activeEngine {
	case "piper":
		durations, err := e.speakPiperSynced(text, words, started)
		if err != nil {
			close(started)
			return nil, err
		}
		return &SyncedSpeech{WordDurations: durations, Started: started}, nil

	case "kokoro":
		durations, err := e.speakKokoroSynced(text, words, started)
		if err != nil {
			close(started)
			return nil, err
		}
		return &SyncedSpeech{WordDurations: durations, Started: started}, nil

	case "say":
		// say can't be pre-synthesized easily; use estimated durations
		err := e.speakSay(text, started)
		if err != nil {
			close(started)
			return nil, err
		}
		durations := make([]time.Duration, len(words))
		for i, w := range words {
			durations[i] = estimateWordDuration(w)
		}
		return &SyncedSpeech{WordDurations: durations, Started: started}, nil

	default:
		close(started)
		return nil, fmt.Errorf("no TTS engine available")
	}
}

// speakPiperSynced: plays piper audio with word-sync durations.
//
// Cache hit  → plays pre-synthesized PCM with exact per-word timing.
// Cache miss → streams piper→sox immediately (audio starts in ~2 s) and uses
//
//	estimated durations, while a background goroutine synthesizes
//	and caches the full PCM for accurate timing on next play.
func (e *Engine) speakPiperSynced(text string, words []string, started chan struct{}) ([]time.Duration, error) {
	const sampleRate = 22050

	// Cache hit: exact timing + instant playback from buffer.
	cacheKey := CacheKey("piper", e.activeVoice.ID, e.rate, text)
	if pcm, meta, ok := e.cache.Get(cacheKey); ok {
		durations := ComputeSyncedDurations(words, time.Duration(meta.DurationMs)*time.Millisecond)
		return durations, e.playPCMBackground(pcm, "22050", started)
	}

	// Cache miss: stream immediately so audio starts without waiting for full synthesis.
	if err := e.speakPiper(text, started); err != nil {
		return nil, err
	}
	durations := make([]time.Duration, len(words))
	for i, w := range words {
		durations[i] = estimateWordDuration(w)
	}

	// Background: synthesize full PCM and cache it for accurate timing next time.
	model := e.piperModel
	voiceID := e.activeVoice.ID
	voiceName := e.activeVoice.Name
	rate := e.rate
	cache := e.cache
	go func() {
		cmd := exec.Command("piper", "--output-raw", "--model", model)
		cmd.Stdin = strings.NewReader(text)
		pcm, err := cmd.Output()
		if err != nil || len(pcm) == 0 {
			return
		}
		totalSamples := len(pcm) / 2
		dur := time.Duration(totalSamples) * time.Second / sampleRate
		preview := text
		if len(preview) > 80 {
			preview = preview[:80]
		}
		cache.Put(cacheKey, pcm, CacheMeta{
			Hash:        cacheKey,
			Engine:      "piper",
			VoiceID:     voiceID,
			VoiceName:   voiceName,
			Rate:        rate,
			SampleRate:  sampleRate,
			TextPreview: preview,
			TextLen:     len(text),
			WordCount:   len(words),
			DurationMs:  dur.Milliseconds(),
			PCMBytes:    len(pcm),
			CreatedAt:   time.Now(),
			LastAccess:  time.Now(),
		})
	}()

	return durations, nil
}

// speakKokoroSynced: pre-synthesizes kokoro output (or loads from cache), measures duration, plays.
func (e *Engine) speakKokoroSynced(text string, words []string, started chan struct{}) ([]time.Duration, error) {
	const sampleRate = 24000
	speed := fmt.Sprintf("%.2f", float64(e.rate)/150.0)

	// Check cache first (rate affects speed, so include it in key)
	cacheKey := CacheKey("kokoro", e.activeVoice.ID, e.rate, text)
	if pcm, meta, ok := e.cache.Get(cacheKey); ok {
		durations := ComputeSyncedDurations(words, time.Duration(meta.DurationMs)*time.Millisecond)
		return durations, e.playPCMBackground(pcm, "24000", started)
	}

	// Synthesize
	kokoroSynth := exec.Command("kokoro-speak", e.activeVoice.ID, speed)
	kokoroSynth.Stdin = strings.NewReader(text)
	var kokoroStderr bytes.Buffer
	kokoroSynth.Stderr = &kokoroStderr

	pcm, err := kokoroSynth.Output()
	if err != nil {
		detail := strings.TrimSpace(kokoroStderr.String())
		if detail != "" {
			return nil, fmt.Errorf("kokoro synthesis: %w\n%s", err, detail)
		}
		return nil, fmt.Errorf("kokoro synthesis: %w", err)
	}
	if len(pcm) == 0 {
		return nil, fmt.Errorf("kokoro produced no audio")
	}

	totalSamples := len(pcm) / 2
	actualDuration := time.Duration(totalSamples) * time.Second / sampleRate

	preview := text
	if len(preview) > 80 {
		preview = preview[:80]
	}
	e.cache.Put(cacheKey, pcm, CacheMeta{
		Hash:        cacheKey,
		Engine:      "kokoro",
		VoiceID:     e.activeVoice.ID,
		VoiceName:   e.activeVoice.Name,
		Rate:        e.rate,
		SampleRate:  sampleRate,
		TextPreview: preview,
		TextLen:     len(text),
		WordCount:   len(words),
		DurationMs:  actualDuration.Milliseconds(),
		PCMBytes:    len(pcm),
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 0,
	})

	durations := ComputeSyncedDurations(words, actualDuration)
	return durations, e.playPCMBackground(pcm, "24000", started)
}

// playPCMBackground starts sox to play raw PCM in the background.
// rate is the sample rate string (e.g. "22050").
func (e *Engine) playPCMBackground(pcm []byte, rate string, started chan struct{}) error {
	soxArgs := []string{"-t", "raw", "-r", rate, "-e", "signed", "-b", "16", "-c", "1", "-", "-d"}
	if runtime.GOOS != "darwin" {
		soxArgs = []string{"-t", "raw", "-r", rate, "-e", "signed", "-b", "16", "-c", "1", "-", "-t", "alsa", "default"}
	}
	soxCmd := exec.Command("sox", soxArgs...)
	soxCmd.Stdin = &firstByteReader{r: strings.NewReader(string(pcm)), started: started}

	if err := soxCmd.Start(); err != nil {
		return fmt.Errorf("sox playback: %w", err)
	}

	e.playerCmd = soxCmd
	e.playing = true
	go func() {
		soxCmd.Wait()
		e.mu.Lock()
		e.playing = false
		e.playerCmd = nil
		e.mu.Unlock()
	}()
	return nil
}

// playPCMSync plays PCM data synchronously, blocking until sox finishes or is killed.
// started is closed when the first audio byte is read by sox.
// Returns nil on natural completion, non-nil if stopped externally.
func (e *Engine) playPCMSync(pcm []byte, rate string, started chan struct{}, gen int) error {
	soxArgs := []string{"-t", "raw", "-r", rate, "-e", "signed", "-b", "16", "-c", "1", "-", "-d"}
	if runtime.GOOS != "darwin" {
		soxArgs = []string{"-t", "raw", "-r", rate, "-e", "signed", "-b", "16", "-c", "1", "-", "-t", "alsa", "default"}
	}
	soxCmd := exec.Command("sox", soxArgs...)
	soxCmd.Stdin = &firstByteReader{r: bytes.NewReader(pcm), started: started}

	if err := soxCmd.Start(); err != nil {
		return fmt.Errorf("sox: %w", err)
	}

	e.mu.Lock()
	e.playerCmd = soxCmd
	e.playing = true
	e.mu.Unlock()

	soxCmd.Wait() //nolint:errcheck

	e.mu.Lock()
	curGen := e.generation
	e.playing = false
	e.playerCmd = nil
	e.mu.Unlock()

	if curGen != gen {
		return fmt.Errorf("stopped")
	}
	return nil
}

// getSentencePCM synthesizes a sentence and returns (pcm, sampleRateStr, durations, err).
// Uses cache first. Does NOT hold the engine mutex during synthesis.
func (e *Engine) getSentencePCM(s Sentence) ([]byte, string, []time.Duration, error) {
	e.mu.Lock()
	eng := e.activeEngine
	voiceID := e.activeVoice.ID
	voiceName := e.activeVoice.Name
	rate := e.rate
	piperModel := e.piperModel
	e.mu.Unlock()

	switch eng {
	case "piper":
		return e.getSentencePCMPiper(s, voiceID, voiceName, rate, piperModel)
	case "kokoro":
		return e.getSentencePCMKokoro(s, voiceID, voiceName, rate)
	case "say":
		durations := make([]time.Duration, len(s.Words))
		for i, w := range s.Words {
			durations[i] = estimateWordDuration(w)
		}
		return nil, "say", durations, nil
	default:
		return nil, "", nil, fmt.Errorf("no TTS engine available")
	}
}

func (e *Engine) getSentencePCMPiper(s Sentence, voiceID, voiceName string, rate int, model string) ([]byte, string, []time.Duration, error) {
	const sampleRate = 22050
	cacheKey := CacheKey("piper", voiceID, rate, s.Text)
	if pcm, meta, ok := e.cache.Get(cacheKey); ok {
		durations := ComputeSyncedDurations(s.Words, time.Duration(meta.DurationMs)*time.Millisecond)
		return pcm, "22050", durations, nil
	}

	cmd := exec.Command("piper", "--output-raw", "--model", model)
	cmd.Stdin = strings.NewReader(s.Text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pcm, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, "", nil, fmt.Errorf("piper: %w\n%s", err, detail)
		}
		return nil, "", nil, fmt.Errorf("piper: %w", err)
	}
	if len(pcm) == 0 {
		return nil, "", nil, fmt.Errorf("piper produced no audio")
	}
	dur := time.Duration(len(pcm)/2) * time.Second / sampleRate
	preview := s.Text
	if len(preview) > 80 {
		preview = preview[:80]
	}
	e.cache.Put(cacheKey, pcm, CacheMeta{
		Hash: cacheKey, Engine: "piper",
		VoiceID: voiceID, VoiceName: voiceName,
		Rate: rate, SampleRate: sampleRate,
		TextPreview: preview, TextLen: len(s.Text),
		WordCount: len(s.Words), DurationMs: dur.Milliseconds(),
		PCMBytes: len(pcm), CreatedAt: time.Now(), LastAccess: time.Now(),
	})
	return pcm, "22050", ComputeSyncedDurations(s.Words, dur), nil
}

func (e *Engine) getSentencePCMKokoro(s Sentence, voiceID, voiceName string, rate int) ([]byte, string, []time.Duration, error) {
	const sampleRate = 24000
	speed := fmt.Sprintf("%.2f", float64(rate)/150.0)
	cacheKey := CacheKey("kokoro", voiceID, rate, s.Text)
	if pcm, meta, ok := e.cache.Get(cacheKey); ok {
		durations := ComputeSyncedDurations(s.Words, time.Duration(meta.DurationMs)*time.Millisecond)
		return pcm, "24000", durations, nil
	}

	cmd := exec.Command("kokoro-speak", voiceID, speed)
	cmd.Stdin = strings.NewReader(s.Text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pcm, err := cmd.Output()
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, "", nil, fmt.Errorf("kokoro: %w\n%s", err, detail)
		}
		return nil, "", nil, fmt.Errorf("kokoro: %w", err)
	}
	if len(pcm) == 0 {
		return nil, "", nil, fmt.Errorf("kokoro produced no audio")
	}
	dur := time.Duration(len(pcm)/2) * time.Second / sampleRate
	preview := s.Text
	if len(preview) > 80 {
		preview = preview[:80]
	}
	e.cache.Put(cacheKey, pcm, CacheMeta{
		Hash: cacheKey, Engine: "kokoro",
		VoiceID: voiceID, VoiceName: voiceName,
		Rate: rate, SampleRate: sampleRate,
		TextPreview: preview, TextLen: len(s.Text),
		WordCount: len(s.Words), DurationMs: dur.Milliseconds(),
		PCMBytes: len(pcm), CreatedAt: time.Now(), LastAccess: time.Now(),
	})
	return pcm, "24000", ComputeSyncedDurations(s.Words, dur), nil
}

// PlayChunked plays sentences sequentially, calling onChunk(wordOffset, durations)
// when each sentence's audio begins. startWordOffset skips sentences before that word.
// Returns nil when all sentences finish or playback is stopped.
func (e *Engine) PlayChunked(sentences []Sentence, startWordOffset int, onChunk func(wordOffset int, durations []time.Duration)) error {
	e.mu.Lock()
	e.stopLocked()
	gen := e.generation
	e.mu.Unlock()

	wordOffset := 0

	for _, s := range sentences {
		endOffset := wordOffset + len(s.Words)

		if endOffset <= startWordOffset {
			wordOffset = endOffset
			continue
		}

		e.mu.Lock()
		curGen := e.generation
		e.mu.Unlock()
		if curGen != gen {
			return nil
		}

		pcm, sampleRate, durations, err := e.getSentencePCM(s)
		if err != nil {
			return err
		}

		highlightOffset := wordOffset
		if wordOffset < startWordOffset && startWordOffset < endOffset {
			skip := startWordOffset - wordOffset
			if skip > 0 && skip < len(durations) {
				durations = durations[skip:]
			}
			highlightOffset = startWordOffset
		}

		if sampleRate == "say" {
			onChunk(highlightOffset, durations)
			e.mu.Lock()
			args := []string{"-r", fmt.Sprintf("%d", e.rate)}
			if e.activeVoice.ID != "" {
				args = append(args, "-v", e.activeVoice.ID)
			}
			args = append(args, s.Text)
			cmd := exec.Command("say", args...)
			e.cmd = cmd
			e.mu.Unlock()
			cmd.Start() //nolint:errcheck
			cmd.Wait()  //nolint:errcheck
			e.mu.Lock()
			curGen2 := e.generation
			e.cmd = nil
			e.mu.Unlock()
			if curGen2 != gen {
				return nil
			}
		} else {
			started := make(chan struct{})
			go func(wo int, d []time.Duration) {
				<-started
				onChunk(wo, d)
			}(highlightOffset, durations)

			if err := e.playPCMSync(pcm, sampleRate, started, gen); err != nil {
				return nil
			}
		}

		wordOffset = endOffset
		startWordOffset = 0
	}
	return nil
}

// PrecacheSentences synthesises all sentences for the given text and stores them
// in the audio cache without starting playback.
func (e *Engine) PrecacheSentences(cleanedText string) error {
	sentences := SplitSentences(cleanedText, 50)
	for _, s := range sentences {
		e.mu.Lock()
		eng := e.activeEngine
		e.mu.Unlock()
		if eng == "say" || eng == "" {
			return nil
		}
		_, _, _, err := e.getSentencePCM(s)
		if err != nil {
			return err
		}
	}
	return nil
}

// Cache returns the engine's audio cache (for CLI stats/management).
func (e *Engine) Cache() *AudioCache { return e.cache }

// SpeakSyncedFrom is like SpeakSynced but seeks to startWord in the cached PCM.
//
// Cache hit:  trims the PCM buffer to the byte offset for startWord, so audio
//
//	starts immediately at that word with no silence preamble, and
//	returns word durations beginning at startWord.
//
// Cache miss: falls back to SpeakSynced from the beginning; actualStart=0.
func (e *Engine) SpeakSyncedFrom(text string, words []string, startWord int) (synced *SyncedSpeech, actualStart int, err error) {
	if startWord <= 0 {
		s, err := e.SpeakSynced(text, words)
		return s, 0, err
	}

	// Read engine params under a brief lock.
	e.mu.Lock()
	eng := e.activeEngine
	voiceID := e.activeVoice.ID
	rate := e.rate
	e.mu.Unlock()

	var cacheKey string
	var sampleRate int64
	switch eng {
	case "piper":
		cacheKey = CacheKey("piper", voiceID, rate, text)
		sampleRate = 22050
	case "kokoro":
		cacheKey = CacheKey("kokoro", voiceID, rate, text)
		sampleRate = 24000
	default:
		// say or unknown: no PCM cache, fall back to full synthesis from start.
		s, err := e.SpeakSynced(text, words)
		return s, 0, err
	}

	pcm, meta, ok := e.cache.Get(cacheKey)
	if !ok || startWord >= len(words) {
		s, err := e.SpeakSynced(text, words)
		return s, 0, err
	}

	allDurations := ComputeSyncedDurations(words, time.Duration(meta.DurationMs)*time.Millisecond)
	if startWord >= len(allDurations) {
		s, err := e.SpeakSynced(text, words)
		return s, 0, err
	}

	// Calculate byte offset for startWord (16-bit mono PCM).
	var offsetMs int64
	for i := 0; i < startWord; i++ {
		offsetMs += allDurations[i].Milliseconds()
	}
	offsetSamples := offsetMs * sampleRate / 1000
	offsetBytes := int(offsetSamples * 2) // 16-bit = 2 bytes/sample
	offsetBytes &^= 1                     // align to sample boundary
	if offsetBytes < 0 || offsetBytes >= len(pcm) {
		offsetBytes = 0
	}

	e.mu.Lock()
	e.stopLocked()
	started := make(chan struct{})
	rateStr := fmt.Sprintf("%d", sampleRate)
	playErr := e.playPCMBackground(pcm[offsetBytes:], rateStr, started)
	e.mu.Unlock()
	if playErr != nil {
		return nil, 0, playErr
	}
	return &SyncedSpeech{
		WordDurations: allDurations[startWord:],
		Started:       started,
	}, startWord, nil
}

// Precache synthesises text and stores the result in the audio cache WITHOUT
// starting playback.  It is safe to call from a background goroutine while
// playback is active — it snapshots the current voice/rate under a short lock,
// then runs the (potentially slow) synthesis subprocess WITHOUT holding the
// mutex, so it never blocks the main TTS path.
//
// Returns nil if the text is already cached (nothing to do) or on success.
func (e *Engine) Precache(text string, words []string) error {
	// Snapshot engine settings under a brief lock so we don't block playback.
	e.mu.Lock()
	eng := e.activeEngine
	voiceID := e.activeVoice.ID
	voiceName := e.activeVoice.Name
	rate := e.rate
	piperModel := e.piperModel
	kokoroVoices := e.kokoroVoices
	e.mu.Unlock()

	if eng == "none" || eng == "" || eng == "say" {
		// 'say' can't be pre-synthesised; skip.
		return nil
	}

	switch eng {
	case "piper":
		return e.precachePiper(text, words, voiceID, voiceName, rate, piperModel)
	case "kokoro":
		return e.precacheKokoro(text, words, voiceID, voiceName, rate, kokoroVoices)
	}
	return nil
}

func (e *Engine) precachePiper(text string, words []string, voiceID, voiceName string, rate int, model string) error {
	const sampleRate = 22050
	cacheKey := CacheKey("piper", voiceID, rate, text)
	if _, _, ok := e.cache.Get(cacheKey); ok {
		return nil // already cached
	}
	synth := exec.Command("piper", "--output-raw", "--model", model)
	synth.Stdin = strings.NewReader(text)
	var synthErr bytes.Buffer
	synth.Stderr = &synthErr
	pcm, err := synth.Output()
	if err != nil {
		detail := strings.TrimSpace(synthErr.String())
		if detail != "" {
			return fmt.Errorf("piper precache: %w\n%s", err, detail)
		}
		return fmt.Errorf("piper precache: %w", err)
	}
	if len(pcm) == 0 {
		return nil
	}
	dur := time.Duration(len(pcm)/2) * time.Second / sampleRate
	preview := text
	if len(preview) > 80 {
		preview = preview[:80]
	}
	e.cache.Put(cacheKey, pcm, CacheMeta{
		Hash:        cacheKey,
		Engine:      "piper",
		VoiceID:     voiceID,
		VoiceName:   voiceName,
		Rate:        rate,
		SampleRate:  sampleRate,
		TextPreview: preview,
		TextLen:     len(text),
		WordCount:   len(words),
		DurationMs:  dur.Milliseconds(),
		PCMBytes:    len(pcm),
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 0,
	})
	return nil
}

func (e *Engine) precacheKokoro(text string, words []string, voiceID, voiceName string, rate int, _ string) error {
	const sampleRate = 24000
	speed := fmt.Sprintf("%.2f", float64(rate)/150.0)
	cacheKey := CacheKey("kokoro", voiceID, rate, text)
	if _, _, ok := e.cache.Get(cacheKey); ok {
		return nil // already cached
	}
	synth := exec.Command("kokoro-speak", voiceID, speed)
	synth.Stdin = strings.NewReader(text)
	var synthErr bytes.Buffer
	synth.Stderr = &synthErr
	pcm, err := synth.Output()
	if err != nil {
		detail := strings.TrimSpace(synthErr.String())
		if detail != "" {
			return fmt.Errorf("kokoro precache: %w\n%s", err, detail)
		}
		return fmt.Errorf("kokoro precache: %w", err)
	}
	if len(pcm) == 0 {
		return nil
	}
	dur := time.Duration(len(pcm)/2) * time.Second / sampleRate
	preview := text
	if len(preview) > 80 {
		preview = preview[:80]
	}
	e.cache.Put(cacheKey, pcm, CacheMeta{
		Hash:        cacheKey,
		Engine:      "kokoro",
		VoiceID:     voiceID,
		VoiceName:   voiceName,
		Rate:        rate,
		SampleRate:  sampleRate,
		TextPreview: preview,
		TextLen:     len(text),
		WordCount:   len(words),
		DurationMs:  dur.Milliseconds(),
		PCMBytes:    len(pcm),
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 0,
	})
	return nil
}
