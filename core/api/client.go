package api

import (
"encoding/json"
"fmt"
"net/http"
"net/url"
"strconv"
"sync"
"time"
)

const baseURL = "https://api.scripture.api.bible/v1"

type cacheEntry struct {
data      []byte
expiresAt time.Time
}

// Client is an HTTP client for the API.Bible service
type Client struct {
apiKey     string
httpClient *http.Client
cache      map[string]cacheEntry
cacheMu    sync.RWMutex
}

// NewClient creates a new API.Bible client
func NewClient(apiKey string) *Client {
return &Client{
apiKey: apiKey,
httpClient: &http.Client{
Timeout: 15 * time.Second,
},
cache: make(map[string]cacheEntry),
}
}

func (c *Client) get(path string, out interface{}) error {
c.cacheMu.RLock()
entry, ok := c.cache[path]
c.cacheMu.RUnlock()

if ok && time.Now().Before(entry.expiresAt) {
return json.Unmarshal(entry.data, out)
}

req, err := http.NewRequest("GET", baseURL+path, nil)
if err != nil {
return fmt.Errorf("creating request: %w", err)
}
req.Header.Set("api-key", c.apiKey)

resp, err := c.httpClient.Do(req)
if err != nil {
return fmt.Errorf("executing request: %w", err)
}
defer resp.Body.Close()

if resp.StatusCode != http.StatusOK {
return fmt.Errorf("API returned status %d for %s", resp.StatusCode, path)
}

if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
return fmt.Errorf("decoding response: %w", err)
}

// Cache the response
if b, err := json.Marshal(out); err == nil {
c.cacheMu.Lock()
c.cache[path] = cacheEntry{data: b, expiresAt: time.Now().Add(10 * time.Minute)}
c.cacheMu.Unlock()
}

return nil
}

// GetBibles returns available Bible translations, optionally filtered by language
func (c *Client) GetBibles(language string) ([]Bible, error) {
path := "/bibles"
if language != "" {
path += "?language=" + url.QueryEscape(language)
}
var resp BiblesResponse
if err := c.get(path, &resp); err != nil {
return nil, err
}
return resp.Data, nil
}

// GetBooks returns the books of the Bible for a given Bible ID
func (c *Client) GetBooks(bibleID string) ([]Book, error) {
var resp BooksResponse
if err := c.get(fmt.Sprintf("/bibles/%s/books", bibleID), &resp); err != nil {
return nil, err
}
return resp.Data, nil
}

// GetChapters returns chapters for a book, filtering out intro chapters
func (c *Client) GetChapters(bibleID, bookID string) ([]Chapter, error) {
var resp ChaptersResponse
if err := c.get(fmt.Sprintf("/bibles/%s/books/%s/chapters", bibleID, bookID), &resp); err != nil {
return nil, err
}
var chapters []Chapter
for _, ch := range resp.Data {
if ch.Number != "intro" {
chapters = append(chapters, ch)
}
}
return chapters, nil
}

// GetChapter returns the full content of a chapter
func (c *Client) GetChapter(bibleID, chapterID string) (ChapterContent, error) {
path := fmt.Sprintf("/bibles/%s/chapters/%s?content-type=text&include-verse-numbers=true&include-titles=true",
bibleID, chapterID)
var resp ChapterResponse
if err := c.get(path, &resp); err != nil {
return ChapterContent{}, err
}
return resp.Data, nil
}

// GetVerse returns the full content of a verse
func (c *Client) GetVerse(bibleID, verseID string) (VerseContent, error) {
path := fmt.Sprintf("/bibles/%s/verses/%s?content-type=text&include-verse-numbers=true",
bibleID, verseID)
var resp VerseResponse
if err := c.get(path, &resp); err != nil {
return VerseContent{}, err
}
return resp.Data, nil
}

// Search searches for verses matching the query
func (c *Client) Search(bibleID, query string, limit int) (SearchData, error) {
if limit <= 0 {
limit = 20
}
path := fmt.Sprintf("/bibles/%s/search?query=%s&limit=%s",
bibleID, url.QueryEscape(query), strconv.Itoa(limit))
var resp SearchResponse
if err := c.get(path, &resp); err != nil {
return SearchData{}, err
}
return resp.Data, nil
}
