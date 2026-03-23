// Package importer provides pluggable importers for loading Bible translations
// into the local SQLite database from various file formats.
package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/db"
)

// Progress is called periodically during import with status updates.
type Progress func(msg string)

// ImportOptions configures an import operation.
type ImportOptions struct {
	TranslationID   string   // override auto-detected ID
	Name            string   // override auto-detected name
	Abbreviation    string   // e.g. "KJV"
	Language        string   // e.g. "eng"
	Progress        Progress // optional callback
}

func (o *ImportOptions) progress(msg string) {
	if o.Progress != nil {
		o.Progress(msg)
	}
}

// ── CSV Importer ──────────────────────────────────────────────────────────────
//
// Accepts any CSV where columns are: book, chapter, verse, text
// (column positions are auto-detected by header; positional fallback = 0,1,2,3).
//
// Common open-data CSVs supported:
//   - Scrollmapper/bible_databases: b,c,v,t
//   - OpenBible.info:               book,chapter,verse,text
//   - Custom exports

// ImportCSV imports a CSV file into the database.
func ImportCSV(bibleDB *db.DB, path string, opts ImportOptions) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	return ImportCSVReader(bibleDB, f, path, opts)
}

// ImportCSVReader imports from an io.Reader (for testing or stdin).
func ImportCSVReader(bibleDB *db.DB, r io.Reader, sourceName string, opts ImportOptions) error {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	headers, err := cr.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	// Detect column positions
	bCol, cCol, vCol, tCol := 0, 1, 2, 3
	for i, h := range headers {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "b", "book", "book_id", "book_num":
			bCol = i
		case "c", "chapter", "chapter_num":
			cCol = i
		case "v", "verse", "verse_num":
			vCol = i
		case "t", "text", "verse_text", "scripture":
			tCol = i
		}
	}

	// Build translation metadata
	name := opts.Name
	if name == "" {
		base := filepath.Base(sourceName)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	abbr := opts.Abbreviation
	if abbr == "" {
		abbr = strings.ToUpper(name)
		if len(abbr) > 8 {
			abbr = abbr[:8]
		}
	}
	lang := opts.Language
	if lang == "" {
		lang = "eng"
	}
	tid := opts.TranslationID
	if tid == "" {
		tid = "local-" + strings.ToLower(abbr)
	}

	if err := bibleDB.UpsertTranslation(db.Translation{
		ID:           tid,
		Name:         name,
		Abbreviation: abbr,
		Language:     lang,
		Source:       "local",
		ImportedAt:   time.Now(),
	}); err != nil {
		return fmt.Errorf("upsert translation: %w", err)
	}

	// canonicalBookID converts a book identifier to uppercase 3-letter code.
	// Handles numeric (1=GEN), short-name ("Gen"), and full name ("Genesis").
	bookMap := buildBookMap()
	canonBook := func(raw string) string {
		raw = strings.TrimSpace(raw)
		if n, err := strconv.Atoi(raw); err == nil {
			if n >= 1 && n <= len(bookOrder) {
				return bookOrder[n-1]
			}
		}
		up := strings.ToUpper(raw)
		if len(up) >= 3 {
			candidate := up[:3]
			if _, ok := bookMap[candidate]; ok {
				return candidate
			}
		}
		if id, ok := bookMap[up]; ok {
			return id
		}
		return strings.ToUpper(raw)
	}

	var verses []db.Verse
	seenBooks := map[string]bool{}
	seenChapters := map[string]bool{}
	rowCount := 0

	flush := func() error {
		if len(verses) == 0 {
			return nil
		}
		if err := bibleDB.BulkUpsertVerses(verses); err != nil {
			return err
		}
		opts.progress(fmt.Sprintf("  imported %d verses…", rowCount))
		verses = verses[:0]
		return nil
	}

	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			opts.progress(fmt.Sprintf("  warning: skipped malformed row: %v", err))
			continue
		}
		if len(row) <= tCol {
			continue
		}

		bookRaw := row[bCol]
		chRaw := strings.TrimSpace(row[cCol])
		vRaw := strings.TrimSpace(row[vCol])
		text := strings.TrimSpace(row[tCol])
		if text == "" {
			continue
		}

		bookID := canonBook(bookRaw)
		chNum, _ := strconv.Atoi(chRaw)
		vNum, _ := strconv.Atoi(vRaw)

		if !seenBooks[bookID] {
			bName, _ := bookMap[bookID]
			if bName == "" {
				bName = bookID
			}
			bNum := bookNumber(bookID)
			testament := "OT"
			if bNum > 39 {
				testament = "NT"
			}
			_ = bibleDB.UpsertBook(db.Book{
				ID:            bookID,
				TranslationID: tid,
				Name:          bName,
				ShortName:     bookID,
				Number:        bNum,
				Testament:     testament,
			})
			seenBooks[bookID] = true
		}

		chapterID := fmt.Sprintf("%s.%d", bookID, chNum)
		if !seenChapters[chapterID] {
			_ = bibleDB.UpsertChapter(db.Chapter{
				ID:            chapterID,
				BookID:        bookID,
				TranslationID: tid,
				Number:        chNum,
			})
			seenChapters[chapterID] = true
		}

		verses = append(verses, db.Verse{
			ID:            fmt.Sprintf("%s.%d.%d", bookID, chNum, vNum),
			ChapterID:     chapterID,
			BookID:        bookID,
			TranslationID: tid,
			Number:        vNum,
			Text:          text,
		})
		rowCount++

		if len(verses) >= 500 {
			if err := flush(); err != nil {
				return fmt.Errorf("flush: %w", err)
			}
		}
	}

	if err := flush(); err != nil {
		return fmt.Errorf("final flush: %w", err)
	}
	opts.progress(fmt.Sprintf("✓ Imported %d verses into '%s' (%s)", rowCount, name, tid))
	return nil
}

