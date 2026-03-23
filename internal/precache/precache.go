// Package precache pre-synthesises TTS audio for every chapter in an offline
// Bible translation and stores results in the TTS audio cache.  It is designed
// to run entirely in the background after an import so the user never waits for
// synthesis at playback time.
package precache

import (
	"context"
	"fmt"
	"time"

	localdb "github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/tts"
)

// Progress reports the state of a running pre-cache job.
type Progress struct {
	TranslationID string
	BookName      string
	ChapterID     string
	Done          int
	Total         int
	Err           error
	Finished      bool
}

// Job pre-caches all chapters of a single translation.
type Job struct {
	db     *localdb.DB
	engine *tts.Engine
	id     string // translation ID
	ch     chan Progress
	cancel context.CancelFunc
}

// NewJob creates a Job but does not start it yet.
func NewJob(db *localdb.DB, engine *tts.Engine, translationID string) *Job {
	return &Job{
		db:     db,
		engine: engine,
		id:     translationID,
		ch:     make(chan Progress, 32),
	}
}

// Progress returns the channel on which progress updates are sent.
// The channel is closed when the job finishes or is cancelled.
func (j *Job) Progress() <-chan Progress { return j.ch }

// Cancel stops the job.
func (j *Job) Cancel() {
	if j.cancel != nil {
		j.cancel()
	}
}

// Start launches the pre-cache job in a background goroutine.
func (j *Job) Start() {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	j.cancel = cancel
	go j.run(ctx)
}

func (j *Job) run(ctx context.Context) {
	defer close(j.ch)
	defer j.cancel()

	send := func(p Progress) {
		select {
		case j.ch <- p:
		default:
		}
	}

	if j.engine == nil || !j.engine.Available() {
		send(Progress{TranslationID: j.id, Err: fmt.Errorf("TTS engine not available"), Finished: true})
		return
	}

	// Collect all chapters across all books
	type chapterRef struct {
		bookName  string
		chapterID string
	}
	var chapters []chapterRef

	books, err := j.db.ListBooks(j.id)
	if err != nil {
		send(Progress{TranslationID: j.id, Err: err, Finished: true})
		return
	}
	for _, b := range books {
		chs, err := j.db.ListChapters(b.ID, j.id)
		if err != nil {
			continue
		}
		for _, c := range chs {
			chapters = append(chapters, chapterRef{bookName: b.Name, chapterID: c.ID})
		}
	}

	total := len(chapters)
	send(Progress{TranslationID: j.id, Total: total, Done: 0})

	cache := j.engine.Cache()

	for i, ref := range chapters {
		if ctx.Err() != nil {
			return
		}

		send(Progress{
			TranslationID: j.id,
			BookName:      ref.bookName,
			ChapterID:     ref.chapterID,
			Done:          i,
			Total:         total,
		})

		// Load chapter text from DB
		content, err := j.db.GetChapterContent(ref.chapterID, j.id)
		if err != nil || content == "" {
			continue
		}

		// Clean text for TTS
		clean := tts.CleanForTTS(content)
		words := tts.SplitWords(clean)
		if len(words) == 0 {
			continue
		}

		// Check if already cached — skip synthesis if so
		voice    := j.engine.ActiveVoice()
		cacheKey := tts.CacheKey(voice.Engine, voice.ID, 1.0, clean)
		if cache != nil {
			if _, _, ok := cache.Get(cacheKey); ok {
				send(Progress{
					TranslationID: j.id,
					BookName:      ref.bookName,
					ChapterID:     ref.chapterID,
					Done:          i + 1,
					Total:         total,
				})
				continue
			}
		}

		// Synthesise — SpeakSynced writes to cache as a side-effect
		synced, synthErr := j.engine.SpeakSynced(clean, words)
		if synthErr != nil {
			send(Progress{
				TranslationID: j.id,
				BookName:      ref.bookName,
				ChapterID:     ref.chapterID,
				Done:          i + 1,
				Total:         total,
				Err:           fmt.Errorf("chapter %s: %w", ref.chapterID, synthErr),
			})
			continue
		}

		// Stop audio immediately — we only want the cache write, not playback
		if synced != nil {
			j.engine.Stop()
		}

		// Small courtesy pause between chapters to avoid hammering the CPU
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return
		}

		send(Progress{
			TranslationID: j.id,
			BookName:      ref.bookName,
			ChapterID:     ref.chapterID,
			Done:          i + 1,
			Total:         total,
		})
	}

	send(Progress{
		TranslationID: j.id,
		Done:          total,
		Total:         total,
		Finished:      true,
	})
}
