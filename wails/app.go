package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/biblemeta"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var chapterPlayCancel context.CancelFunc

func stopChapterPlayback() {
	if chapterPlayCancel != nil {
		chapterPlayCancel()
		chapterPlayCancel = nil
	}
}

type LogosService struct {
	client    *api.Client
	ttsEngine *tts.Engine
	localDB   *localdb.DB
}

type BibleSummary struct {
	ID           string       `json:"id"`
	Abbreviation string       `json:"abbreviation"`
	Name         string       `json:"name"`
	NameLocal    string       `json:"nameLocal"`
	Description  string       `json:"description"`
	Language     api.Language `json:"language"`
	Type         string       `json:"type"`
	Source       string       `json:"source"`
}

type VoiceOption struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Engine string `json:"engine"`
	Label  string `json:"label"`
}

type SpeechSettings struct {
	Available   bool          `json:"available"`
	Engine      string        `json:"engine"`
	Rate        int           `json:"rate"`
	ActiveVoice *VoiceOption  `json:"activeVoice,omitempty"`
	Voices      []VoiceOption `json:"voices"`
}

type SyncedSpeechPlan struct {
	WordDurationsMs []int `json:"wordDurationsMs"`
}

func NewLogosService(client *api.Client, ttsEngine *tts.Engine) *LogosService {
	result := &LogosService{
		client:    client,
		ttsEngine: ttsEngine,
	}
	if db, err := localdb.Open(localdb.DefaultDBPath()); err == nil {
		result.localDB = db
	}
	return result
}

func (s *LogosService) ServiceName() string { return "LogosService" }

func (s *LogosService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	return nil
}

func (s *LogosService) ServiceShutdown() error {
	if s.localDB == nil {
		return nil
	}
	return s.localDB.Close()
}

func (s *LogosService) GetBibles(language string) ([]BibleSummary, error) {
	apiBibles, apiErr := s.client.GetBibles(language)

	seenID := map[string]bool{}
	seenAbbr := map[string]bool{}
	var items []BibleSummary

	for _, local := range s.localBibleSummaries(language) {
		normAbbr := strings.ToUpper(stripLangPrefix(local.Abbreviation))
		items = append(items, local)
		seenID[local.ID] = true
		seenAbbr[normAbbr] = true
	}

	if apiErr == nil {
		for _, bible := range apiBibles {
			if seenID[bible.ID] {
				continue
			}
			normAbbr := strings.ToUpper(stripLangPrefix(bible.Abbreviation))
			if seenAbbr[normAbbr] {
				continue
			}
			items = append(items, BibleSummary{
				ID:           bible.ID,
				Abbreviation: displayBibleAbbreviation(bible.Abbreviation),
				Name:         bible.Name,
				NameLocal:    bible.NameLocal,
				Description:  bible.Description,
				Language:     bible.Language,
				Type:         bible.Type,
				Source:       "api",
			})
			seenID[bible.ID] = true
			seenAbbr[normAbbr] = true
		}
	}

	sort.Slice(items, func(i, j int) bool {
		left := strings.ToUpper(stripLangPrefix(items[i].Abbreviation))
		right := strings.ToUpper(stripLangPrefix(items[j].Abbreviation))
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left < right
	})

	if apiErr != nil && len(items) == 0 {
		return nil, apiErr
	}
	return items, nil
}

func (s *LogosService) GetBooks(bibleID string) ([]api.Book, error) {
	if s.isLocalBible(bibleID) {
		books, err := s.localDB.ListBooks(bibleID)
		if err != nil {
			return nil, err
		}
		result := make([]api.Book, len(books))
		for i, book := range books {
			abbr := book.ShortName
			if abbr == "" {
				abbr = book.ID
			}
			result[i] = api.Book{
				ID:           book.ID,
				BibleID:      bibleID,
				Abbreviation: abbr,
				Name:         book.Name,
				NameLong:     book.Name,
			}
		}
		return result, nil
	}
	return s.client.GetBooks(bibleID)
}

