// Package crawler — Ollama-powered fallback for generic Bible websites.
//
// When the heuristic crawler finds no verses and an ai.Client is available,
// this fallback strips the HTML to plain text and asks the Ollama model to
// extract verse numbers + text as JSON.
package crawler

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	gohtml "golang.org/x/net/html"

	"github.com/jd4rider/logos/internal/ai"
)

// htmlToText strips all HTML tags and collapses whitespace into readable plain text.
func htmlToText(body string) string {
	doc, err := gohtml.Parse(strings.NewReader(body))
	if err != nil {
		// Fallback: strip tags with regex
		r := regexp.MustCompile(`<[^>]+>`)
		return r.ReplaceAllString(body, " ")
	}

	var sb strings.Builder
	var walk func(*gohtml.Node)
	walk = func(n *gohtml.Node) {
		// Skip script / style / noscript content entirely
		if n.Type == gohtml.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "head", "nav", "footer", "aside":
				return
			}
		}
		if n.Type == gohtml.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				sb.WriteString(t)
				sb.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Collapse 3+ consecutive newlines to 2
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllString(sb.String(), "\n\n")
}

// extractChapterWithAI uses an Ollama model to extract verses when the
// heuristic parser found nothing. Returns the same types as extractChapter.
func extractChapterWithAI(aiClient *ai.Client, body, pageURL string, bookName string, chNum int) ([]rawVerse, error) {
	if aiClient == nil {
		return nil, fmt.Errorf("no AI client available")
	}

	// Convert HTML to plain text first (much shorter for the model)
	plainText := htmlToText(body)

	// If we don't know the book/chapter yet, ask the model to detect them too
	if bookName == "" || chNum == 0 {
		detected, err := detectBookChapterWithAI(aiClient, plainText)
		if err == nil {
			if bookName == "" {
				bookName = detected.book
			}
			if chNum == 0 {
				chNum = detected.chapter
			}
		}
	}

	if bookName == "" {
		return nil, fmt.Errorf("could not determine book name from page")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	chStr := strconv.Itoa(chNum)
	maps, err := aiClient.ExtractBibleFromHTML(ctx, plainText, bookName, chStr)
	if err != nil {
		return nil, fmt.Errorf("AI extraction: %w", err)
	}

	verses := make([]rawVerse, 0, len(maps))
	for _, m := range maps {
		vStr := m["verse"]
		text := strings.TrimSpace(m["text"])
		if text == "" {
			continue
		}
		var vNum int
		fmt.Sscanf(vStr, "%d", &vNum)
		if vNum == 0 {
			continue
		}
		verses = append(verses, rawVerse{Number: vNum, Text: text})
	}

	if len(verses) == 0 {
		return nil, fmt.Errorf("AI returned no parseable verses")
	}
	return verses, nil
}

type bookChapterHint struct {
	book    string
	chapter int
}

// detectBookChapterWithAI asks the model to identify the Bible book and chapter
// from page text. Used as a fallback when the heuristic title parser fails.
func detectBookChapterWithAI(aiClient *ai.Client, pageText string) (bookChapterHint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	systemPrompt := `You are a data extraction specialist for Bible content.
Return ONLY valid JSON — no explanation, no markdown, no backticks.`

	prompt := fmt.Sprintf(`Identify the Bible book name and chapter number from this web page text.
Return ONLY a JSON object like: {"book": "Genesis", "chapter": 1}

Page text (first 2000 chars):
%s`, truncateStr(pageText, 2000))

	full, err := aiClient.GenerateFull(ctx, systemPrompt, prompt, &ai.Options{Temperature: 0.0, NumPredict: 50})
	if err != nil {
		return bookChapterHint{}, err
	}

	// Extract JSON
	jsonStr := extractJSONStr(full)
	var result struct {
		Book    string `json:"book"`
		Chapter int    `json:"chapter"`
	}
	// Simple parse without full JSON lib
	bookMatch := regexp.MustCompile(`"book"\s*:\s*"([^"]+)"`).FindStringSubmatch(jsonStr)
	chapMatch := regexp.MustCompile(`"chapter"\s*:\s*(\d+)`).FindStringSubmatch(jsonStr)

	if len(bookMatch) > 1 {
		result.Book = bookMatch[1]
	}
	if len(chapMatch) > 1 {
		fmt.Sscanf(chapMatch[1], "%d", &result.Chapter)
	}

	if result.Book == "" {
		return bookChapterHint{}, fmt.Errorf("no book found in AI response: %s", full)
	}
	return bookChapterHint{book: result.Book, chapter: result.Chapter}, nil
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func extractJSONStr(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) > 1 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
	}
	// Find first { or [
	for i, c := range s {
		if c == '{' || c == '[' {
			return s[i:]
		}
	}
	return s
}
