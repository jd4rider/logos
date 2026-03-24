package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jd4rider/logos/internal/api"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main Wails application struct
type App struct {
	ctx     context.Context
	client  *api.Client
	ttsEng  *tts.Engine
	localDB *localdb.DB
}

// NewApp creates a new App instance
func NewApp(client *api.Client, ttsEng *tts.Engine) *App {
	app := &App{client: client, ttsEng: ttsEng}
	if db, err := localdb.Open(localdb.DefaultDBPath()); err == nil {
		app.localDB = db
	}
	return app
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) domReady(ctx context.Context) {
	runtime.WindowShow(ctx)
	runtime.WindowMaximise(ctx)
}

// GetBibles returns available Bible translations
func (a *App) GetBibles(language string) ([]api.Bible, error) {
	apiBibles, apiErr := a.client.GetBibles(language)

	seenID := map[string]bool{}
	seenAbbr := map[string]bool{}
	var items []api.Bible

	if a.localDB != nil {
		if locals, err := a.localDB.ListTranslations(); err == nil {
			for _, t := range locals {
				normAbbr := strings.ToUpper(stripLangPrefix(t.Abbreviation))
				items = append(items, api.Bible{
					ID:                t.ID,
					Name:              t.Name,
					NameLocal:         t.Name,
					Abbreviation:      displayBibleAbbreviation(t.Abbreviation),
					AbbreviationLocal: displayBibleAbbreviation(t.Abbreviation),
					Description:       t.Description,
					Language: api.Language{
						ID:        t.Language,
						Name:      displayLanguageName(t.Language),
						NameLocal: displayLanguageName(t.Language),
					},
					Type: "text",
				})
				seenID[t.ID] = true
				seenAbbr[normAbbr] = true
			}
		}
	}

	if apiErr == nil {
		for _, b := range apiBibles {
			if seenID[b.ID] {
				continue
			}
			normAbbr := strings.ToUpper(stripLangPrefix(b.Abbreviation))
			if seenAbbr[normAbbr] {
				continue
			}
			b.Abbreviation = displayBibleAbbreviation(b.Abbreviation)
			if b.AbbreviationLocal != "" {
				b.AbbreviationLocal = displayBibleAbbreviation(b.AbbreviationLocal)
			}
			items = append(items, b)
			seenID[b.ID] = true
			seenAbbr[normAbbr] = true
		}
	}

	sort.Slice(items, func(i, j int) bool {
		ai := strings.ToUpper(stripLangPrefix(items[i].Abbreviation))
		aj := strings.ToUpper(stripLangPrefix(items[j].Abbreviation))
		return ai < aj
	})

	if apiErr != nil && len(items) == 0 {
		return nil, apiErr
	}
	return items, nil
}

// GetBooks returns books for a Bible
func (a *App) GetBooks(bibleID string) ([]api.Book, error) {
	if a.isLocalTranslation(bibleID) {
		books, err := a.localDB.ListBooks(bibleID)
		if err != nil {
			return nil, err
		}

		out := make([]api.Book, len(books))
		for i, b := range books {
			abbr := b.ShortName
			if abbr == "" {
				abbr = b.ID
			}
			out[i] = api.Book{
				ID:           b.ID,
				BibleID:      bibleID,
				Abbreviation: abbr,
				Name:         b.Name,
				NameLong:     b.Name,
			}
		}
		return out, nil
	}
	return a.client.GetBooks(bibleID)
}

// GetChapters returns chapters for a book
func (a *App) GetChapters(bibleID, bookID string) ([]api.Chapter, error) {
	if a.isLocalTranslation(bibleID) {
		chapters, err := a.localDB.ListChapters(bookID, bibleID)
		if err != nil {
			return nil, err
		}

		out := make([]api.Chapter, len(chapters))
		for i, c := range chapters {
			out[i] = api.Chapter{
				ID:       c.ID,
				BibleID:  bibleID,
				BookID:   bookID,
				Number:   strconv.Itoa(c.Number),
				Position: c.Number,
			}
		}
		return out, nil
	}
	return a.client.GetChapters(bibleID, bookID)
}

// GetChapter returns the full content of a chapter
func (a *App) GetChapter(bibleID, chapterID string) (api.ChapterContent, error) {
	if a.isLocalTranslation(bibleID) {
		return a.loadOfflineChapter(bibleID, chapterID)
	}
	return a.client.GetChapter(bibleID, chapterID)
}

// GetVerse returns the full content of a verse
func (a *App) GetVerse(bibleID, verseID string) (api.VerseContent, error) {
	return a.client.GetVerse(bibleID, verseID)
}

