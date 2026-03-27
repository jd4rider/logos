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
// Gen must match the current ttsGen in the model; stale messages are discarded.
type WordAdvanceMsg struct {
	Index int
	Gen   int
}

// TTSStartedMsg is sent when the TTS engine begins producing audio.
// Gen must match the current ttsGen in the model.
type TTSStartedMsg struct{ Gen int }

var (
	verseNumRe = regexp.MustCompile(`\[(\d+)\]`)
	pilcrowRe  = regexp.MustCompile(`¶\s*`)
	extraSpace = regexp.MustCompile(`\s{2,}`)

	// Markdown patterns
	mdFencedCode = regexp.MustCompile("(?s)```[^`]*```")          // ```code blocks```
	mdInlineCode = regexp.MustCompile("`[^`]+`")                  // `inline code`
	mdHeading    = regexp.MustCompile(`(?m)^#{1,6}\s+`)           // # ## ### headings
	mdHRule      = regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`)       // --- *** ___ hr
	mdBoldItalic = regexp.MustCompile(`\*{1,3}([^*\n]+)\*{1,3}`)  // *italic* **bold** ***both***
	mdUnderline  = regexp.MustCompile(`_{1,2}([^_\n]+)_{1,2}`)    // _italic_ __bold__
	mdStrike     = regexp.MustCompile(`~~([^~]+)~~`)              // ~~strikethrough~~
	mdLink       = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)  // [text](url) and ![alt](url)
	mdLinkRef    = regexp.MustCompile(`!?\[([^\]]*)\]\[[^\]]*\]`) // [text][ref]
	mdBlockquote = regexp.MustCompile(`(?m)^>\s?`)                // > blockquote
	mdListBullet = regexp.MustCompile(`(?m)^\s*[-*+]\s+`)         // - * + list items
	mdListNum    = regexp.MustCompile(`(?m)^\s*\d+\.\s+`)         // 1. numbered list
	mdHTMLTag    = regexp.MustCompile(`<[^>]+>`)                  // <html tags>

	// HTML entities
	mdHTMLEntities = map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&#39;":    "'",
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&mdash;":  "—",
		"&ndash;":  "–",
		"&ldquo;":  "\"",
		"&rdquo;":  "\"",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
		"&hellip;": "...",
		"&bull;":   "",
		"&copy;":   "",
		"&reg;":    "",
		"&trade;":  "",
	}
)

// stripMarkdown removes markdown and HTML formatting from text, leaving only
// the natural prose that should be spoken aloud.
func stripMarkdown(s string) string {
	// Remove fenced code blocks entirely — they're not meant to be read aloud
	s = mdFencedCode.ReplaceAllString(s, " ")
	// Inline code: keep the content but strip backticks
	s = mdInlineCode.ReplaceAllStringFunc(s, func(m string) string {
		return m[1 : len(m)-1]
	})
	// Headings: strip the # markers
	s = mdHeading.ReplaceAllString(s, "")
	// Horizontal rules: replace with pause
	s = mdHRule.ReplaceAllString(s, "  ")
	// Bold/italic: keep inner text
	s = mdBoldItalic.ReplaceAllString(s, "$1")
	s = mdUnderline.ReplaceAllString(s, "$1")
	s = mdStrike.ReplaceAllString(s, "$1")
	// Links: keep link text, drop URL
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdLinkRef.ReplaceAllString(s, "$1")
	// Blockquotes: strip the > prefix
	s = mdBlockquote.ReplaceAllString(s, "")
	// List markers: strip - * + 1. etc.
	s = mdListBullet.ReplaceAllString(s, "")
	s = mdListNum.ReplaceAllString(s, "")
	// Strip HTML tags
	s = mdHTMLTag.ReplaceAllString(s, " ")
	// HTML entities
	for entity, replacement := range mdHTMLEntities {
		s = strings.ReplaceAll(s, entity, replacement)
	}
	// Any remaining stray backticks, ~~ remnants, bare asterisks/underscores
	// that weren't part of balanced pairs — strip them so TTS doesn't say the symbol name.
	// We do this carefully: only strip runs of 1–3 of these that are surrounded by
	// spaces or at line boundaries, not mid-word punctuation like "don't" or "it's".
	s = regexp.MustCompile(`(?:^|\s)[*_~]{1,3}(?:\s|$)`).ReplaceAllStringFunc(s, func(m string) string {
		// Preserve surrounding spaces, drop the symbol
		return strings.Map(func(r rune) rune {
			if r == '*' || r == '_' || r == '~' {
				return -1
			}
			return r
		}, m)
	})
	return s
}

// CleanForTTS converts raw chapter content to spoken prose.
// Strips markdown/HTML formatting first, then replaces verse markers with
// brief pause markers ("...") so the TTS engine pauses naturally at verse
// boundaries without announcing numbers.
// The first verse marker ([1]) is dropped so reading begins immediately.
func CleanForTTS(content string) string {
	s := stripMarkdown(content)
	first := true
	s = verseNumRe.ReplaceAllStringFunc(s, func(m string) string {
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
// Pause-marker tokens ("...") are excluded from the word list: they exist in
// the TTS text so the engine creates a natural verse-break pause, but they are
// not real words and should not appear in the highlight index.
func SplitWords(text string) []string {
	raw := strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) })
	words := make([]string, 0, len(raw))
	for _, w := range raw {
		if w != "" && w != "..." {
			words = append(words, w)
		}
	}
	return words
}

// WaitForTTSStart blocks until the engine signals audio has begun, then sends TTSStartedMsg.
// gen is the current TTS session generation; it is echoed back so stale sessions can be discarded.
func WaitForTTSStart(started <-chan struct{}, gen int) tea.Cmd {
	return func() tea.Msg {
		<-started
		return TTSStartedMsg{Gen: gen}
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

// SyncedWordTickCmd fires WordAdvanceMsg{idx, gen} after the measured duration for
// words[idx-1].  gen is the current TTS session generation; stale messages are
// automatically discarded by the model's WordAdvanceMsg handler.
func SyncedWordTickCmd(durations []time.Duration, idx int, gen int) tea.Cmd {
	if idx <= 0 || idx > len(durations) {
		return nil
	}
	d := durations[idx-1]
	capture := idx
	return tea.Tick(d, func(time.Time) tea.Msg {
		return WordAdvanceMsg{Index: capture, Gen: gen}
	})
}

// WordTickCmd is the fallback estimator (used for say/kokoro when no pre-synthesis).
func WordTickCmd(words []string, idx int, gen int) tea.Cmd {
	if idx <= 0 || idx > len(words) {
		return nil
	}
	d := estimateWordDuration(words[idx-1])
	capture := idx
	return tea.Tick(d, func(time.Time) tea.Msg {
		return WordAdvanceMsg{Index: capture, Gen: gen}
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

// Sentence is a single chunk of cleaned TTS text ready for synthesis.
type Sentence struct {
	Text  string   // full text for TTS engine (includes pause markers)
	Words []string // highlight words only (no "..." markers)
}

// SplitSentences breaks cleaned TTS text into sentence-sized chunks.
// Sentences end at natural boundaries (. ! ? or ...) with at least minWords
// words, or when they reach maxWords.
func SplitSentences(cleanedText string, maxWords int) []Sentence {
	if maxWords <= 0 {
		maxWords = 50
	}
	const minWords = 4

	tokens := strings.Fields(cleanedText)
	if len(tokens) == 0 {
		return nil
	}

	var sentences []Sentence
	var chunkTokens []string
	var chunkWords []string

	flush := func() {
		if len(chunkWords) == 0 {
			return
		}
		sentences = append(sentences, Sentence{
			Text:  strings.Join(chunkTokens, " "),
			Words: append([]string(nil), chunkWords...),
		})
		chunkTokens = nil
		chunkWords = nil
	}

	for _, tok := range tokens {
		chunkTokens = append(chunkTokens, tok)
		if tok != "..." {
			chunkWords = append(chunkWords, tok)
		}

		isBoundary := tok == "..." ||
			strings.HasSuffix(tok, ".") ||
			strings.HasSuffix(tok, "!") ||
			strings.HasSuffix(tok, "?")

		if (isBoundary && len(chunkWords) >= minWords) || len(chunkWords) >= maxWords {
			flush()
		}
	}
	flush()
	return sentences
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
