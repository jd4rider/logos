package api

// Language represents a language in the API.Bible system
type Language struct {
ID              string `json:"id"`
Name            string `json:"name"`
NameLocal       string `json:"nameLocal"`
Script          string `json:"script"`
ScriptDirection string `json:"scriptDirection"`
}

// Bible represents a Bible translation
type Bible struct {
ID           string   `json:"id"`
Abbreviation string   `json:"abbreviation"`
Name         string   `json:"name"`
NameLocal    string   `json:"nameLocal"`
Description  string   `json:"description"`
Language     Language `json:"language"`
Type         string   `json:"type"`
}

// Book represents a book of the Bible
type Book struct {
ID           string `json:"id"`
BibleID      string `json:"bibleId"`
Abbreviation string `json:"abbreviation"`
Name         string `json:"name"`
NameLong     string `json:"nameLong"`
}

// Chapter represents a chapter reference
type Chapter struct {
ID       string `json:"id"`
BibleID  string `json:"bibleId"`
BookID   string `json:"bookId"`
Number   string `json:"number"`
Position int    `json:"position"`
}

// ChapterRef is a reference to an adjacent chapter
type ChapterRef struct {
ID     string `json:"id"`
Number string `json:"number"`
BookID string `json:"bookId"`
}

// ChapterContent is the full content of a chapter
type ChapterContent struct {
ID         string      `json:"id"`
BibleID    string      `json:"bibleId"`
BookID     string      `json:"bookId"`
Number     string      `json:"number"`
Reference  string      `json:"reference"`
Content    string      `json:"content"`
Copyright  string      `json:"copyright"`
VerseCount int         `json:"verseCount"`
Next       *ChapterRef `json:"next"`
Previous   *ChapterRef `json:"previous"`
}

// VerseRef is a reference to an adjacent verse
type VerseRef struct {
ID     string `json:"id"`
Number string `json:"number"`
}

// VerseContent is the full content of a verse
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

// SearchVerse represents a single search result
type SearchVerse struct {
ID        string `json:"id"`
OrgID     string `json:"orgId"`
BookID    string `json:"bookId"`
BibleID   string `json:"bibleId"`
ChapterID string `json:"chapterId"`
Reference string `json:"reference"`
Text      string `json:"text"`
}

// SearchData is the result of a search query
type SearchData struct {
Query      string        `json:"query"`
Limit      int           `json:"limit"`
Offset     int           `json:"offset"`
Total      int           `json:"total"`
VerseCount int           `json:"verseCount"`
Verses     []SearchVerse `json:"verses"`
}

// Response wrapper types
type BiblesResponse struct {
Data []Bible `json:"data"`
}

type BooksResponse struct {
Data []Book `json:"data"`
}

type ChaptersResponse struct {
Data []Chapter `json:"data"`
}

type ChapterResponse struct {
Data ChapterContent `json:"data"`
}

type VerseResponse struct {
Data VerseContent `json:"data"`
}

type SearchResponse struct {
Data SearchData `json:"data"`
}
