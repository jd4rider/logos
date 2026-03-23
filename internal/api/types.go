package api

// ── Wrappers ────────────────────────────────────────────────────────────────

type BiblesResponse struct {
	Data []Bible `json:"data"`
}

type BibleResponse struct {
	Data Bible `json:"data"`
}

type BooksResponse struct {
	Data []Book `json:"data"`
}

type ChaptersResponse struct {
	Data []Chapter `json:"data"`
}

type ChapterContentResponse struct {
	Data ChapterContent `json:"data"`
}

type VerseContentResponse struct {
	Data VerseContent `json:"data"`
}

type SearchResponse struct {
	Data SearchData `json:"data"`
}

// ── Core types ───────────────────────────────────────────────────────────────

type Language struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	NameLocal       string `json:"nameLocal"`
	Script          string `json:"script"`
	ScriptDirection string `json:"scriptDirection"`
}

type Bible struct {
	ID                string   `json:"id"`
	DblID             string   `json:"dblId"`
	Abbreviation      string   `json:"abbreviation"`
	AbbreviationLocal string   `json:"abbreviationLocal"`
	Name              string   `json:"name"`
	NameLocal         string   `json:"nameLocal"`
	Description       string   `json:"description"`
	Language          Language `json:"language"`
	Type              string   `json:"type"`
}

type Book struct {
	ID           string `json:"id"`
	BibleID      string `json:"bibleId"`
	Abbreviation string `json:"abbreviation"`
	Name         string `json:"name"`
	NameLong     string `json:"nameLong"`
}

type Chapter struct {
	ID       string `json:"id"`
	BibleID  string `json:"bibleId"`
	BookID   string `json:"bookId"`
	Number   string `json:"number"`
	Position int    `json:"position"`
}

type ChapterRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	BookID string `json:"bookId"`
}

type ChapterContent struct {
	ID         string      `json:"id"`
	BibleID    string      `json:"bibleId"`
	BookID     string      `json:"bookId"`
	Number     string      `json:"number"`
	Reference  string      `json:"reference"`
	Content    string      `json:"content"`
	VerseCount int         `json:"verseCount"`
	Copyright  string      `json:"copyright"`
	Next       *ChapterRef `json:"next"`
	Previous   *ChapterRef `json:"previous"`
}

type VerseRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

type VerseContent struct {
	ID        string    `json:"id"`
	BookID    string    `json:"bookId"`
	ChapterID string    `json:"chapterId"`
	BibleID   string    `json:"bibleId"`
	Reference string    `json:"reference"`
	Content   string    `json:"content"`
	Copyright string    `json:"copyright"`
	Next      *VerseRef `json:"next"`
	Previous  *VerseRef `json:"previous"`
}

type SearchVerse struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	BookID    string `json:"bookId"`
	BibleID   string `json:"bibleId"`
	ChapterID string `json:"chapterId"`
	Reference string `json:"reference"`
	Text      string `json:"text"`
}

type SearchData struct {
	Query      string        `json:"query"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
	Total      int           `json:"total"`
	VerseCount int           `json:"verseCount"`
	Verses     []SearchVerse `json:"verses"`
}