func (s *LogosService) GetChapters(bibleID, bookID string) ([]api.Chapter, error) {
	if s.isLocalBible(bibleID) {
		chapters, err := s.localDB.ListChapters(bookID, bibleID)
		if err != nil {
			return nil, err
		}
		result := make([]api.Chapter, len(chapters))
		for i, chapter := range chapters {
			result[i] = api.Chapter{
				ID:       chapter.ID,
				BibleID:  bibleID,
				BookID:   bookID,
				Number:   strconv.Itoa(chapter.Number),
				Position: chapter.Number,
			}
		}
		return result, nil
	}
	return s.client.GetChapters(bibleID, bookID)
}

func (s *LogosService) GetChapter(bibleID, chapterID string) (api.ChapterContent, error) {
	if s.isLocalBible(bibleID) {
		return s.loadLocalChapter(bibleID, chapterID)
	}
	return s.client.GetChapter(bibleID, chapterID)
}

func (s *LogosService) Search(bibleID, query string, limit int) (api.SearchData, error) {
	if s.isLocalBible(bibleID) {
		results, err := s.localDB.Search(bibleID, query, limit)
		if err != nil {
			return api.SearchData{}, err
		}
		verses := make([]api.SearchVerse, len(results))
		for i, result := range results {
			verses[i] = api.SearchVerse{
				ID:        result.VerseID,
				OrgID:     result.VerseID,
				BookID:    result.BookID,
				BibleID:   bibleID,
				ChapterID: result.ChapterID,
				Reference: formatLocalReference(result.VerseID),
				Text:      result.Text,
			}
		}
		return api.SearchData{
			Query:      query,
			Limit:      limit,
			Offset:     0,
			Total:      len(verses),
			VerseCount: len(verses),
			Verses:     verses,
		}, nil
	}
	return s.client.Search(bibleID, query, limit)
}

func (s *LogosService) GetSpeechSettings() SpeechSettings {
	voices := s.ttsEngine.RefreshVoices()
	options := make([]VoiceOption, len(voices))
	for i, voice := range voices {
		options[i] = makeVoiceOption(voice)
	}
	var activeVoice *VoiceOption
	if voice := s.ttsEngine.ActiveVoice(); voice.ID != "" {
		value := makeVoiceOption(voice)
		activeVoice = &value
	}
	return SpeechSettings{
		Available:   s.ttsEngine.Available(),
		Engine:      s.ttsEngine.EngineName(),
		Rate:        s.ttsEngine.Rate(),
		ActiveVoice: activeVoice,
		Voices:      options,
	}
}

func (s *LogosService) SetVoice(voiceID string) (SpeechSettings, error) {
	for _, voice := range s.ttsEngine.ListVoices() {
		if voice.ID == voiceID {
			s.ttsEngine.SetVoiceEntry(voice)
			return s.GetSpeechSettings(), nil
		}
	}
	return s.GetSpeechSettings(), fmt.Errorf("voice not found: %s", voiceID)
}

func (s *LogosService) SetSpeechRate(rate int) SpeechSettings {
	if rate < 80 {
		rate = 80
	}
	if rate > 260 {
		rate = 260
	}
	s.ttsEngine.SetRate(rate)
	return s.GetSpeechSettings()
}

func (s *LogosService) SpeakText(text string) error {
	_, err := s.ttsEngine.Speak(text)
	return err
}

func (s *LogosService) SpeakChapter(content string) (SyncedSpeechPlan, error) {
	clean := tts.CleanForTTS(content)
	words := tts.SplitWords(clean)
	if len(words) == 0 {
		return SyncedSpeechPlan{}, nil
	}
	synced, err := s.ttsEngine.SpeakSynced(clean, words)
	if err != nil {
		return SyncedSpeechPlan{}, err
	}
	<-synced.Started
	result := make([]int, len(synced.WordDurations))
	for i, duration := range synced.WordDurations {
		ms := int(duration / time.Millisecond)
		if ms < 1 {
			ms = 1
		}
		result[i] = ms
	}
	return SyncedSpeechPlan{WordDurationsMs: result}, nil
}