// ── SQLite Importer ───────────────────────────────────────────────────────────
//
// Imports from another SQLite database. Tries common open-source schemas:
//   - Scrollmapper (b INTEGER, c INTEGER, v INTEGER, t TEXT)
//   - BibleSuperSearch (book_id, chapter_id, verse_id, verse_text)
//   - Our own internal schema (copies from one db to another)

// ImportSQLiteFile imports verses from a foreign SQLite file.
func ImportSQLiteFile(bibleDB *db.DB, path string, opts ImportOptions) error {
	srcDB, err := db.Open(path)
	if err != nil {
		return fmt.Errorf("open source db: %w", err)
	}
	defer srcDB.Close()

	// Try to import from our own schema first (translation-aware copy)
	if err := importFromOwnSchema(bibleDB, srcDB, opts); err == nil {
		return nil
	}

	// Fall back to raw CSV-style reader from the SQLite file
	return importFromScrollmapper(bibleDB, path, opts)
}

// importFromOwnSchema copies translations from another logos SQLite db.
func importFromOwnSchema(dst, src *db.DB, opts ImportOptions) error {
	translations, err := src.ListTranslations()
	if err != nil {
		return err
	}
	if len(translations) == 0 {
		return fmt.Errorf("no translations in source db")
	}
	for _, t := range translations {
		if opts.Abbreviation != "" {
			t.Abbreviation = opts.Abbreviation
		}
		if opts.TranslationID != "" {
			t.ID = opts.TranslationID
		}
		t.Source = "local"
		t.ImportedAt = time.Now()

		if err := dst.UpsertTranslation(t); err != nil {
			return err
		}

		books, _ := src.ListBooks(t.ID)
		for _, b := range books {
			_ = dst.UpsertBook(b)
			chapters, _ := src.ListChapters(b.ID, t.ID)
			for _, c := range chapters {
				_ = dst.UpsertChapter(c)
			}
		}

		opts.progress(fmt.Sprintf("  copying %s…", t.Abbreviation))
	}
	return nil
}

// importFromScrollmapper handles the Scrollmapper numeric format:
// Table "t_kjv" (or similar), columns: b, c, v, t
func importFromScrollmapper(bibleDB *db.DB, path string, opts ImportOptions) error {
	rawDB, err := openRawSQLite(path)
	if err != nil {
		return err
	}
	defer rawDB.Close()

	// Detect table name
	table, cols, err := detectScrollmapperTable(rawDB)
	if err != nil {
		return fmt.Errorf("detect schema: %w", err)
	}

	name := opts.Name
	if name == "" {
		name = strings.ToUpper(strings.TrimPrefix(table, "t_"))
	}
	abbr := opts.Abbreviation
	if abbr == "" {
		abbr = name
	}
	tid := opts.TranslationID
	if tid == "" {
		tid = "local-" + strings.ToLower(abbr)
	}

	if err := bibleDB.UpsertTranslation(db.Translation{
		ID:           tid,
		Name:         name,
		Abbreviation: abbr,
		Language:     opts.Language,
		Source:       "local",
		ImportedAt:   time.Now(),
	}); err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT %s, %s, %s, %s FROM %s ORDER BY %s, %s, %s",
		cols[0], cols[1], cols[2], cols[3], table, cols[0], cols[1], cols[2])
	rows, err := rawDB.Query(query)
	if err != nil {
		return fmt.Errorf("query source: %w", err)
	}
	defer rows.Close()

	var verses []db.Verse
	seenBooks := map[string]bool{}
	seenChapters := map[string]bool{}
	rowCount := 0
	bookMap := buildBookMap()

	for rows.Next() {
		var b, c, v int
		var t string
		if err := rows.Scan(&b, &c, &v, &t); err != nil {
			continue
		}
		if b < 1 || b > len(bookOrder) {
			continue
		}
		bookID := bookOrder[b-1]

		if !seenBooks[bookID] {
			bName, _ := bookMap[bookID]
			bNum := b
			testament := "OT"
			if bNum > 39 {
				testament = "NT"
			}
			_ = bibleDB.UpsertBook(db.Book{
				ID: bookID, TranslationID: tid, Name: bName,
				ShortName: bookID, Number: bNum, Testament: testament,
			})
			seenBooks[bookID] = true
		}
		chapterID := fmt.Sprintf("%s.%d", bookID, c)
		if !seenChapters[chapterID] {
			_ = bibleDB.UpsertChapter(db.Chapter{
				ID: chapterID, BookID: bookID, TranslationID: tid, Number: c,
			})
			seenChapters[chapterID] = true
		}
		verses = append(verses, db.Verse{
			ID:            fmt.Sprintf("%s.%d.%d", bookID, c, v),
			ChapterID:     chapterID, BookID: bookID, TranslationID: tid,
			Number: v, Text: t,
		})
		rowCount++
		if len(verses) >= 500 {
			_ = bibleDB.BulkUpsertVerses(verses)
			opts.progress(fmt.Sprintf("  imported %d verses…", rowCount))
			verses = verses[:0]
		}
	}
	_ = bibleDB.BulkUpsertVerses(verses)
	opts.progress(fmt.Sprintf("✓ Imported %d verses into '%s'", rowCount, name))
	return nil
}
