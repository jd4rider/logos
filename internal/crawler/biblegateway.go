package crawler

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	gohtml "golang.org/x/net/html"

	"github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/importer"
)

// IsBibleGatewayURL returns true if the URL points to biblegateway.com.
func IsBibleGatewayURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(u.Hostname()), "biblegateway.com")
}

// CrawlBibleGateway imports a full Bible translation from a BibleGateway URL.
//
// Accepts two URL forms:
//  1. Version/booklist page: https://www.biblegateway.com/versions/New-Life-Version-NLV-Bible/#booklist
//  2. Single chapter:        https://www.biblegateway.com/passage/?search=Genesis+1&version=NLV
//
// In case 1 it enumerates every chapter linked on the page.
// In case 2 it starts there and follows "next chapter" links.
func CrawlBibleGateway(bibleDB *db.DB, startURL string, opts Options) error {
	if opts.Delay == 0 {
		opts.Delay = 1200 * time.Millisecond // be polite to BG's servers
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "logos/1.0 (personal Bible reader)"
	}
	if opts.Language == "" {
		opts.Language = "eng"
	}

	client := &http.Client{Timeout: 30 * time.Second}

	// --- Step 1: resolve version code and chapter URLs ---
	versionCode, chapterURLs, err := resolveBGEntry(client, startURL, opts.UserAgent)
	if err != nil {
		return fmt.Errorf("resolve entry: %w", err)
	}

	abbr := opts.Abbreviation
	if abbr == "" {
		abbr = versionCode
	}
	name := opts.Name
	if name == "" {
		name = abbr
	}
	tid := opts.TranslationID
	if tid == "" {
		tid = "bg-" + strings.ToLower(abbr)
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

	opts.progress(fmt.Sprintf("Translation: %s (%s)  •  chapters to fetch: %d", name, abbr, len(chapterURLs)))

	bookTracker := &bookTracker{tid: tid, bibleDB: bibleDB}
	visited := map[string]bool{}
	chapterCount := 0

	// --- Step 2: crawl each chapter ---
	for _, chURL := range chapterURLs {
		if opts.MaxChapters > 0 && chapterCount >= opts.MaxChapters {
			break
		}
		clean := normalisePassageURL(chURL)
		if visited[clean] {
			continue
		}
		visited[clean] = true

		opts.progress(fmt.Sprintf("  fetching %s", clean))
		body, err := fetchPage(client, clean, opts.UserAgent)
		if err != nil {
			opts.progress(fmt.Sprintf("  warning: %v", err))
			continue
		}

		bookName, chNum, verses := parseBGChapterPage(body)
		if len(verses) == 0 || bookName == "" {
			opts.progress(fmt.Sprintf("  warning: no verses found at %s", clean))
			continue
		}

		bookID := importer.BookIDFromName(bookName)
		chapterID := fmt.Sprintf("%s.%d", bookID, chNum)

		// Skip if chapter already fully imported (resume mode)
		if opts.SkipExisting {
			if bibleDB.ChapterVerseCount(tid, chapterID) > 0 {
				opts.progress(fmt.Sprintf("  ↷ skip %s %d (already imported)", bookName, chNum))
				chapterCount++
				continue
			}
		}

		if err := bookTracker.ensure(bookID, bookName); err != nil {
			opts.progress(fmt.Sprintf("  warning: book upsert: %v", err))
		}

		_ = bibleDB.UpsertChapter(db.Chapter{
			ID: chapterID, BookID: bookID, TranslationID: tid, Number: chNum,
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
			opts.progress(fmt.Sprintf("  warning: insert: %v", err))
		} else {
			opts.progress(fmt.Sprintf("  ✓ %s %d (%d verses)", bookName, chNum, len(verses)))
			chapterCount++
		}

		time.Sleep(opts.Delay)
	}

	opts.progress(fmt.Sprintf("✓ Done: %d chapters imported into '%s'", chapterCount, tid))
	return nil
}

// ── Entry resolution ──────────────────────────────────────────────────────────

var bgVersionFromURL = regexp.MustCompile(`[/_-]([A-Z]{2,8})-Bible`)
var bgVersionFromParam = regexp.MustCompile(`[?&]version=([A-Z0-9]+)`)

// resolveBGEntry: if given a booklist page, extract all chapter URLs.
// If given a passage page, return it as the sole seed and follow next-links at crawl time.
func resolveBGEntry(client *http.Client, rawURL, ua string) (versionCode string, chapterURLs []string, err error) {
	u, _ := url.Parse(rawURL)

	// Detect version code from URL
	if m := bgVersionFromURL.FindStringSubmatch(rawURL); m != nil {
		versionCode = m[1]
	} else if m := bgVersionFromParam.FindStringSubmatch(rawURL); m != nil {
		versionCode = m[1]
	}

	// Is this a /versions/ page (booklist)?
	if strings.Contains(u.Path, "/versions/") {
		body, err := fetchPage(client, rawURL, ua)
		if err != nil {
			return versionCode, nil, err
		}
		chapterURLs = extractBGBooklistLinks(body, u)
		if len(chapterURLs) == 0 {
			return versionCode, nil, fmt.Errorf("no chapter links found on booklist page")
		}
		return versionCode, chapterURLs, nil
	}

	// It's a passage page — return as seed; CrawlBibleGateway will follow next links.
	// We need to fetch all chapter links via next-chapter navigation instead.
	// To handle this, we seed with this URL and then follow next-chapter links.
	chapterURLs = []string{rawURL}
	// Expand by following next links right now so we have the full list
	expanded, err := expandBGNextLinks(client, rawURL, ua)
	if err == nil && len(expanded) > 0 {
		chapterURLs = expanded
	}
	return versionCode, chapterURLs, nil
}

// extractBGBooklistLinks pulls all /passage/?search=... links from a BG versions page.
func extractBGBooklistLinks(body string, base *url.URL) []string {
	seen := map[string]bool{}
	var out []string

	re := regexp.MustCompile(`/passage/\?search=([^"&]+)&(?:amp;)?version=([A-Z0-9]+)`)
	matches := re.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		search := strings.ReplaceAll(m[1], "%20", "+")
		version := m[2]
		ref := fmt.Sprintf("https://www.biblegateway.com/passage/?search=%s&version=%s", search, version)
		norm := normalisePassageURL(ref)
		if !seen[norm] {
			seen[norm] = true
			out = append(out, ref)
		}
	}
	_ = base
	return out
}

// expandBGNextLinks follows next-chapter links from a starting passage page and
// returns all discovered chapter URLs in order.
func expandBGNextLinks(client *http.Client, startURL, ua string) ([]string, error) {
	var all []string
	visited := map[string]bool{}
	current := startURL
	for current != "" && len(all) < 1200 {
		norm := normalisePassageURL(current)
		if visited[norm] {
			break
		}
		visited[norm] = true
		all = append(all, current)

		body, err := fetchPage(client, current, ua)
		if err != nil {
			break
		}
		next := findBGNextLink(body, current)
		current = next
		if current != "" {
			time.Sleep(300 * time.Millisecond)
		}
	}
	return all, nil
}

// ── Chapter page parsing ──────────────────────────────────────────────────────

var (
	bgVerseClassRe  = regexp.MustCompile(`class="text ([A-Za-z0-9]+)-(\d+)-(\d+)"`)
	bgVerseSup      = regexp.MustCompile(`<sup[^>]*class="[^"]*versenum[^"]*"[^>]*>.*?</sup>`)
	bgHeadingBlock  = regexp.MustCompile(`(?s)<h[2-6][^>]*>.*?</h[2-6]>`)
	bgHTMLTag       = regexp.MustCompile(`<[^>]+>`)
	bgMultiSpace    = regexp.MustCompile(`\s{2,}`)
)

// parseBGChapterPage extracts (bookName, chapterNum, verses) from a BG passage page.
func parseBGChapterPage(body string) (bookName string, chNum int, verses []rawVerse) {
	// Extract book/chapter from the h1 title
	doc, err := gohtml.Parse(strings.NewReader(body))
	if err != nil {
		return
	}

	// Walk to find h1 with book+chapter
	var walkH1 func(*gohtml.Node)
	walkH1 = func(n *gohtml.Node) {
		if n.Type == gohtml.ElementNode && n.Data == "h1" {
			text := nodeText(n)
			if b, c := parseTitle(text); b != "" && c > 0 {
				bookName = b
				chNum = c
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkH1(c)
		}
	}
	walkH1(doc)

	// Extract verses using class="text {Book}-{Ch}-{V}" pattern
	seen := map[int]bool{}

	// Find all spans with verse class — operate on raw HTML for reliability
	verseMatches := bgVerseClassRe.FindAllStringSubmatchIndex(body, -1)
	for _, idx := range verseMatches {
		bookCode := body[idx[2]:idx[3]]
		// chStr := body[idx[4]:idx[5]]
		vStr := body[idx[6]:idx[7]]

		var vNum int
		fmt.Sscanf(vStr, "%d", &vNum)
		if vNum == 0 || seen[vNum] {
			continue
		}

		// Extract text of this span
		spanStart := strings.LastIndex(body[:idx[0]], "<span")
		if spanStart < 0 {
			spanStart = idx[0]
		}
		// Find matching closing </span>
		spanEnd := findSpanEnd(body, idx[0])
		if spanEnd <= idx[0] {
			continue
		}

		spanHTML := body[spanStart:spanEnd]
		// Remove verse-number sups
		spanHTML = bgVerseSup.ReplaceAllString(spanHTML, "")
		// Remove section headings (h2–h6) including their text content
		spanHTML = bgHeadingBlock.ReplaceAllString(spanHTML, " ")
		// Remove all HTML tags
		text := bgHTMLTag.ReplaceAllString(spanHTML, " ")
		text = html.UnescapeString(text)
		text = bgMultiSpace.ReplaceAllString(text, " ")
		text = strings.TrimSpace(text)

		if text == "" || len(text) < 2 {
			continue
		}
		// Skip if text is just the book name (heading nodes)
		if strings.ToUpper(text) == strings.ToUpper(bookCode) {
			continue
		}

		seen[vNum] = true
		verses = append(verses, rawVerse{Number: vNum, Text: text})
	}

	// Sort by verse number
	for i := 1; i < len(verses); i++ {
		for j := i; j > 0 && verses[j].Number < verses[j-1].Number; j-- {
			verses[j], verses[j-1] = verses[j-1], verses[j]
		}
	}

	return
}

// findSpanEnd finds the end position of a <span> tag starting at/after pos.
func findSpanEnd(body string, pos int) int {
	depth := 0
	i := pos
	for i < len(body) {
		open := strings.Index(body[i:], "<span")
		close := strings.Index(body[i:], "</span>")
		if close < 0 {
			return -1
		}
		if open >= 0 && open < close {
			depth++
			i += open + 5
		} else {
			if depth == 0 {
				return i + close + 7
			}
			depth--
			i += close + 7
		}
	}
	return -1
}

// ── Navigation ────────────────────────────────────────────────────────────────

var bgNextLinkRe = regexp.MustCompile(`<a[^>]+class="[^"]*next[^"]*"[^>]*href="([^"]+)"`)

func findBGNextLink(body, currentURL string) string {
	matches := bgNextLinkRe.FindStringSubmatch(body)
	if matches == nil {
		return ""
	}
	href := html.UnescapeString(matches[1])
	base, _ := url.Parse(currentURL)
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref).String()
	if normalisePassageURL(resolved) == normalisePassageURL(currentURL) {
		return ""
	}
	return resolved
}

// normalisePassageURL strips fragment and sorts query params for deduplication.
func normalisePassageURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	u.Fragment = ""
	q := u.Query()
	u.RawQuery = q.Encode()
	return strings.ToLower(u.String())
}