// Search searches the Bible
func (a *App) Search(bibleID, query string, limit int) (api.SearchData, error) {
	if a.isLocalTranslation(bibleID) {
		results, err := a.localDB.Search(bibleID, query, limit)
		if err != nil {
			return api.SearchData{}, err
		}

		verses := make([]api.SearchVerse, len(results))
		for i, r := range results {
			verses[i] = api.SearchVerse{
				ID:        r.VerseID,
				BookID:    r.BookID,
				BibleID:   bibleID,
				ChapterID: r.ChapterID,
				Reference: formatLocalReference(r.VerseID),
				Text:      r.Text,
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
	return a.client.Search(bibleID, query, limit)
}

// GetTTSEngine returns the name of the active TTS engine
func (a *App) GetTTSEngine() string {
	return a.ttsEng.EngineName()
}

// SpeakText speaks text using the TTS engine
func (a *App) SpeakText(text string) error {
	_, err := a.ttsEng.Speak(text)
	return err
}

// StopSpeaking stops TTS playback
func (a *App) StopSpeaking() {
	a.ttsEng.Stop()
}

// IsSpeaking returns whether TTS is currently active
func (a *App) IsSpeaking() bool {
	return a.ttsEng.IsPlaying()
}

func (a *App) isLocalTranslation(translationID string) bool {
	if a.localDB == nil {
		return false
	}
	translations, err := a.localDB.ListTranslations()
	if err != nil {
		return false
	}
	for _, t := range translations {
		if t.ID == translationID {
			return true
		}
	}
	return false
}

func (a *App) loadOfflineChapter(bibleID, chapterID string) (api.ChapterContent, error) {
	if a.localDB == nil {
		return api.ChapterContent{}, fmt.Errorf("local bible database unavailable")
	}

	bookID, _, found := strings.Cut(chapterID, ".")
	if !found || bookID == "" {
		return api.ChapterContent{}, fmt.Errorf("invalid local chapter id: %s", chapterID)
	}

	bookName := bookID
	if books, err := a.localDB.ListBooks(bibleID); err == nil {
		for _, b := range books {
			if b.ID == bookID {
				bookName = b.Name
				break
			}
		}
	}

	content, err := a.localDB.GetChapterContent(chapterID, bibleID)
	if err != nil {
		return api.ChapterContent{}, err
	}

	chapters, err := a.localDB.ListChapters(bookID, bibleID)
	if err != nil {
		return api.ChapterContent{}, err
	}

	var (
		number string
		prev   *api.ChapterRef
		next   *api.ChapterRef
	)
	for i, ch := range chapters {
		if ch.ID != chapterID {
			continue
		}
		number = strconv.Itoa(ch.Number)
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

	copyright := "Offline import"
	if translations, err := a.localDB.ListTranslations(); err == nil {
		for _, t := range translations {
			if t.ID == bibleID {
				copyright = fmt.Sprintf("%s (offline import)", t.Name)
				break
			}
		}
	}

	return api.ChapterContent{
		ID:         chapterID,
		BibleID:    bibleID,
		BookID:     bookID,
		Number:     number,
		Reference:  fmt.Sprintf("%s %s", bookName, number),
		Content:    strings.TrimSpace(content),
		VerseCount: strings.Count(content, "["),
		Copyright:  copyright,
		Next:       next,
		Previous:   prev,
	}, nil
}

func stripLangPrefix(abbr string) string {
	known3 := []string{
		"eng", "spa", "esp", "fra", "deu", "ger", "por", "zho", "hin",
		"ara", "rus", "kor", "jpn", "vie", "ind", "nld", "ita", "pol",
		"tur", "heb", "grc", "lat", "afr", "swa", "urd", "ben", "tam",
	}
	lower := strings.ToLower(abbr)
	for _, pfx := range known3 {
		if strings.HasPrefix(lower, pfx) && len(abbr) > len(pfx) {
			result := abbr[len(pfx):]
			if len(result) >= 2 {
				return result
			}
		}
	}

	known2 := []string{"en", "es", "fr", "de", "pt", "it", "nl", "pl"}
	for _, pfx := range known2 {
		if strings.HasPrefix(lower, pfx) && len(abbr) > len(pfx) {
			result := abbr[len(pfx):]
			if len(result) >= 2 && result[0] >= 'A' && result[0] <= 'Z' {
				return result
			}
		}
	}
	return abbr
}

func displayBibleAbbreviation(abbr string) string {
	return stripLangPrefix(abbr)
}

func displayLanguageName(code string) string {
	m := map[string]string{
		"eng": "English", "en": "English",
		"spa": "Spanish", "es": "Spanish", "esp": "Spanish",
		"fra": "French", "fr": "French",
		"deu": "German", "ger": "German", "de": "German",
		"ita": "Italian", "it": "Italian",
		"por": "Portuguese", "pt": "Portuguese",
		"nld": "Dutch", "nl": "Dutch",
		"pol": "Polish", "pl": "Polish",
		"rus": "Russian", "ru": "Russian",
		"zho": "Chinese", "zh": "Chinese",
		"hin": "Hindi", "hi": "Hindi",
		"ara": "Arabic", "ar": "Arabic",
		"kor": "Korean", "ko": "Korean",
		"jpn": "Japanese", "ja": "Japanese",
		"vie": "Vietnamese", "vi": "Vietnamese",
		"ind": "Indonesian", "id": "Indonesian",
		"tur": "Turkish", "tr": "Turkish",
		"swa": "Swahili", "sw": "Swahili",
		"urd": "Urdu", "ur": "Urdu",
		"ben": "Bengali", "bn": "Bengali",
		"tam": "Tamil", "ta": "Tamil",
		"afr": "Afrikaans",
		"lat": "Latin",
		"grc": "Greek (Ancient)",
		"heb": "Hebrew",
	}
	if name, ok := m[strings.ToLower(strings.TrimSpace(code))]; ok {
		return name
	}
	return code
}

func formatLocalReference(verseID string) string {
	parts := strings.Split(verseID, ".")
	if len(parts) != 3 {
		return verseID
	}
	return fmt.Sprintf("%s %s:%s", parts[0], parts[1], parts[2])
}
