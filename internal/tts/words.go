package tts

import (
"errors"
"regexp"
"strings"
"time"
"unicode"

tea "github.com/charmbracelet/bubbletea"
)

// WordAdvanceMsg is sent to advance the highlighted word.
type WordAdvanceMsg struct{ Index int }

// TTSStartedMsg is sent when the TTS engine begins producing audio.
type TTSStartedMsg struct{}

var (
verseNumRe = regexp.MustCompile(`\[(\d+)\]`)
pilcrowRe  = regexp.MustCompile(`¶\s*`)
extraSpace = regexp.MustCompile(`\s{2,}`)
)

// CleanForTTS converts raw chapter content to spoken prose.
// Verse numbers are replaced with a brief pause marker ("...") so the TTS
// engine pauses naturally at verse boundaries without announcing numbers.
// The first verse marker ([1]) is dropped so reading begins immediately.
func CleanForTTS(content string) string {
first := true
s := verseNumRe.ReplaceAllStringFunc(content, func(m string) string {
if first {
first = false
return "  "
}
return "...  "
})
s = pilcrowRe.ReplaceAllString(s, "  ")
s = extraSpace.ReplaceAllString(s, "  ")
return strings.TrimSpace(s)
}

// SplitWords splits cleaned TTS text into individual speakable tokens.
func SplitWords(text string) []string {
raw := strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) })
words := make([]string, 0, len(raw))
for _, w := range raw {
if w != "" {
words = append(words, w)
}
}
return words
}

// WaitForTTSStart blocks until the engine signals audio has begun, then sends TTSStartedMsg.
func WaitForTTSStart(started <-chan struct{}) tea.Cmd {
return func() tea.Msg {
<-started
return TTSStartedMsg{}
}
}

// ComputeSyncedDurations scales per-word timing estimates so they sum to the
// actual audio duration measured from synthesized PCM.
//
// Verse announcement tokens ("Verse" and "N.") are given a heavily weighted
// budget so they receive a proportionally large share of the real audio time —
// TTS engines spend extra time on sentence-boundary pauses around verse numbers.
//
// A two-pass approach avoids floor-drift: we first apply the minimum floor,
// redistribute the remaining time proportionally among non-floored words.
func ComputeSyncedDurations(words []string, actualDuration time.Duration) []time.Duration {
durations := make([]time.Duration, len(words))
if len(words) == 0 {
return durations
}

const minDur = 60 * time.Millisecond

estimated := make([]time.Duration, len(words))
var totalEstimated time.Duration
for i, w := range words {
d := estimateWordDuration(w)
estimated[i] = d
totalEstimated += d
}

if totalEstimated == 0 {
flat := actualDuration / time.Duration(len(words))
for i := range durations {
durations[i] = flat
}
return durations
}

scale := float64(actualDuration) / float64(totalEstimated)

// Pass 1: apply scale, floor, then track leftover budget for redistribution.
var floored time.Duration
var scaledTotal time.Duration
for i, d := range estimated {
scaled := time.Duration(float64(d) * scale)
durations[i] = scaled
scaledTotal += scaled
}

// Pass 2: clamp minimums and track how much was "added" by flooring.
for i, d := range durations {
if d < minDur {
floored += minDur - d
durations[i] = minDur
}
}

// Redistribute the deficit (floored time) proportionally from non-floored words.
if floored > 0 {
var reducible time.Duration
for i, d := range durations {
if d > minDur*2 {
reducible += d - minDur
_ = estimated[i]
}
}
if reducible > 0 {
for i, d := range durations {
if d > minDur*2 {
take := time.Duration(float64(floored) * float64(d-minDur) / float64(reducible))
durations[i] -= take
}
}
}
}

return durations
}

// SyncedWordTickCmd fires WordAdvanceMsg{idx} after the measured duration for
// words[idx-1]. Uses pre-computed per-word durations from ComputeSyncedDurations.
func SyncedWordTickCmd(durations []time.Duration, idx int) tea.Cmd {
if idx <= 0 || idx > len(durations) {
return nil
}
d := durations[idx-1]
capture := idx
return tea.Tick(d, func(time.Time) tea.Msg {
return WordAdvanceMsg{Index: capture}
})
}

// WordTickCmd is the fallback estimator (used for say/kokoro when no pre-synthesis).
func WordTickCmd(words []string, idx int) tea.Cmd {
if idx <= 0 || idx > len(words) {
return nil
}
d := estimateWordDuration(words[idx-1])
capture := idx
return tea.Tick(d, func(time.Time) tea.Msg {
return WordAdvanceMsg{Index: capture}
})
}

func estimateWordDuration(word string) time.Duration {
const baseMs = 400 // 150 WPM baseline (~5-char word)

// ── Pause token (verse boundary) ─────────────────────────────────────────
// "..." is inserted between verses by CleanForTTS; give it a generous budget
// so the TTS pause is clearly audible between verses.
if word == "..." {
return 700 * time.Millisecond
}

// ── Normal words ───────────────────────────────────────────────────────────
core := strings.TrimRight(word, ".,;:!?\"'")
chars := len(core)
if chars < 1 {
chars = 1
}

scale := float64(chars) / 5.0
if scale < 0.4 {
scale = 0.4
}
if scale > 2.2 {
scale = 2.2
}
ms := float64(baseMs) * scale

last := word[len(word)-1]
switch last {
case '.', '!', '?':
ms += 350
case ',':
ms += 120
case ';', ':':
ms += 180
}

if strings.HasSuffix(word, "...") {
ms += 450
}

d := time.Duration(ms) * time.Millisecond
if d < 80*time.Millisecond {
d = 80 * time.Millisecond
}
if d > 1200*time.Millisecond {
d = 1200 * time.Millisecond
}
return d
}

func parseDigits(s string) (int, error) {
if s == "" {
return 0, errors.New("empty")
}
n := 0
for _, c := range s {
if c < '0' || c > '9' {
return 0, errors.New("not digits")
}
n = n*10 + int(c-'0')
}
return n, nil
}

// StripVerseMarkers removes [N] and ¶ from display text.
func StripVerseMarkers(s string) string {
s = verseNumRe.ReplaceAllString(s, "")
s = pilcrowRe.ReplaceAllString(s, "")
return s
}
