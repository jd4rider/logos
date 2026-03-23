package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jd4rider/logos/internal/ai"
	"github.com/jd4rider/logos/internal/crawler"
	"github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/importer"
	"github.com/spf13/cobra"
)

// ── import command ────────────────────────────────────────────────────────────

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a Bible translation from a file",
	Long: `Import a Bible translation into the local SQLite database.

Supported formats:
  CSV     - any CSV with book/chapter/verse/text columns (auto-detected)
  SQLite  - Scrollmapper, BibleSuperSearch, or logos SQLite exports

Example:
  logos import kjv.csv --abbr KJV --name "King James Version"
  logos import scrollmapper.db --abbr ESV`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		abbr, _ := cmd.Flags().GetString("abbr")
		name, _ := cmd.Flags().GetString("name")
		lang, _ := cmd.Flags().GetString("lang")
		id, _ := cmd.Flags().GetString("id")

		bibleDB, err := db.Open(db.DefaultDBPath())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer bibleDB.Close()

		opts := importer.ImportOptions{
			TranslationID: id,
			Name:          name,
			Abbreviation:  abbr,
			Language:      lang,
			Progress: func(msg string) {
				fmt.Println(msg)
			},
		}

		ext := strings.ToLower(path[strings.LastIndex(path, ".")+1:])
		switch ext {
		case "csv", "tsv", "txt":
			err = importer.ImportCSV(bibleDB, path, opts)
		case "db", "sqlite", "sqlite3":
			err = importer.ImportSQLiteFile(bibleDB, path, opts)
		default:
			// Try CSV first, fall back to SQLite
			err = importer.ImportCSV(bibleDB, path, opts)
			if err != nil {
				err = importer.ImportSQLiteFile(bibleDB, path, opts)
			}
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Import failed: %v\n", err)
			os.Exit(1)
		}
	},
}

// ── crawl command ─────────────────────────────────────────────────────────────

var crawlCmd = &cobra.Command{
	Use:   "crawl <url>",
	Short: "Crawl a Bible website and import it locally",
	Long: `Crawl a Bible website starting from a chapter URL and import all
chapters into the local SQLite database.

The crawler follows "next chapter" links automatically. Point it at any
chapter page of a site that displays verse-numbered Bible text.

Example:
  logos crawl "https://www.biblegateway.com/passage/?search=Genesis+1&version=KJV" \
      --abbr KJV --name "King James Version" --max-chapters 50`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		startURL := args[0]
		abbr, _ := cmd.Flags().GetString("abbr")
		name, _ := cmd.Flags().GetString("name")
		lang, _ := cmd.Flags().GetString("lang")
		id, _ := cmd.Flags().GetString("id")
		maxChapters, _ := cmd.Flags().GetInt("max-chapters")
		delayMs, _ := cmd.Flags().GetInt("delay")
		resume, _ := cmd.Flags().GetBool("resume")
		useAI, _ := cmd.Flags().GetBool("ai")

		bibleDB, err := db.Open(db.DefaultDBPath())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer bibleDB.Close()

		opts := crawler.Options{
			TranslationID: id,
			Name:          name,
			Abbreviation:  abbr,
			Language:      lang,
			MaxChapters:   maxChapters,
			Delay:         time.Duration(delayMs) * time.Millisecond,
			SkipExisting:  resume,
			Progress: func(msg string) {
				fmt.Println(msg)
			},
		}

		if useAI {
			opts.AIClient = ai.NewClient()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			available := opts.AIClient.IsAvailable(ctx)
			cancel()
			if !available {
				fmt.Fprintln(os.Stderr, "Warning: Ollama not available — AI fallback disabled")
				opts.AIClient = nil
			} else {
				fmt.Println("✓ Ollama AI fallback enabled")
			}
		}

		if err := crawler.Crawl(bibleDB, startURL, opts); err != nil {
			fmt.Fprintf(os.Stderr, "Crawl failed: %v\n", err)
			os.Exit(1)
		}
	},
}

// ── bibles command (list local translations) ──────────────────────────────────

var biblesCmd = &cobra.Command{
	Use:   "bibles",
	Short: "List locally stored Bible translations",
	Run: func(cmd *cobra.Command, args []string) {
		bibleDB, err := db.Open(db.DefaultDBPath())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		defer bibleDB.Close()

		stats, err := bibleDB.GetStats()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(stats) == 0 {
			fmt.Println("No local translations. Use 'logos import' or 'logos crawl' to add one.")
			return
		}

		translations, _ := bibleDB.ListTranslations()
		tMap := map[string]db.Translation{}
		for _, t := range translations {
			tMap[t.ID] = t
		}

		fmt.Printf("%-12s  %-30s  %5s  %7s  %6s  %s\n",
			"ABBR", "NAME", "BOOKS", "CHAPTERS", "VERSES", "SOURCE")
		fmt.Println(strings.Repeat("─", 80))
		for _, s := range stats {
			t := tMap[s.TranslationID]
			fmt.Printf("%-12s  %-30s  %5d  %7d  %6d  %s\n",
				t.Abbreviation, t.Name, s.Books, s.Chapters, s.Verses, t.Source)
		}
	},
}

func init() {
	// import flags
	importCmd.Flags().String("abbr", "", "Translation abbreviation (e.g. KJV)")
	importCmd.Flags().String("name", "", "Translation full name")
	importCmd.Flags().String("lang", "eng", "Language code")
	importCmd.Flags().String("id", "", "Override translation ID")

	// crawl flags
	crawlCmd.Flags().String("abbr", "", "Translation abbreviation")
	crawlCmd.Flags().String("name", "", "Translation full name")
	crawlCmd.Flags().String("lang", "eng", "Language code")
	crawlCmd.Flags().String("id", "", "Override translation ID")
	crawlCmd.Flags().Int("max-chapters", 0, "Stop after N chapters (0 = all)")
	crawlCmd.Flags().Int("delay", 1000, "Delay between requests in milliseconds")
	crawlCmd.Flags().Bool("resume", false, "Skip chapters already in the database (resume an interrupted crawl)")
	crawlCmd.Flags().Bool("ai", false, "Use Ollama AI as fallback parser for unsupported website formats")

	rootCmd.AddCommand(importCmd, crawlCmd, biblesCmd)
}