// StartChapterPlayback synthesises the entire chapter as one continuous audio
// stream and plays it with word-level highlight sync.
//
// startWordIndex: pass 0 to start from the beginning; pass a word index to
// jump (seeks in cached PCM on cache hit, falls back to beginning on miss).
//
// Events emitted:
//
//	tts:synthesizing — immediately; frontend should show a spinner.
//	tts:ready        — {startWordIndex int, wordDurationsMs []int}; audio is
//	                   now playing; frontend should schedule word highlights.
//	tts:done         — playback cancelled or synthesis failed (frontend cleanup).
//	tts:error        — synthesis error string (frontend shows the message).
func (s *LogosService) StartChapterPlayback(content string, startWordIndex int) {
	stopChapterPlayback()
	clean := tts.CleanForTTS(content)
	words := tts.SplitWords(clean)
	if len(words) == 0 {
		return
	}

	application.Get().Event.Emit("tts:synthesizing", nil)

	ctx, cancel := context.WithCancel(context.Background())
	chapterPlayCancel = cancel

	go func() {
		defer cancel()
		synced, actualStart, err := s.ttsEngine.SpeakSyncedFrom(clean, words, startWordIndex)
		if err != nil {
			if ctx.Err() == nil {
				application.Get().Event.Emit("tts:error", err.Error())
			} else {
				application.Get().Event.Emit("tts:done", nil)
			}
			return
		}

		select {
		case <-synced.Started:
		case <-ctx.Done():
			application.Get().Event.Emit("tts:done", nil)
			return
		}

		ms := make([]int, len(synced.WordDurations))
		for i, d := range synced.WordDurations {
			v := int(d / time.Millisecond)
			if v < 1 {
				v = 1
			}
			ms[i] = v
		}
		application.Get().Event.Emit("tts:ready", map[string]any{
			"startWordIndex":  actualStart,
			"wordDurationsMs": ms,
		})
	}()
}

func (s *LogosService) StopSpeaking() {
	stopChapterPlayback() // cancel synthesis goroutine → emits tts:done
	s.ttsEngine.Stop()
}

func (s *LogosService) IsSpeaking() bool {
	return s.ttsEngine.IsPlaying()
}

// ── local helpers ────────────────────────────────────────────────────────────

func (s *LogosService) localBibleSummaries(language string) []BibleSummary {
	if s.localDB == nil {
		return nil
	}
	translations, err := s.localDB.ListTranslations()
	if err != nil {
		return nil
	}
	result := make([]BibleSummary, 0, len(translations))
	for _, translation := range translations {
		if !biblemeta.MatchesLanguage(translation.Language, language) {
			continue
		}
		description := strings.TrimSpace(translation.Description)
		if description == "" {
			description = "Offline import"
		}
		languageName := displayLanguageName(translation.Language)
		result = append(result, BibleSummary{
			ID:           translation.ID,
			Abbreviation: displayBibleAbbreviation(translation.Abbreviation),
			Name:         translation.Name,
			NameLocal:    translation.Name,
			Description:  description,
			Language: api.Language{
				ID:        translation.Language,
				Name:      languageName,
				NameLocal: languageName,
			},
			Type:   "text",
			Source: "local",
		})
	}
	return result
}

func (s *LogosService) isLocalBible(bibleID string) bool {
	if s.localDB == nil {
		return false
	}
	translations, err := s.localDB.ListTranslations()
	if err != nil {
		return false
	}
	for _, translation := range translations {
		if translation.ID == bibleID {
			return true
		}
	}
	return false
}

