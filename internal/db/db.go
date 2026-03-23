package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultDBPath returns ~/.local/share/logos/bibles.db
func DefaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "logos", "bibles.db")
}

// DB wraps a SQLite connection with Bible-specific helpers.
type DB struct {
	sql *sql.DB
}

// Open opens (or creates) the Bible SQLite database at path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	d := &DB{sql: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

// Close closes the database connection.
func (d *DB) Close() error { return d.sql.Close() }

// ── Schema ────────────────────────────────────────────────────────────────────

const schema = `
-- Translations / versions (e.g. KJV, NIV, ESV)
CREATE TABLE IF NOT EXISTS translations (
    id          TEXT PRIMARY KEY,   -- e.g. "kjv-local", "de4e12af7f28f599-02"
    name        TEXT NOT NULL,      -- "King James Version"
    abbreviation TEXT NOT NULL,     -- "KJV"
    language    TEXT NOT NULL DEFAULT 'eng',
    description TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT 'local', -- "local" | "api"
    copyright   TEXT NOT NULL DEFAULT '',
    imported_at TEXT NOT NULL
);

-- Books (Genesis, Exodus, ...)
CREATE TABLE IF NOT EXISTS books (
    id             TEXT NOT NULL,   -- "GEN"
    translation_id TEXT NOT NULL REFERENCES translations(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,   -- "Genesis"
    short_name     TEXT NOT NULL DEFAULT '',
    number         INTEGER NOT NULL DEFAULT 0,
    testament      TEXT NOT NULL DEFAULT '',  -- "OT" | "NT"
    PRIMARY KEY (id, translation_id)
);

-- Chapters
CREATE TABLE IF NOT EXISTS chapters (
    id             TEXT NOT NULL,   -- "GEN.1"
    book_id        TEXT NOT NULL,
    translation_id TEXT NOT NULL,
    number         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (id, translation_id),
    FOREIGN KEY (book_id, translation_id) REFERENCES books(id, translation_id) ON DELETE CASCADE
);

-- Verses
CREATE TABLE IF NOT EXISTS verses (
    id             TEXT NOT NULL,   -- "GEN.1.1"
    chapter_id     TEXT NOT NULL,
    book_id        TEXT NOT NULL,
    translation_id TEXT NOT NULL,
    number         INTEGER NOT NULL DEFAULT 0,
    text           TEXT NOT NULL,
    PRIMARY KEY (id, translation_id),
    FOREIGN KEY (chapter_id, translation_id) REFERENCES chapters(id, translation_id) ON DELETE CASCADE
);

-- Full-text search virtual table over verses
CREATE VIRTUAL TABLE IF NOT EXISTS verses_fts USING fts5(
    text,
    verse_id    UNINDEXED,
    chapter_id  UNINDEXED,
    book_id     UNINDEXED,
    translation_id UNINDEXED,
    content='verses',
    content_rowid='rowid'
);

-- FTS triggers to keep index in sync
CREATE TRIGGER IF NOT EXISTS verses_fts_insert AFTER INSERT ON verses BEGIN
    INSERT INTO verses_fts(rowid, text, verse_id, chapter_id, book_id, translation_id)
    VALUES (new.rowid, new.text, new.id, new.chapter_id, new.book_id, new.translation_id);
END;
CREATE TRIGGER IF NOT EXISTS verses_fts_delete AFTER DELETE ON verses BEGIN
    INSERT INTO verses_fts(verses_fts, rowid, text, verse_id, chapter_id, book_id, translation_id)
    VALUES ('delete', old.rowid, old.text, old.id, old.chapter_id, old.book_id, old.translation_id);
END;
CREATE TRIGGER IF NOT EXISTS verses_fts_update AFTER UPDATE ON verses BEGIN
    INSERT INTO verses_fts(verses_fts, rowid, text, verse_id, chapter_id, book_id, translation_id)
    VALUES ('delete', old.rowid, old.text, old.id, old.chapter_id, old.book_id, old.translation_id);
    INSERT INTO verses_fts(rowid, text, verse_id, chapter_id, book_id, translation_id)
    VALUES (new.rowid, new.text, new.id, new.chapter_id, new.book_id, new.translation_id);
END;

-- ── AI Content Tables ──────────────────────────────────────────────────────

-- AI-generated notes linked to a verse
CREATE TABLE IF NOT EXISTS ai_notes (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    verse_id       TEXT NOT NULL,
    translation_id TEXT NOT NULL,
    content        TEXT NOT NULL,
    ai_model       TEXT NOT NULL DEFAULT '',
    created_at     TEXT NOT NULL
);

-- Daily devotionals
CREATE TABLE IF NOT EXISTS devotionals (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL,
    verse_ref    TEXT NOT NULL,   -- e.g. "John 3:16"
    content      TEXT NOT NULL,
    theme        TEXT NOT NULL DEFAULT '',
    ai_model     TEXT NOT NULL DEFAULT '',
    audio_cached INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

-- Sermons
CREATE TABLE IF NOT EXISTS sermons (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL,
    scripture_ref TEXT NOT NULL,
    content      TEXT NOT NULL,
    ai_model     TEXT NOT NULL DEFAULT '',
    audio_cached INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT NOT NULL
);

-- Study plans (header)
CREATE TABLE IF NOT EXISTS study_plans (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    topic        TEXT NOT NULL DEFAULT '',
    weeks_count  INTEGER NOT NULL DEFAULT 4,
    ai_model     TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL
);

-- Study plan weekly entries
CREATE TABLE IF NOT EXISTS study_plan_weeks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id      INTEGER NOT NULL REFERENCES study_plans(id) ON DELETE CASCADE,
    week_number  INTEGER NOT NULL,
    theme        TEXT NOT NULL DEFAULT '',
    reading      TEXT NOT NULL DEFAULT '',
    verses_json  TEXT NOT NULL DEFAULT '[]',
    notes        TEXT NOT NULL DEFAULT ''
);

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY);
INSERT OR IGNORE INTO schema_version (version) VALUES (2);
`

