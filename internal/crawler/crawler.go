// Package crawler fetches Bible text from a website URL and imports it into
// the local SQLite database.
//
// Strategy:
//  1. Fetch the given URL and detect what kind of Bible site it is.
//  2. Walk chapter-by-chapter (following "next chapter" links or a sitemap).
//  3. Extract verse text using heuristics tuned for common Bible sites.
//  4. Write everything into the local DB via internal/db.
package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"

	"github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/importer"
)

// Options configures a crawl session.
type Options struct {
	TranslationID string
	Name          string   // translation name
	Abbreviation  string   // e.g. "KJV"
	Language      string   // e.g. "eng"
	MaxChapters   int      // 0 = unlimited
	Delay         time.Duration // politeness delay between requests (default 1s)
	SkipExisting  bool     // skip chapters that already have verses in the DB (resume)
	Progress      importer.Progress
	UserAgent     string
}

func (o *Options) progress(msg string) {
	if o.Progress != nil {
		o.Progress(msg)
	}
}

// Crawl fetches a Bible from startURL and imports it into bibleDB.
// If the URL is from biblegateway.com, uses the dedicated BibleGateway crawler.
// Otherwise follows "next chapter" links to walk through all chapters.
func Crawl(bibleDB *db.DB, startURL string, opts Options) error {
	if IsBibleGatewayURL(startURL) {
		return CrawlBibleGateway(bibleDB, startURL, opts)
	}
	if opts.Delay == 0 {
		opts.Delay = time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "logos/1.0 (local Bible reader)"
	}
	if opts.Language == "" {
		opts.Language = "eng"
	}

	base, err := url.Parse(startURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	abbr := opts.Abbreviation
	if abbr == "" {
		abbr = strings.ToUpper(base.Hostname())
		if len(abbr) > 8 {
			abbr = abbr[:8]
		}
	}
	name := opts.Name
	if name == "" {
		name = abbr
	}
	tid := opts.TranslationID
	if tid == "" {
		tid = "crawl-" + strings.ToLower(abbr)
	}

	if err := bibleDB.UpsertTranslation(db.Translation{
		ID:           tid,
		Name:         name,
		Abbreviation: abbr,
		Language:     opts.Language,
		Source:       "local",
		ImportedAt:   time.Now(),
	}); err != nil {
		return fmt.Errorf("upsert translation: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	visited := map[string]bool{}
	currentURL := startURL
	chapterCount := 0

	bookTracker := &bookTracker{tid: tid, bibleDB: bibleDB}

	for currentURL != "" {
		if visited[currentURL] {
			break
		}
		visited[currentURL] = true

		opts.progress(fmt.Sprintf("  fetching %s", currentURL))
		page, err := fetchPage(client, currentURL, opts.UserAgent)
		if err != nil {
			opts.progress(fmt.Sprintf("  warning: %v", err))
			break
		}

		verses, nextURL, bookName, chNum := extractChapter(page, currentURL, base)
		nextURL = resolveURL(base, nextURL)

		if len(verses) > 0 && bookName != "" {
			bookID := importer.BookIDFromName(bookName)
			if err := bookTracker.ensure(bookID, bookName); err != nil {
				opts.progress(fmt.Sprintf("  warning: book upsert: %v", err))
			}

			chapterID := fmt.Sprintf("%s.%d", bookID, chNum)
			_ = bibleDB.UpsertChapter(db.Chapter{
				ID:            chapterID,
				BookID:        bookID,
				TranslationID: tid,
				Number:        chNum,
			})

			dbVerses := make([]db.Verse, len(verses))
			for i, v := range verses {
				dbVerses[i] = db.Verse{
					ID:            fmt.Sprintf("%s.%d.%d", bookID, chNum, v.Number),
					ChapterID:     chapterID,
					BookID:        bookID,
					TranslationID: tid,
					Number:        v.Number,
					Text:          v.Text,
				}
			}
			if err := bibleDB.BulkUpsertVerses(dbVerses); err != nil {
				opts.progress(fmt.Sprintf("  warning: insert verses: %v", err))
			} else {
				opts.progress(fmt.Sprintf("  ✓ %s chapter %d (%d verses)", bookName, chNum, len(verses)))
				chapterCount++
			}
		}

		if opts.MaxChapters > 0 && chapterCount >= opts.MaxChapters {
			break
		}

		currentURL = nextURL
		if currentURL != "" {
			time.Sleep(opts.Delay)
		}
	}

	opts.progress(fmt.Sprintf("✓ Crawl complete: %d chapters imported into '%s'", chapterCount, tid))
	return nil
}

// ── Page fetching ─────────────────────────────────────────────────────────────

func fetchPage(client *http.Client, rawURL, ua string) (string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, rawURL)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	return string(body), err
}

// ── Chapter extraction ────────────────────────────────────────────────────────

type rawVerse struct {
	Number int
	Text   string
}

var (
	verseNumRe   = regexp.MustCompile(`^\s*(\d{1,3})[\s.:]\s*`)
	chapterInURL = regexp.MustCompile(`[/._-](\d{1,3})(?:[/._-]|$)`)
)

// extractChapter parses an HTML page to find:
//   - A slice of verse (number, text) pairs
//   - The URL of the next chapter
//   - The book name
//   - The chapter number
func extractChapter(body, pageURL string, base *url.URL) ([]rawVerse, string, string, int) {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, "", "", 0
	}

	var verses []rawVerse
	var nextHref string
	var bookName string
	chNum := 0

	// Walk the DOM
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "span", "p", "div":
				// Look for verse-numbered text blocks
				cls := attrVal(n, "class")
				id := attrVal(n, "id")
				if looksLikeVerseContainer(cls, id) {
					if v, ok := extractVerse(n); ok {
						verses = append(verses, v)
					}
				}
			case "a":
				// Look for "next chapter" / "next" links
				href := attrVal(n, "href")
				text := strings.ToLower(strings.TrimSpace(nodeText(n)))
				if (strings.Contains(text, "next") || strings.Contains(text, "»")) &&
					!strings.Contains(text, "previous") &&
					href != "" && href != "#" {
					nextHref = href
				}
			case "title":
				// Extract book/chapter from <title>
				t := strings.TrimSpace(nodeText(n))
				bookName, chNum = parseTitle(t)
			case "h1", "h2", "h3":
				if bookName == "" {
					t := strings.TrimSpace(nodeText(n))
					if b, c := parseTitle(t); b != "" {
						bookName, chNum = b, c
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Fallback: detect chapter number from URL if not found in page
	if chNum == 0 {
		if m := chapterInURL.FindStringSubmatch(pageURL); m != nil {
			fmt.Sscanf(m[1], "%d", &chNum)
		}
	}

	// If we couldn't find structured verse containers, try paragraph-level extraction
	if len(verses) == 0 {
		verses = extractVersesFromParagraphs(doc)
	}

	return verses, nextHref, bookName, chNum
}

// looksLikeVerseContainer returns true if the CSS class or id suggests this
// element holds a single Bible verse.
func looksLikeVerseContainer(class, id string) bool {
	haystack := strings.ToLower(class + " " + id)
	for _, kw := range []string{"verse", "v-", "bvs", "bible-verse", "scripture-verse"} {
		if strings.Contains(haystack, kw) {
			return true
		}
	}
	return false
}

// extractVerse pulls a verse number and text from a DOM node.
func extractVerse(n *html.Node) (rawVerse, bool) {
	// Try to find a child span/sup with just a number (verse number label)
	vNum := 0
	var textParts []string

	var scan func(*html.Node)
	scan = func(node *html.Node) {
		if node.Type == html.ElementNode {
			cls := strings.ToLower(attrVal(node, "class"))
			if vNum == 0 && (strings.Contains(cls, "num") ||
				strings.Contains(cls, "versenum") ||
				node.Data == "sup") {
				t := strings.TrimSpace(nodeText(node))
				fmt.Sscanf(t, "%d", &vNum)
				return
			}
		}
		if node.Type == html.TextNode {
			t := strings.TrimSpace(node.Data)
			if t != "" {
				textParts = append(textParts, t)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			scan(c)
		}
	}
	scan(n)

	text := strings.Join(textParts, " ")
	text = cleanVerseText(text)

	// Try extracting leading verse number from text if not found via class
	if vNum == 0 {
		if m := verseNumRe.FindStringSubmatchIndex(text); m != nil {
			fmt.Sscanf(text[m[2]:m[3]], "%d", &vNum)
			text = strings.TrimSpace(text[m[1]:])
		}
	}

	if vNum == 0 || text == "" {
		return rawVerse{}, false
	}
	return rawVerse{Number: vNum, Text: text}, true
}

// extractVersesFromParagraphs is a last-resort parser that looks for
// lines starting with a bold/sup verse number in plain paragraph text.
func extractVersesFromParagraphs(doc *html.Node) []rawVerse {
	var verses []rawVerse
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div") {
			text := nodeText(n)
			found := extractNumberedLines(text)
			verses = append(verses, found...)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return verses
}

func extractNumberedLines(text string) []rawVerse {
	var out []rawVerse
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if m := verseNumRe.FindStringSubmatchIndex(line); m != nil {
			var vNum int
			fmt.Sscanf(line[m[2]:m[3]], "%d", &vNum)
			verseText := cleanVerseText(strings.TrimSpace(line[m[1]:]))
			if vNum > 0 && len(verseText) > 10 {
				out = append(out, rawVerse{Number: vNum, Text: verseText})
			}
		}
	}
	return out
}

// ── Title parsing ─────────────────────────────────────────────────────────────

var titleRe = regexp.MustCompile(`(?i)([\w\s]+?)\s+(\d+)`)

// parseTitle extracts book name and chapter number from a page title like
// "Genesis 1 - Bible Gateway" or "John 3 | NIV Bible".
func parseTitle(title string) (string, int) {
	parts := strings.FieldsFunc(title, func(r rune) bool {
		return r == '-' || r == '|' || r == '–' || r == '—' || r == ':'
	})
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if m := titleRe.FindStringSubmatch(part); m != nil {
			bookName := strings.TrimSpace(m[1])
			var chNum int
			fmt.Sscanf(m[2], "%d", &chNum)
			if chNum > 0 && isLikelyBookName(bookName) {
				return bookName, chNum
			}
		}
	}
	return "", 0
}

func isLikelyBookName(s string) bool {
	words := strings.Fields(s)
	if len(words) == 0 || len(words) > 4 {
		return false
	}
	for _, w := range words {
		if len(w) > 0 && unicode.IsLetter(rune(w[0])) {
			return true
		}
	}
	return false
}

// ── DOM helpers ───────────────────────────────────────────────────────────────

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

var multiSpace = regexp.MustCompile(`\s{2,}`)

func cleanVerseText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return ' '
		}
		return r
	}, s)
	s = multiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func resolveURL(base *url.URL, href string) string {
	if href == "" {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

// ── bookTracker ───────────────────────────────────────────────────────────────

type bookTracker struct {
	tid     string
	bibleDB *db.DB
	seen    map[string]bool
}

func (bt *bookTracker) ensure(bookID, bookName string) error {
	if bt.seen == nil {
		bt.seen = map[string]bool{}
	}
	if bt.seen[bookID] {
		return nil
	}
	bt.seen[bookID] = true
	num := importer.BookNumber(bookID)
	testament := "OT"
	if num > 39 {
		testament = "NT"
	}
	return bt.bibleDB.UpsertBook(db.Book{
		ID:            bookID,
		TranslationID: bt.tid,
		Name:          bookName,
		ShortName:     bookID,
		Number:        num,
		Testament:     testament,
	})
}
