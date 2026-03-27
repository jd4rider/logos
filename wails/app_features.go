package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/ai"
	"github.com/jd4rider/logos/internal/crawler"
	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/importer"
	"github.com/jd4rider/logos/internal/pdf"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ── types ────────────────────────────────────────────────────────────────────

// LibraryEntry represents a saved AI-generated item
type LibraryEntry struct {
	Kind    string `json:"kind"`
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Ref     string `json:"ref"`
	Content string `json:"content"`
	Model   string `json:"model"`
	Date    string `json:"date"`
}

// ── AI ───────────────────────────────────────────────────────────────────────

type featureService struct {
	aiClient     *ai.Client
	aiCancel     context.CancelFunc
	importCancel context.CancelFunc
}

var features = &featureService{
	aiClient: ai.NewClient(),
}

func (s *LogosService) IsAIAvailable() bool {
	if features.aiClient == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return features.aiClient.IsAvailable(ctx)
}

func (s *LogosService) ListAIModels() []string {
	if features.aiClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	models, _ := features.aiClient.ListModels(ctx)
	return models
}

// StartAIStream starts an AI generation stream.
// Events: "ai:token" (string), "ai:done" (empty), "ai:error" (string)
// action: "explain_verse" | "explain_chapter" | "devotional" | "sermon" | "ask"
func (s *LogosService) StartAIStream(action, verseRef, verseText, chapterText, bookName, chapterNum, translation, userInput string) {
	if features.aiCancel != nil {
		features.aiCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	features.aiCancel = cancel

	go func() {
		defer cancel()
		emit := func(name string, data any) {
			application.Get().Event.Emit(name, data)
		}

		vCtx := ai.VerseContext{
			Reference:   verseRef,
			Text:        verseText,
			Translation: translation,
		}
		if strings.TrimSpace(vCtx.Text) == "" {
			vCtx.Text = chapterText
		}

		var tokens <-chan string
		var errc <-chan error
		switch action {
		case "explain_verse":
			tokens, errc = features.aiClient.ExplainVerse(ctx, vCtx)
		case "explain_chapter":
			tokens, errc = features.aiClient.ExplainChapter(ctx, bookName, chapterNum, translation, chapterText)
		case "devotional":
			tokens, errc = features.aiClient.GenerateDevotional(ctx, vCtx, "")
		case "sermon":
			tokens, errc = features.aiClient.GenerateSermon(ctx, vCtx, "")
		case "ask":
			tokens, errc = features.aiClient.AskAboutPassage(ctx, userInput, vCtx)
		default:
			emit("ai:error", "unknown action: "+action)
			return
		}

		for {
			select {
			case t, ok := <-tokens:
				if !ok {
					emit("ai:done", "")
					return
				}
				emit("ai:token", t)
			case err, ok := <-errc:
				if ok && err != nil {
					emit("ai:error", err.Error())
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *LogosService) StopAIStream() {
	if features.aiCancel != nil {
		features.aiCancel()
		features.aiCancel = nil
	}
}

// ── Library ──────────────────────────────────────────────────────────────────

func (s *LogosService) SaveToLibrary(kind, title, ref, content, model string) error {
	if s.localDB == nil {
		return fmt.Errorf("no local database")
	}
	now := time.Now()
	switch kind {
	case "devotional":
		_, err := s.localDB.SaveDevotional(localdb.Devotional{
			Title: title, VerseRef: ref, Content: content, AIModel: model, CreatedAt: now,
		})
		return err
	case "sermon":
		_, err := s.localDB.SaveSermon(localdb.Sermon{
			Title: title, ScriptureRef: ref, Content: content, AIModel: model, CreatedAt: now,
		})
		return err
	default:
		_, err := s.localDB.SaveNote(localdb.AINote{
			VerseID: ref, Content: content, AIModel: model, CreatedAt: now,
		})
		return err
	}
}

func (s *LogosService) ListLibrary() ([]LibraryEntry, error) {
	if s.localDB == nil {
		return nil, nil
	}
	var entries []LibraryEntry
	if devs, err := s.localDB.ListDevotionals(30); err == nil {
		for _, d := range devs {
			entries = append(entries, LibraryEntry{
				Kind: "devotional", ID: d.ID, Title: d.Title,
				Ref: d.VerseRef, Content: d.Content, Model: d.AIModel,
				Date: d.CreatedAt.Format("Jan 2, 2006"),
			})
		}
	}
	if serms, err := s.localDB.ListSermons(30); err == nil {
		for _, s := range serms {
			entries = append(entries, LibraryEntry{
				Kind: "sermon", ID: s.ID, Title: s.Title,
				Ref: s.ScriptureRef, Content: s.Content, Model: s.AIModel,
				Date: s.CreatedAt.Format("Jan 2, 2006"),
			})
		}
	}
	if notes, err := s.localDB.ListAllNotes(30); err == nil {
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

func (s *LogosService) ExportPDF(kind, title, ref, content string) (string, error) {
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

func (s *LogosService) ImportBibleURL(url, name, abbr, lang string) {
	if features.importCancel != nil {
		features.importCancel()
	}
	_, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	features.importCancel = cancel

	go func() {
		defer cancel()
		emit := func(name string, data any) {
			application.Get().Event.Emit(name, data)
		}
		opts := crawler.Options{
			Name:         name,
			Abbreviation: abbr,
			Language:     lang,
			Progress: func(msg string) {
				emit("import:progress", msg)
			},
		}
		if err := crawler.Crawl(s.localDB, url, opts); err != nil {
			emit("import:error", err.Error())
			return
		}
		emit("import:done", "Import complete")
	}()
}

func (s *LogosService) ImportBibleFile(path, name, abbr, lang string) {
	if features.importCancel != nil {
		features.importCancel()
	}
	_, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	features.importCancel = cancel

	go func() {
		defer cancel()
		emit := func(evtName string, data any) {
			application.Get().Event.Emit(evtName, data)
		}
		opts := importer.ImportOptions{
			Name:         name,
			Abbreviation: abbr,
			Language:     lang,
			Progress: func(msg string) {
				emit("import:progress", msg)
			},
		}
		ext := strings.ToLower(filepath.Ext(path))
		var err error
		if ext == ".csv" || ext == ".tsv" {
			err = importer.ImportCSV(s.localDB, path, opts)
		} else {
			err = importer.ImportSQLiteFile(s.localDB, path, opts)
		}
		if err != nil {
			emit("import:error", err.Error())
			return
		}
		emit("import:done", "Import complete")
	}()
}

func (s *LogosService) CancelImport() {
	if features.importCancel != nil {
		features.importCancel()
		features.importCancel = nil
	}
}

func (s *LogosService) OpenFileDialog() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Select Bible File").
		AddFilter("Bible Files (*.csv, *.db, *.sqlite)", "*.csv;*.db;*.sqlite;*.sqlite3").
		PromptForSingleSelection()
}

// ── Chapter TTS precache ─────────────────────────────────────────────────────

var chapterPrecacheCancel context.CancelFunc

// PrecacheChapter begins synthesising TTS audio for the given chapter HTML/text
// in the background so that StartChapterPlayback starts instantly.  Safe to call on
// every chapter navigation — any in-progress precache for a previous chapter is
// cancelled first.
func (s *LogosService) PrecacheChapter(content string) {
	if chapterPrecacheCancel != nil {
		chapterPrecacheCancel()
	}
	if !s.ttsEngine.Available() {
		return
	}
	clean := tts.CleanForTTS(content)
	if clean == "" {
		return
	}
	words := tts.SplitWords(clean)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	chapterPrecacheCancel = cancel
	go func() {
		defer cancel()
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = s.ttsEngine.Precache(clean, words)
	}()
}

// ── AI TTS: precache + speak ──────────────────────────────────────────────────

var aiPrecacheCancel context.CancelFunc

// StartAIPrecache begins synthesising TTS audio for the given text in the
// background and writes it to the disk cache.  Called automatically when AI
// streaming finishes so that SpeakAIContent is instant for the user.
func (s *LogosService) StartAIPrecache(text string) {
	if aiPrecacheCancel != nil {
		aiPrecacheCancel()
	}
	if !s.ttsEngine.Available() {
		return
	}
	clean := tts.CleanForTTS(text)
	words := tts.SplitWords(clean)
	if len(words) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	aiPrecacheCancel = cancel
	go func() {
		defer cancel()
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = s.ttsEngine.Precache(clean, words)
	}()
}

// SpeakAIContent synthesises and plays the given AI-generated text, returning
// word durations for live highlighting.  Uses the disk cache so if
// StartAIPrecache was called first it will begin almost instantly.
func (s *LogosService) SpeakAIContent(text string) (SyncedSpeechPlan, error) {
	clean := tts.CleanForTTS(text)
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
	for i, d := range synced.WordDurations {
		ms := int(d / time.Millisecond)
		if ms < 1 {
			ms = 1
		}
		result[i] = ms
	}
	return SyncedSpeechPlan{WordDurationsMs: result}, nil
}

// SaveLibraryAudio stores TTS cache key + word durations for a library entry
// so future reads are instant.  Call after SpeakAIContent returns.
func (s *LogosService) SaveLibraryAudio(kind string, id int64, wordDurationsJSON string) error {
	if s.localDB == nil {
		return nil
	}
	voice := s.ttsEngine.ActiveVoice()
	cacheKey := tts.CacheKey(voice.Engine, voice.ID, s.ttsEngine.Rate(), "")
	return s.localDB.UpdateLibraryAudio(kind, id, cacheKey, wordDurationsJSON)
}

// GetLibraryAudio returns stored word durations for a library entry if the
// TTS cache key still matches the current voice/rate; otherwise synthesises
// fresh and returns new durations.
func (s *LogosService) GetLibraryAudio(kind string, id int64, content string) (SyncedSpeechPlan, error) {
	if s.localDB != nil {
		storedKey, durJSON, err := s.localDB.GetLibraryAudio(kind, id)
		if err == nil && storedKey != "" {
			voice := s.ttsEngine.ActiveVoice()
			currentKey := tts.CacheKey(voice.Engine, voice.ID, s.ttsEngine.Rate(), "")
			if storedKey == currentKey && durJSON != "[]" && durJSON != "" {
				// parse stored durations
				var durs []int
				if jsonErr := jsonUnmarshal([]byte(durJSON), &durs); jsonErr == nil && len(durs) > 0 {
					return SyncedSpeechPlan{WordDurationsMs: durs}, nil
				}
			}
		}
	}
	// Fall back to fresh synthesis (cache may still be warm from Precache)
	return s.SpeakAIContent(content)
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