func (s *LogosService) loadLocalChapter(bibleID, chapterID string) (api.ChapterContent, error) {
	if s.localDB == nil {
		return api.ChapterContent{}, fmt.Errorf("local bible database unavailable")
	}
	bookID, _, found := strings.Cut(chapterID, ".")
	if !found || bookID == "" {
		return api.ChapterContent{}, fmt.Errorf("invalid local chapter id: %s", chapterID)
	}
	bookName := bookID
	translationName := bibleID
	if books, err := s.localDB.ListBooks(bibleID); err == nil {
		for _, book := range books {
			if book.ID == bookID {
				bookName = book.Name
				break
			}
		}
	}
	for _, translation := range s.localBibleSummaries("") {
		if translation.ID == bibleID {
			translationName = translation.Name
			break
		}
	}
	content, err := s.localDB.GetChapterContent(chapterID, bibleID)
	if err != nil {
		return api.ChapterContent{}, err
	}
	chapters, err := s.localDB.ListChapters(bookID, bibleID)
	if err != nil {
		return api.ChapterContent{}, err
	}
	var (
		number string
		prev   *api.ChapterRef
		next   *api.ChapterRef
	)
	for i, chapter := range chapters {
		if chapter.ID != chapterID {
			continue
		}
		number = strconv.Itoa(chapter.Number)
		if i > 0 {
			prev = &api.ChapterRef{
				ID:     chapters[i-1].ID,
				Number: strconv.Itoa(chapters[i-1].Number),
				BookID: bookID,
			}
		}
		if i+1 < len(chapters) {
			next = &api.ChapterRef{
				ID:     chapters[i+1].ID,
				Number: strconv.Itoa(chapters[i+1].Number),
				BookID: bookID,
			}
		}
		break
	}
	if number == "" {
		return api.ChapterContent{}, fmt.Errorf("local chapter not found: %s", chapterID)
	}
	return api.ChapterContent{
		ID:         chapterID,
		BibleID:    bibleID,
		BookID:     bookID,
		Number:     number,
		Reference:  fmt.Sprintf("%s %s", bookName, number),
		Content:    strings.TrimSpace(content),
		VerseCount: strings.Count(content, "["),
		Copyright:  fmt.Sprintf("%s (offline import)", translationName),
		Next:       next,
		Previous:   prev,
	}, nil
}

func makeVoiceOption(voice tts.VoiceEntry) VoiceOption {
	return VoiceOption{
		Name:   voice.Name,
		ID:     voice.ID,
		Engine: voice.Engine,
		Label:  shortVoiceLabel(voice),
	}
}

func stripLangPrefix(abbr string) string {
	return biblemeta.StripLangPrefix(abbr)
}

func displayBibleAbbreviation(abbr string) string {
	return biblemeta.DisplayBibleAbbreviation(abbr)
}

func displayLanguageName(code string) string {
	return biblemeta.DisplayLanguageName(code)
}

func formatLocalReference(verseID string) string {
	parts := strings.Split(verseID, ".")
	if len(parts) != 3 {
		return verseID
	}
	return fmt.Sprintf("%s %s:%s", parts[0], parts[1], parts[2])
}

func shortVoiceLabel(voice tts.VoiceEntry) string {
	if voice.Name == "" {
		return ttsDisplayName(voice.Engine)
	}
	name := voice.Name
	if idx := strings.Index(name, ": "); idx >= 0 {
		name = name[idx+2:]
	}
	if voice.Engine == "piper" {
		parts := strings.Split(name, "-")
		if len(parts) >= 2 {
			name = strings.Join(parts[1:], "-")
		}
	}
	if voice.Engine == "kokoro" {
		if idx := strings.Index(name, " "); idx > 0 {
			name = name[:idx]
		}
	}
	return ttsDisplayName(voice.Engine) + " · " + name
}

func ttsDisplayName(engine string) string {
	switch engine {
	case "speechd":
		return "Speech Dispatcher"
	case "espeak":
		return "eSpeak"
	default:
		return strings.Title(engine) //nolint:staticcheck
	}
}

// ── TTS extras ──────────────────────────────────────────────────────────────

func (s *LogosService) PauseSpeaking() {
	s.ttsEngine.Pause()
}

func (s *LogosService) ResumeSpeaking() {
	s.ttsEngine.Resume()
}

func (s *LogosService) IsPaused() bool {
	return s.ttsEngine.IsPaused()
}
