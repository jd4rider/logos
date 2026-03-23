package cmd

import (
	"fmt"
	"os"

	"github.com/jd4rider/logos/internal/db"
	"github.com/jd4rider/logos/internal/precache"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/spf13/cobra"
)

var precacheCmd = &cobra.Command{
	Use:   "precache [translation-id]",
	Short: "Pre-synthesise TTS audio for an offline Bible translation",
	Long: `Synthesises TTS audio for every chapter in an offline translation
and stores it in the audio cache (~/.cache/logos/tts/).

Subsequent reads will start instantly with no synthesis delay.
If a chapter is already cached it is skipped automatically.

Use 'logos bibles' to list available offline translation IDs.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		translationID := args[0]

		// Open DB
		database, err := db.Open(db.DefaultDBPath())
		if err != nil {
			return fmt.Errorf("could not open database: %w", err)
		}
		defer database.Close()

		// Verify translation exists
		locals, err := database.ListTranslations()
		if err != nil {
			return err
		}
		found := false
		var transName string
		for _, t := range locals {
			if t.ID == translationID {
				found = true
				transName = t.Name
				break
			}
		}
		if !found {
			return fmt.Errorf("translation %q not found in local database.\nRun 'logos bibles' to see available offline translations.", translationID)
		}

		// Build TTS engine
		piperModel := os.Getenv("PIPER_MODEL")
		engine := tts.New(piperModel)
		if !engine.Available() {
			return fmt.Errorf("no TTS engine available — install Piper or Kokoro first")
		}

		fmt.Printf("Pre-caching TTS audio for: %s (%s)\n", transName, translationID)
		fmt.Printf("Using voice: %s [%s]\n\n", engine.ActiveVoice().Name, engine.EngineName())

		job := precache.NewJob(database, engine, translationID)
		job.Start()

		var lastBook string
		for p := range job.Progress() {
			if p.Err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠  %v\n", p.Err)
				continue
			}
			if p.Finished {
				fmt.Printf("\n✓  Done! Cached %d chapters for %s\n", p.Done, transName)
				break
			}
			if p.BookName != lastBook {
				if lastBook != "" {
					fmt.Println()
				}
				fmt.Printf("  📖 %s\n", p.BookName)
				lastBook = p.BookName
			}
			pct := 0
			if p.Total > 0 {
				pct = (p.Done * 100) / p.Total
			}
			fmt.Printf("\r    [%3d%%] chapter %-30s", pct, p.ChapterID)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(precacheCmd)
}