func (d *DB) migrate() error {
	_, err := d.sql.Exec(schema)
	return err
}

// ── Translation CRUD ──────────────────────────────────────────────────────────

// Translation mirrors the translations table.
type Translation struct {
	ID           string
	Name         string
	Abbreviation string
	Language     string
	Description  string
	Source       string // "local" | "api"
	Copyright    string
	ImportedAt   time.Time
}

// UpsertTranslation inserts or replaces a translation record.
func (d *DB) UpsertTranslation(t Translation) error {
	_, err := d.sql.Exec(`
		INSERT INTO translations (id, name, abbreviation, language, description, source, copyright, imported_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			abbreviation=excluded.abbreviation,
			language=excluded.language,
			description=excluded.description,
			source=excluded.source,
			copyright=excluded.copyright,
			imported_at=excluded.imported_at`,
		t.ID, t.Name, t.Abbreviation, t.Language, t.Description, t.Source, t.Copyright,
		t.ImportedAt.Format(time.RFC3339),
	)
	return err
}

// ListTranslations returns all locally stored translations.
func (d *DB) ListTranslations() ([]Translation, error) {
	rows, err := d.sql.Query(
		`SELECT id, name, abbreviation, language, description, source, copyright, imported_at
		 FROM translations ORDER BY abbreviation ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Translation
	for rows.Next() {
		var t Translation
		var imp string
		if err := rows.Scan(&t.ID, &t.Name, &t.Abbreviation, &t.Language,
			&t.Description, &t.Source, &t.Copyright, &imp); err != nil {
			return nil, err
		}
		t.ImportedAt, _ = time.Parse(time.RFC3339, imp)
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTranslation removes a translation and all its books/chapters/verses.
func (d *DB) DeleteTranslation(id string) error {
	_, err := d.sql.Exec(`DELETE FROM translations WHERE id=?`, id)
	return err
}

// ── Book CRUD ─────────────────────────────────────────────────────────────────

// Book mirrors the books table.
type Book struct {
	ID            string
	TranslationID string
	Name          string
	ShortName     string
	Number        int
	Testament     string
}

// UpsertBook inserts or replaces a book record.
func (d *DB) UpsertBook(b Book) error {
	_, err := d.sql.Exec(`
		INSERT INTO books (id, translation_id, name, short_name, number, testament)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, translation_id) DO UPDATE SET
			name=excluded.name, short_name=excluded.short_name,
			number=excluded.number, testament=excluded.testament`,
		b.ID, b.TranslationID, b.Name, b.ShortName, b.Number, b.Testament,
	)
	return err
}

// ListBooks returns all books for a translation, ordered by number.
func (d *DB) ListBooks(translationID string) ([]Book, error) {
	rows, err := d.sql.Query(
		`SELECT id, translation_id, name, short_name, number, testament
		 FROM books WHERE translation_id=? ORDER BY number ASC`,
		translationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.TranslationID, &b.Name, &b.ShortName, &b.Number, &b.Testament); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ── Chapter CRUD ──────────────────────────────────────────────────────────────

// Chapter mirrors the chapters table.
type Chapter struct {
	ID            string
	BookID        string
	TranslationID string
	Number        int
}

// UpsertChapter inserts or replaces a chapter record.
func (d *DB) UpsertChapter(c Chapter) error {
	_, err := d.sql.Exec(`
		INSERT INTO chapters (id, book_id, translation_id, number)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id, translation_id) DO UPDATE SET number=excluded.number`,
		c.ID, c.BookID, c.TranslationID, c.Number,
	)
	return err
}

// ListChapters returns all chapters for a book.
func (d *DB) ListChapters(bookID, translationID string) ([]Chapter, error) {
	rows, err := d.sql.Query(
		`SELECT id, book_id, translation_id, number
		 FROM chapters WHERE book_id=? AND translation_id=? ORDER BY number ASC`,
		bookID, translationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chapter
	for rows.Next() {
		var c Chapter
		if err := rows.Scan(&c.ID, &c.BookID, &c.TranslationID, &c.Number); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ── Verse CRUD ────────────────────────────────────────────────────────────────

// Verse mirrors the verses table.
type Verse struct {
	ID            string
	ChapterID     string
	BookID        string
	TranslationID string
	Number        int
	Text          string
}

// UpsertVerse inserts or replaces a verse.
func (d *DB) UpsertVerse(v Verse) error {
	_, err := d.sql.Exec(`
		INSERT INTO verses (id, chapter_id, book_id, translation_id, number, text)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, translation_id) DO UPDATE SET text=excluded.text`,
		v.ID, v.ChapterID, v.BookID, v.TranslationID, v.Number, v.Text,
	)
	return err
}

// BulkUpsertVerses inserts many verses in a single transaction.
func (d *DB) BulkUpsertVerses(verses []Verse) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO verses (id, chapter_id, book_id, translation_id, number, text)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, translation_id) DO UPDATE SET text=excluded.text`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, v := range verses {
		if _, err := stmt.Exec(v.ID, v.ChapterID, v.BookID, v.TranslationID, v.Number, v.Text); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// GetChapterContent returns a pseudo-content string (same [N] format as API.Bible)
// for a chapter stored locally, suitable for rendering in the reader.
func (d *DB) GetChapterContent(chapterID, translationID string) (string, error) {
	rows, err := d.sql.Query(
		`SELECT number, text FROM verses
		 WHERE chapter_id=? AND translation_id=? ORDER BY number ASC`,
		chapterID, translationID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var content string
	for rows.Next() {
		var num int
		var text string
		if err := rows.Scan(&num, &text); err != nil {
			return "", err
		}
		content += fmt.Sprintf("[%d]%s ", num, text)
	}
	return content, rows.Err()
}

// ── Search ────────────────────────────────────────────────────────────────────

// SearchResult is one FTS hit.
type SearchResult struct {
	VerseID       string
	ChapterID     string
	BookID        string
	TranslationID string
	Text          string
}

// Search performs a full-text search across verses in the given translation.
func (d *DB) Search(translationID, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.sql.Query(`
		SELECT verse_id, chapter_id, book_id, translation_id, text
		FROM verses_fts
		WHERE verses_fts MATCH ? AND translation_id=?
		ORDER BY rank
		LIMIT ?`, query, translationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.VerseID, &r.ChapterID, &r.BookID, &r.TranslationID, &r.Text); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── Stats ─────────────────────────────────────────────────────────────────────

// TranslationStats holds row counts for a translation.
type TranslationStats struct {
	TranslationID string
	Books         int
	Chapters      int
	Verses        int
}

// GetStats returns import statistics for each local translation.
func (d *DB) GetStats() ([]TranslationStats, error) {
	rows, err := d.sql.Query(`
		SELECT t.id,
		       (SELECT COUNT(*) FROM books    WHERE translation_id=t.id),
		       (SELECT COUNT(*) FROM chapters WHERE translation_id=t.id),
		       (SELECT COUNT(*) FROM verses   WHERE translation_id=t.id)
		FROM translations t ORDER BY t.abbreviation`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TranslationStats
	for rows.Next() {
		var s TranslationStats
		if err := rows.Scan(&s.TranslationID, &s.Books, &s.Chapters, &s.Verses); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── AI Content CRUD ───────────────────────────────────────────────────────────

// AINote is a verse-linked AI-generated note.
type AINote struct {
	ID            int64
	VerseID       string
	TranslationID string
	Content       string
	AIModel       string
	CreatedAt     time.Time
}

// SaveNote stores an AI note and returns its new ID.
func (d *DB) SaveNote(n AINote) (int64, error) {
	res, err := d.sql.Exec(
		`INSERT INTO ai_notes (verse_id, translation_id, content, ai_model, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		n.VerseID, n.TranslationID, n.Content, n.AIModel,
		n.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListNotes returns AI notes for a verse.
func (d *DB) ListNotes(verseID, translationID string) ([]AINote, error) {
	rows, err := d.sql.Query(
		`SELECT id, verse_id, translation_id, content, ai_model, created_at
		 FROM ai_notes WHERE verse_id=? AND translation_id=? ORDER BY created_at DESC`,
		verseID, translationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AINote
	for rows.Next() {
		var n AINote
		var ts string
		if err := rows.Scan(&n.ID, &n.VerseID, &n.TranslationID, &n.Content, &n.AIModel, &ts); err != nil {
			return nil, err
		}
		n.CreatedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, n)
	}
	return out, rows.Err()
}

// Devotional is an AI-generated daily devotional.
type Devotional struct {
	ID          int64
	Title       string
	VerseRef    string
	Content     string
	Theme       string
	AIModel     string
	AudioCached bool
	CreatedAt   time.Time
}

// SaveDevotional stores a devotional and returns its new ID.
func (d *DB) SaveDevotional(dv Devotional) (int64, error) {
	res, err := d.sql.Exec(
		`INSERT INTO devotionals (title, verse_ref, content, theme, ai_model, audio_cached, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		dv.Title, dv.VerseRef, dv.Content, dv.Theme, dv.AIModel,
		boolInt(dv.AudioCached), dv.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListDevotionals returns all stored devotionals, most recent first.
func (d *DB) ListDevotionals(limit int) ([]Devotional, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.sql.Query(
		`SELECT id, title, verse_ref, content, theme, ai_model, audio_cached, created_at
		 FROM devotionals ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDevotionals(rows)
}

func scanDevotionals(rows *sql.Rows) ([]Devotional, error) {
	var out []Devotional
	for rows.Next() {
		var dv Devotional
		var ts string
		var ac int
		if err := rows.Scan(&dv.ID, &dv.Title, &dv.VerseRef, &dv.Content,
			&dv.Theme, &dv.AIModel, &ac, &ts); err != nil {
			return nil, err
		}
		dv.AudioCached = ac != 0
		dv.CreatedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, dv)
	}
	return out, rows.Err()
}

// Sermon is an AI-generated sermon.
type Sermon struct {
	ID           int64
	Title        string
	ScriptureRef string
	Content      string
	AIModel      string
	AudioCached  bool
	CreatedAt    time.Time
}

// SaveSermon stores a sermon and returns its new ID.
func (d *DB) SaveSermon(s Sermon) (int64, error) {
	res, err := d.sql.Exec(
		`INSERT INTO sermons (title, scripture_ref, content, ai_model, audio_cached, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.Title, s.ScriptureRef, s.Content, s.AIModel,
		boolInt(s.AudioCached), s.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListSermons returns all stored sermons, most recent first.
func (d *DB) ListSermons(limit int) ([]Sermon, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.sql.Query(
		`SELECT id, title, scripture_ref, content, ai_model, audio_cached, created_at
		 FROM sermons ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sermon
	for rows.Next() {
		var s Sermon
		var ts string
		var ac int
		if err := rows.Scan(&s.ID, &s.Title, &s.ScriptureRef, &s.Content, &s.AIModel, &ac, &ts); err != nil {
			return nil, err
		}
		s.AudioCached = ac != 0
		s.CreatedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, s)
	}
	return out, rows.Err()
}

// StudyPlanRecord is the header row for a study plan.
type StudyPlanRecord struct {
	ID          int64
	Title       string
	Description string
	Topic       string
	WeeksCount  int
	AIModel     string
	CreatedAt   time.Time
}

// StudyPlanWeekRecord is one week of a study plan.
type StudyPlanWeekRecord struct {
	ID         int64
	PlanID     int64
	WeekNumber int
	Theme      string
	Reading    string
	VersesJSON string
	Notes      string
}

// SaveStudyPlan stores a study plan with all its weeks.
func (d *DB) SaveStudyPlan(plan StudyPlanRecord, weeks []StudyPlanWeekRecord) (int64, error) {
	tx, err := d.sql.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(
		`INSERT INTO study_plans (title, description, topic, weeks_count, ai_model, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		plan.Title, plan.Description, plan.Topic, plan.WeeksCount, plan.AIModel,
		plan.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		tx.Rollback()
		return 0, err
	}
	planID, _ := res.LastInsertId()
	for _, w := range weeks {
		if _, err := tx.Exec(
			`INSERT INTO study_plan_weeks (plan_id, week_number, theme, reading, verses_json, notes)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			planID, w.WeekNumber, w.Theme, w.Reading, w.VersesJSON, w.Notes,
		); err != nil {
			tx.Rollback()
			return 0, err
		}
	}
	return planID, tx.Commit()
}

// ListStudyPlans returns all study plan headers.
func (d *DB) ListStudyPlans(limit int) ([]StudyPlanRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.sql.Query(
		`SELECT id, title, description, topic, weeks_count, ai_model, created_at
		 FROM study_plans ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StudyPlanRecord
	for rows.Next() {
		var p StudyPlanRecord
		var ts string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Topic, &p.WeeksCount, &p.AIModel, &ts); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, p)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
