package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const baseURL = "https://api.scripture.api.bible/v1"

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// Client is a caching HTTP client for the API.Bible REST API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	cache      map[string]cacheEntry
	mu         sync.RWMutex
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		cache: make(map[string]cacheEntry),
	}
}

func (c *Client) get(path string, params map[string]string) ([]byte, error) {
	u := baseURL + path
	if len(params) > 0 {
		v := url.Values{}
		for k, val := range params {
			v.Set(k, val)
		}
		u += "?" + v.Encode()
	}

	c.mu.RLock()
	entry, ok := c.cache[u]
	c.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.data, nil
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[u] = cacheEntry{data: data, expiresAt: time.Now().Add(10 * time.Minute)}
	c.mu.Unlock()

	return data, nil
}

func (c *Client) GetBibles(language string) ([]Bible, error) {
	params := map[string]string{}
	if language != "" {
		params["language"] = language
	}
	data, err := c.get("/bibles", params)
	if err != nil {
		return nil, err
	}
	var r BiblesResponse
	return r.Data, json.Unmarshal(data, &r)
}

func (c *Client) GetBooks(bibleID string) ([]Book, error) {
	data, err := c.get(fmt.Sprintf("/bibles/%s/books", bibleID), nil)
	if err != nil {
		return nil, err
	}
	var r BooksResponse
	return r.Data, json.Unmarshal(data, &r)
}

func (c *Client) GetChapters(bibleID, bookID string) ([]Chapter, error) {
	data, err := c.get(fmt.Sprintf("/bibles/%s/books/%s/chapters", bibleID, bookID), nil)
	if err != nil {
		return nil, err
	}
	var r ChaptersResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	// Filter out intro chapters
	var out []Chapter
	for _, ch := range r.Data {
		if ch.Number != "intro" {
			out = append(out, ch)
		}
	}
	return out, nil
}

func (c *Client) GetChapter(bibleID, chapterID string) (ChapterContent, error) {
	params := map[string]string{
		"content-type":            "text",
		"include-verse-numbers":   "true",
		"include-chapter-numbers": "false",
		"include-titles":          "true",
	}
	data, err := c.get(fmt.Sprintf("/bibles/%s/chapters/%s", bibleID, chapterID), params)
	if err != nil {
		return ChapterContent{}, err
	}
	var r ChapterContentResponse
	return r.Data, json.Unmarshal(data, &r)
}

func (c *Client) GetVerse(bibleID, verseID string) (VerseContent, error) {
	params := map[string]string{
		"content-type":          "text",
		"include-verse-numbers": "true",
	}
	data, err := c.get(fmt.Sprintf("/bibles/%s/verses/%s", bibleID, verseID), params)
	if err != nil {
		return VerseContent{}, err
	}
	var r VerseContentResponse
	return r.Data, json.Unmarshal(data, &r)
}

func (c *Client) Search(bibleID, query string, limit int) (SearchData, error) {
	if limit <= 0 {
		limit = 20
	}
	params := map[string]string{
		"query": query,
		"limit": fmt.Sprintf("%d", limit),
	}
	data, err := c.get(fmt.Sprintf("/bibles/%s/search", bibleID), params)
	if err != nil {
		return SearchData{}, err
	}
	var r SearchResponse
	return r.Data, json.Unmarshal(data, &r)
}
