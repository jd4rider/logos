package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ---- System prompts --------------------------------------------------------

const systemBibleScholar = `You are a knowledgeable Bible scholar and pastor with expertise in
theology, Greek/Hebrew languages, church history, and practical Christian living.
You give thoughtful, balanced, scripture-grounded answers.
Respond clearly and warmly. Use the translation provided unless asked otherwise.
Keep responses focused and practical unless the user asks for depth.`

const systemDevotional = `You are a gifted Christian devotional writer.
Write engaging, spiritually enriching devotionals that are:
- Rooted in scripture and historically accurate
- Practically applicable to daily life
- Warm, encouraging, and easy to understand
- Suitable for being read aloud (no markdown, no bullet points in main content)
Write in flowing paragraphs. Always end with a short prayer.`

const systemSermon = `You are an experienced pastor and sermon writer.
Write full sermons that include: an opening illustration, exposition of the scripture,
3 clear points with supporting verses, application, and a compelling close/call to action.
Write in natural spoken language — this will be read aloud.
Do not use markdown formatting. Use paragraph breaks only.`

const systemStudyPlan = `You are a Bible study curriculum designer.
Create structured, practical study plans that challenge and encourage growth.
Always return VALID JSON in the exact schema requested — no extra text before or after the JSON.`

const systemHTMLParser = `You are a data extraction specialist.
Your job is to extract Bible verse data from web page text and return it as JSON.
Return ONLY valid JSON — no explanation, no markdown, no backticks.`

// ---- Context helpers -------------------------------------------------------

// VerseContext bundles a passage reference with its text for prompting.
type VerseContext struct {
	Reference   string // e.g. "John 3:16"
	Text        string // full verse text
	Translation string // e.g. "KJV"
}

func (v VerseContext) String() string {
	return fmt.Sprintf("%s (%s): %s", v.Reference, v.Translation, v.Text)
}

// ---- High-level AI methods -------------------------------------------------

// ExplainVerse returns a streaming explanation of a single verse.
func (c *Client) ExplainVerse(ctx context.Context, v VerseContext) (<-chan string, <-chan error) {
	prompt := fmt.Sprintf(
		"Please explain this Bible verse in depth, covering its original language meaning (if relevant), historical context, theological significance, and practical application for today:\n\n%s",
		v.String(),
	)
	return c.Generate(ctx, systemBibleScholar, prompt, &Options{Temperature: 0.7})
}

// ExplainChapter returns a streaming overview of a chapter.
func (c *Client) ExplainChapter(ctx context.Context, book, chapter, translation, text string) (<-chan string, <-chan error) {
	prompt := fmt.Sprintf(
		"Give an overview and explanation of %s chapter %s (%s):\n\n%s",
		book, chapter, translation, truncate(text, 3000),
	)
	return c.Generate(ctx, systemBibleScholar, prompt, &Options{Temperature: 0.7})
}

// GenerateDevotional creates a daily devotional based on a verse or passage.
func (c *Client) GenerateDevotional(ctx context.Context, v VerseContext, theme string) (<-chan string, <-chan error) {
	themeClause := ""
	if theme != "" {
		themeClause = fmt.Sprintf(" with the theme: %s", theme)
	}
	prompt := fmt.Sprintf(
		"Write a complete daily devotional (about 400 words)%s based on this scripture:\n\n%s\n\nInclude a title at the very beginning.",
		themeClause, v.String(),
	)
	return c.Generate(ctx, systemDevotional, prompt, &Options{Temperature: 0.8, NumPredict: 600})
}

// GenerateSermon creates a full sermon outline and text.
func (c *Client) GenerateSermon(ctx context.Context, v VerseContext, title string) (<-chan string, <-chan error) {
	prompt := fmt.Sprintf(
		"Write a full sermon on this scripture. Title: %q\n\nScripture: %s\n\nInclude an opening illustration, exegesis, three main points with supporting verses, application, and a closing call to action.",
		title, v.String(),
	)
	return c.Generate(ctx, systemSermon, prompt, &Options{Temperature: 0.75, NumPredict: 1500})
}

// StudyPlanWeek is one week of a study plan.
type StudyPlanWeek struct {
	Week    int      `json:"week"`
	Theme   string   `json:"theme"`
	Verses  []string `json:"verses"`
	Reading string   `json:"reading"`
	Notes   string   `json:"notes"`
}

// StudyPlan is the full structured plan.
type StudyPlan struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Weeks       []StudyPlanWeek `json:"weeks"`
}

// GenerateStudyPlan creates a structured multi-week Bible study plan.
func (c *Client) GenerateStudyPlan(ctx context.Context, topic string, weeks int, translation string) (*StudyPlan, error) {
	prompt := fmt.Sprintf(
		`Create a %d-week Bible study plan on the topic: "%s" using the %s translation.
Return ONLY a JSON object with this schema:
{
  "title": "...",
  "description": "...",
  "weeks": [
    {
      "week": 1,
      "theme": "...",
      "verses": ["Book Chapter:Verse", ...],
      "reading": "Longer passage to read (e.g. John 1:1-18)",
      "notes": "Study notes and reflection questions"
    }
  ]
}`,
		weeks, topic, translation,
	)

	full, err := c.GenerateFull(ctx, systemStudyPlan, prompt, &Options{Temperature: 0.6, NumPredict: 2000})
	if err != nil {
		return nil, err
	}

	// Extract JSON from response (model may wrap in markdown)
	jsonStr := extractJSON(full)
	var plan StudyPlan
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse study plan JSON: %w\n\nRaw: %s", err, full)
	}
	return &plan, nil
}

// ExtractBibleFromHTML asks the model to parse Bible verses out of scraped text.
// Returns a JSON array of {verse_number, text} or error.
func (c *Client) ExtractBibleFromHTML(ctx context.Context, pageText, bookName, chapterNum string) ([]map[string]string, error) {
	prompt := fmt.Sprintf(
		`Extract all Bible verses from %s chapter %s from the following web page text.
Return ONLY a JSON array like:
[{"verse": "1", "text": "In the beginning..."}, ...]

Page text:
%s`,
		bookName, chapterNum, truncate(pageText, 6000),
	)

	full, err := c.GenerateFull(ctx, systemHTMLParser, prompt, &Options{Temperature: 0.1, NumPredict: 4000})
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(full)
	var verses []map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &verses); err != nil {
		return nil, fmt.Errorf("could not parse verse JSON from model: %w", err)
	}
	return verses, nil
}

// AskAboutVerse sends a free-form question about a verse.
func (c *Client) AskAboutVerse(ctx context.Context, question string, v VerseContext) (<-chan string, <-chan error) {
	prompt := fmt.Sprintf("Context verse: %s\n\nQuestion: %s", v.String(), question)
	return c.Generate(ctx, systemBibleScholar, prompt, &Options{Temperature: 0.7})
}

// ---- helpers ---------------------------------------------------------------

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...[truncated]"
}

// extractJSON pulls the first JSON object or array out of a string.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Strip markdown code fences
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) > 1 {
			s = lines[1]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	// Find first { or [
	for i, r := range s {
		if r == '{' || r == '[' {
			return s[i:]
		}
	}
	return s
}
