package importer

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// openRawSQLite opens any SQLite file for direct querying (not through our schema).
func openRawSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	return db, nil
}

// detectScrollmapperTable tries to find a Scrollmapper-style table and its columns.
// Returns (tableName, [bCol, cCol, vCol, tCol], error).
func detectScrollmapperTable(db *sql.DB) (string, [4]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		return "", [4]string{}, err
	}
	var tables []string
	for rows.Next() {
		var name string
		_ = rows.Scan(&name)
		tables = append(tables, name)
	}
	rows.Close()

	for _, table := range tables {
		cols, err := tableColumns(db, table)
		if err != nil {
			continue
		}
		if bc, cc, vc, tc, ok := detectNumericCols(cols); ok {
			return table, [4]string{bc, cc, vc, tc}, nil
		}
	}
	return "", [4]string{}, fmt.Errorf("no recognisable Bible table found (tried: %v)", tables)
}

func tableColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		_ = rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
		cols = append(cols, strings.ToLower(name))
	}
	return cols, nil
}

// detectNumericCols maps common column naming conventions.
func detectNumericCols(cols []string) (b, c, v, t string, ok bool) {
	colSet := map[string]string{}
	for _, name := range cols {
		colSet[name] = name
	}

	// Scrollmapper: b, c, v, t
	if _, ok1 := colSet["b"]; ok1 {
		if _, ok2 := colSet["t"]; ok2 {
			return "b", "c", "v", "t", true
		}
	}
	// BibleSuperSearch: book_id, chapter_id, verse_id, verse_text
	if _, ok1 := colSet["book_id"]; ok1 {
		if _, ok2 := colSet["verse_text"]; ok2 {
			return "book_id", "chapter_id", "verse_id", "verse_text", true
		}
	}
	// Generic: book, chapter, verse, text
	if _, ok1 := colSet["book"]; ok1 {
		if _, ok2 := colSet["text"]; ok2 {
			c2 := "chapter"
			if _, ok3 := colSet["chapter"]; !ok3 {
				c2 = "c"
			}
			v2 := "verse"
			if _, ok3 := colSet["verse"]; !ok3 {
				v2 = "v"
			}
			return "book", c2, v2, "text", true
		}
	}
	return "", "", "", "", false
}

// ── Book reference tables ─────────────────────────────────────────────────────

// bookOrder maps 1-based numeric book number to USFM book ID.
var bookOrder = []string{
	"GEN", "EXO", "LEV", "NUM", "DEU", "JOS", "JDG", "RUT", "1SA", "2SA",
	"1KI", "2KI", "1CH", "2CH", "EZR", "NEH", "EST", "JOB", "PSA", "PRO",
	"ECC", "SNG", "ISA", "JER", "LAM", "EZK", "DAN", "HOS", "JOL", "AMO",
	"OBA", "JON", "MIC", "NAM", "HAB", "ZEP", "HAG", "ZEC", "MAL",
	"MAT", "MRK", "LUK", "JHN", "ACT", "ROM", "1CO", "2CO", "GAL", "EPH",
	"PHP", "COL", "1TH", "2TH", "1TI", "2TI", "TIT", "PHM", "HEB", "JAS",
	"1PE", "2PE", "1JN", "2JN", "3JN", "JUD", "REV",
}

var bookNames = map[string]string{
	"GEN": "Genesis", "EXO": "Exodus", "LEV": "Leviticus", "NUM": "Numbers",
	"DEU": "Deuteronomy", "JOS": "Joshua", "JDG": "Judges", "RUT": "Ruth",
	"1SA": "1 Samuel", "2SA": "2 Samuel", "1KI": "1 Kings", "2KI": "2 Kings",
	"1CH": "1 Chronicles", "2CH": "2 Chronicles", "EZR": "Ezra", "NEH": "Nehemiah",
	"EST": "Esther", "JOB": "Job", "PSA": "Psalms", "PRO": "Proverbs",
	"ECC": "Ecclesiastes", "SNG": "Song of Solomon", "ISA": "Isaiah", "JER": "Jeremiah",
	"LAM": "Lamentations", "EZK": "Ezekiel", "DAN": "Daniel", "HOS": "Hosea",
	"JOL": "Joel", "AMO": "Amos", "OBA": "Obadiah", "JON": "Jonah",
	"MIC": "Micah", "NAM": "Nahum", "HAB": "Habakkuk", "ZEP": "Zephaniah",
	"HAG": "Haggai", "ZEC": "Zechariah", "MAL": "Malachi",
	"MAT": "Matthew", "MRK": "Mark", "LUK": "Luke", "JHN": "John",
	"ACT": "Acts", "ROM": "Romans", "1CO": "1 Corinthians", "2CO": "2 Corinthians",
	"GAL": "Galatians", "EPH": "Ephesians", "PHP": "Philippians", "COL": "Colossians",
	"1TH": "1 Thessalonians", "2TH": "2 Thessalonians", "1TI": "1 Timothy",
	"2TI": "2 Timothy", "TIT": "Titus", "PHM": "Philemon", "HEB": "Hebrews",
	"JAS": "James", "1PE": "1 Peter", "2PE": "2 Peter", "1JN": "1 John",
	"2JN": "2 John", "3JN": "3 John", "JUD": "Jude", "REV": "Revelation",
}

// buildBookMap builds a lookup from various name forms to (name, id) pairs.
func buildBookMap() map[string]string {
	m := make(map[string]string, len(bookNames)*3)
	for id, name := range bookNames {
		m[id] = name
		m[strings.ToUpper(name)] = name
		if len(id) >= 3 {
			m[id[:3]] = name
		}
	}
	return m
}

func bookNumber(id string) int {
	for i, b := range bookOrder {
		if b == id {
			return i + 1
		}
	}
	return 0
}

// BookIDFromName converts a full or short book name to a USFM ID ("Genesis" → "GEN").
func BookIDFromName(name string) string {
up := strings.ToUpper(strings.TrimSpace(name))
m := buildBookMap()
if fullName, ok := m[up]; ok {
// reverse-lookup the ID from the full name
for id, n := range bookNames {
if n == fullName {
return id
}
}
}
// Try prefix match
if len(up) >= 3 {
for id := range bookNames {
if strings.HasPrefix(id, up[:3]) {
return id
}
}
}
// Return uppercased as fallback
if len(up) >= 3 {
return up[:3]
}
return up
}

// BookNumber returns the 1-based ordinal of a book ID ("GEN" → 1).
func BookNumber(id string) int {
return bookNumber(id)
}
