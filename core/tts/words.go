package tts

import (
"regexp"
"strings"
"time"
"unicode"
)

var verseMarkerRe = regexp.MustCompile(`\[\d+\]`)

// StripVerseMarkers removes [N] verse markers and pilcrow signs from text
func StripVerseMarkers(s string) string {
s = verseMarkerRe.ReplaceAllString(s, "")
s = strings.ReplaceAll(s, "¶", "")
return s
}

// CleanForTTS converts raw chapter content to plain text suitable for TTS
func CleanForTTS(content string) string {
content = verseMarkerRe.ReplaceAllStringFunc(content, func(m string) string {
num := m[1 : len(m)-1]
return "verse " + num + ":"
})
content = strings.ReplaceAll(content, "¶", "")
content = strings.Join(strings.Fields(content), " ")
return content
}

// SplitWords splits cleaned text into individual words
func SplitWords(text string) []string {
return strings.FieldsFunc(text, func(r rune) bool {
return unicode.IsSpace(r)
})
}

// WordDuration returns the estimated speaking duration for a given word
func WordDuration(word string) time.Duration {
const base = 353 * time.Millisecond
scale := float64(len(word)) / 5.0
if scale < 0.34 {
scale = 0.34
}
if scale > 2.27 {
scale = 2.27
}
return time.Duration(float64(base) * scale)
}
