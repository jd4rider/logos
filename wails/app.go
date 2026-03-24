package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/ai"
	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/crawler"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/importer"
	"github.com/jd4rider/logos/internal/pdf"
	coretts "github.com/jd4rider/logos/internal/tts"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main Wails application struct
type App struct {
	ctx          context.Context
	client       *api.Client
	ttsEng       *coretts.Engine
	localDB      *localdb.DB
	aiClient     *ai.Client
	aiCancel     context.CancelFunc
	importCancel context.CancelFunc
}

// LibraryEntry represents a saved AI-generated item
type LibraryEntry struct {
	Kind    string `json:"kind"` // "devotional" | "sermon" | "note"
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Ref     string `json:"ref"`
	Content string `json:"content"`
	Model   string `json:"model"`
	Date    string `json:"date"`
}

// NewApp creates a new App instance
func NewApp(client *api.Client, ttsEng *coretts.Engine) *App {
	app := &App{client: client, ttsEng: ttsEng, aiClient: ai.NewClient()}
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

// PauseSpeaking pauses TTS playback
func (a *App) PauseSpeaking() { a.ttsEng.Pause() }

// ResumeSpeaking resumes paused TTS playback
func (a *App) ResumeSpeaking() { a.ttsEng.Resume() }

// IsPaused returns whether TTS is currently paused
func (a *App) IsPaused() bool { return a.ttsEng.IsPaused() }

// ListVoices returns all available TTS voices
func (a *App) ListVoices() []coretts.VoiceEntry { return a.ttsEng.ListVoices() }

// GetActiveVoice returns the currently active voice
func (a *App) GetActiveVoice() coretts.VoiceEntry { return a.ttsEng.ActiveVoice() }

// SetVoice sets the active voice by ID
func (a *App) SetVoice(voiceID string) {
	for _, v := range a.ttsEng.ListVoices() {
		if v.ID == voiceID {
			a.ttsEng.SetVoiceEntry(v)
			return
		}
	}
}

// GetTTSRate returns current speech rate in WPM
func (a *App) GetTTSRate() int { return a.ttsEng.Rate() }

// SetTTSRate sets speech rate in WPM
func (a *App) SetTTSRate(rate int) { a.ttsEng.SetRate(rate) }

// SpeakSynced synthesizes text, waits for audio to start, then returns word
// durations in milliseconds. The frontend uses these to drive word highlighting.
func (a *App) SpeakSynced(text string) ([]int64, error) {
	clean := coretts.CleanForTTS(text)
	words := coretts.SplitWords(clean)
	synced, err := a.ttsEng.SpeakSynced(clean, words)
	if err != nil {
		return nil, err
	}
	<-synced.Started
	durs := make([]int64, len(synced.WordDurations))
	for i, d := range synced.WordDurations {
		durs[i] = d.Milliseconds()
	}
	return durs, nil
}

// ── AI ────────────────────────────────────────────────────────────────────────

// IsAIAvailable returns whether the AI backend (Ollama) is reachable
func (a *App) IsAIAvailable() bool {
	if a.aiClient == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return a.aiClient.IsAvailable(ctx)
}

// ListAIModels returns available Ollama models
func (a *App) ListAIModels() []string {
	if a.aiClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	models, _ := a.aiClient.ListModels(ctx)
	return models
}

// StartAIStream starts an AI generation stream. Events emitted:
//   - "ai:token"  — string payload with each generated token
//   - "ai:done"   — empty payload when generation completes
//   - "ai:error"  — string payload with error message
//
// action values: "explain_verse" | "explain_chapter" | "devotional" | "sermon" | "ask"
func (a *App) StartAIStream(action, verseRef, verseText, chapterText, bookName, chapterNum, translation, userInput string) {
	if a.aiCancel != nil {
		a.aiCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	a.aiCancel = cancel

	go func() {
		defer cancel()

		vCtx := ai.VerseContext{
			Reference:   verseRef,
			Text:        verseText,
			Translation: translation,
		}

		var tokens <-chan string
		var errc <-chan error
		switch action {
		case "explain_verse":
			tokens, errc = a.aiClient.ExplainVerse(ctx, vCtx)
		case "explain_chapter":
			tokens, errc = a.aiClient.ExplainChapter(ctx, bookName, chapterNum, translation, chapterText)
		case "devotional":
			tokens, errc = a.aiClient.GenerateDevotional(ctx, vCtx, "")
		case "sermon":
			tokens, errc = a.aiClient.GenerateSermon(ctx, vCtx, "")
		case "ask":
			tokens, errc = a.aiClient.AskAboutVerse(ctx, userInput, vCtx)
		default:
			runtime.EventsEmit(a.ctx, "ai:error", "unknown action: "+action)
			return
		}

		for {
			select {
			case t, ok := <-tokens:
				if !ok {
					runtime.EventsEmit(a.ctx, "ai:done", "")
					return
				}
				runtime.EventsEmit(a.ctx, "ai:token", t)
			case err, ok := <-errc:
				if ok && err != nil {
					runtime.EventsEmit(a.ctx, "ai:error", err.Error())
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopAIStream cancels any running AI generation
func (a *App) StopAIStream() {
	if a.aiCancel != nil {
		a.aiCancel()
		a.aiCancel = nil
	}
}

// ── Library ───────────────────────────────────────────────────────────────────

// SaveToLibrary saves AI-generated content to the local database
func (a *App) SaveToLibrary(kind, title, ref, content, model string) error {
	if a.localDB == nil {
		return fmt.Errorf("no local database")
	}
	now := time.Now()
	switch kind {
	case "devotional":
		_, err := a.localDB.SaveDevotional(localdb.Devotional{
			Title: title, VerseRef: ref, Content: content, AIModel: model, CreatedAt: now,
		})
		return err
	case "sermon":
		_, err := a.localDB.SaveSermon(localdb.Sermon{
			Title: title, ScriptureRef: ref, Content: content, AIModel: model, CreatedAt: now,
		})
		return err
	default:
		_, err := a.localDB.SaveNote(localdb.AINote{
			VerseID: ref, Content: content, AIModel: model, CreatedAt: now,
		})
		return err
	}
}

// ListLibrary returns saved library entries sorted by date descending
func (a *App) ListLibrary() ([]LibraryEntry, error) {
	if a.localDB == nil {
		return nil, nil
	}
	var entries []LibraryEntry
	if devs, err := a.localDB.ListDevotionals(30); err == nil {
		for _, d := range devs {
			entries = append(entries, LibraryEntry{
				Kind: "devotional", ID: d.ID, Title: d.Title,
				Ref: d.VerseRef, Content: d.Content, Model: d.AIModel,
				Date: d.CreatedAt.Format("Jan 2, 2006"),
			})
		}
	}
	if serms, err := a.localDB.ListSermons(30); err == nil {
		for _, s := range serms {
			entries = append(entries, LibraryEntry{
				Kind: "sermon", ID: s.ID, Title: s.Title,
				Ref: s.ScriptureRef, Content: s.Content, Model: s.AIModel,
				Date: s.CreatedAt.Format("Jan 2, 2006"),
			})
		}
	}
	if notes, err := a.localDB.ListAllNotes(30); err == nil {
		for _, n := range notes {
			t := n.Content
			if len(t) > 60 {
				t = t[:60] + "…"
			}
			entries = append(entries, LibraryEntry{
				Kind: "note", ID: n.ID, Title: t,
				Ref: n.VerseID, Content: n.Content, Model: n.AIModel,
				Date: n.CreatedAt.Format("Jan 2, 2006"),
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Date > entries[j].Date })
	return entries, nil
}

// ExportPDF exports AI-generated content to a PDF on the Desktop
func (a *App) ExportPDF(kind, title, ref, content string) (string, error) {
	home, _ := os.UserHomeDir()
	outDir := filepath.Join(home, "Desktop")
	_ = os.MkdirAll(outDir, 0o755)
	ts := time.Now().Format("20060102-150405")
	var outPath string
	var err error
	switch kind {
	case "devotional":
		outPath = filepath.Join(outDir, "logos-devotional-"+ts+".pdf")
		err = pdf.ExportDevotional(title, ref, "", content, outPath)
	case "sermon":
		outPath = filepath.Join(outDir, "logos-sermon-"+ts+".pdf")
		err = pdf.ExportSermon(title, ref, content, outPath)
	default:
		outPath = filepath.Join(outDir, "logos-note-"+ts+".pdf")
		err = pdf.ExportNote(ref, content, outPath)
	}
	if err != nil {
		return "", err
	}
	return outPath, nil
}

// ── Import ────────────────────────────────────────────────────────────────────

// ImportBibleURL crawls a Bible website and imports it into the local database.
// Events: "import:progress" (string), "import:done", "import:error" (string)
func (a *App) ImportBibleURL(url, name, abbr, lang string) {
	if a.importCancel != nil {
		a.importCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	a.importCancel = cancel
	go func() {
		defer cancel()
		opts := crawler.Options{
			Name:         name,
			Abbreviation: abbr,
			Language:     lang,
			Progress: func(msg string) {
				runtime.EventsEmit(a.ctx, "import:progress", msg)
			},
		}
		if err := crawler.Crawl(a.localDB, url, opts); err != nil {
			runtime.EventsEmit(a.ctx, "import:error", err.Error())
			return
		}
		runtime.EventsEmit(ctx, "import:done", "Import complete")
	}()
}

// ImportBibleFile imports a local CSV or SQLite Bible file.
// Events: "import:progress" (string), "import:done", "import:error" (string)
func (a *App) ImportBibleFile(path, name, abbr, lang string) {
	if a.importCancel != nil {
		a.importCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	a.importCancel = cancel
	go func() {
		defer cancel()
		opts := importer.ImportOptions{
			Name:         name,
			Abbreviation: abbr,
			Language:     lang,
			Progress: func(msg string) {
				runtime.EventsEmit(a.ctx, "import:progress", msg)
			},
		}
		ext := strings.ToLower(filepath.Ext(path))
		var err error
		if ext == ".csv" || ext == ".tsv" {
			err = importer.ImportCSV(a.localDB, path, opts)
		} else {
			err = importer.ImportSQLiteFile(a.localDB, path, opts)
		}
		if err != nil {
			runtime.EventsEmit(a.ctx, "import:error", err.Error())
			return
		}
		runtime.EventsEmit(ctx, "import:done", "Import complete")
	}()
}

// CancelImport cancels a running import
func (a *App) CancelImport() {
	if a.importCancel != nil {
		a.importCancel()
		a.importCancel = nil
	}
}

// OpenFileDialog opens a native file picker dialog
func (a *App) OpenFileDialog() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Bible File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Bible Files (*.csv, *.db, *.sqlite)", Pattern: "*.csv;*.db;*.sqlite;*.sqlite3"},
		},
	})
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
