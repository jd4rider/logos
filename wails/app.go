package main

import (
	"context"

	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main Wails application struct
type App struct {
	ctx    context.Context
	client *api.Client
	ttsEng *tts.Engine
}

// NewApp creates a new App instance
func NewApp(client *api.Client, ttsEng *tts.Engine) *App {
	return &App{client: client, ttsEng: ttsEng}
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
	return a.client.GetBibles(language)
}

// GetBooks returns books for a Bible
func (a *App) GetBooks(bibleID string) ([]api.Book, error) {
	return a.client.GetBooks(bibleID)
}

// GetChapters returns chapters for a book
func (a *App) GetChapters(bibleID, bookID string) ([]api.Chapter, error) {
	return a.client.GetChapters(bibleID, bookID)
}

// GetChapter returns the full content of a chapter
func (a *App) GetChapter(bibleID, chapterID string) (api.ChapterContent, error) {
	return a.client.GetChapter(bibleID, chapterID)
}

// GetVerse returns the full content of a verse
func (a *App) GetVerse(bibleID, verseID string) (api.VerseContent, error) {
	return a.client.GetVerse(bibleID, verseID)
}

// Search searches the Bible
func (a *App) Search(bibleID, query string, limit int) (api.SearchData, error) {
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
