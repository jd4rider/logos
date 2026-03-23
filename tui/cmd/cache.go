package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jd4rider/logos/internal/tts"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the TTS audio cache",
	Long:  "Inspect, list, or clear cached TTS audio files stored in ~/.cache/logos/tts/",
}

var cacheStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cache statistics",
	Run: func(cmd *cobra.Command, args []string) {
		c := tts.NewAudioCache()
		s := c.Stats()
		fmt.Printf("Cache directory : %s\n", c.Dir())
		fmt.Printf("Entries         : %d\n", s.Entries)
		fmt.Printf("Total size      : %s\n", formatBytes(s.TotalBytes))
		fmt.Printf("Max size        : %s\n", formatBytes(s.MaxBytes))
		fmt.Printf("Total hits      : %d\n", s.TotalHits)
		if !s.OldestEntry.IsZero() {
			fmt.Printf("Oldest entry    : %s\n", s.OldestEntry.Format(time.RFC3339))
			fmt.Printf("Newest entry    : %s\n", s.NewestEntry.Format(time.RFC3339))
		}
	},
}

var cacheListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cached entries",
	Run: func(cmd *cobra.Command, args []string) {
		c := tts.NewAudioCache()
		entries := c.List()
		if len(entries) == 0 {
			fmt.Println("Cache is empty.")
			return
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ENGINE\tVOICE\tDURATION\tSIZE\tHITS\tLAST ACCESS\tTEXT PREVIEW")
		for _, e := range entries {
			dur := time.Duration(e.DurationMs) * time.Millisecond
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				e.Engine,
				e.VoiceName,
				formatDuration(dur),
				formatBytes(int64(e.PCMBytes)),
				e.AccessCount,
				e.LastAccess.Format("2006-01-02 15:04"),
				truncate(e.TextPreview, 50),
			)
		}
		w.Flush()
	},
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete all cached audio files",
	Run: func(cmd *cobra.Command, args []string) {
		c := tts.NewAudioCache()
		s := c.Stats()
		if s.Entries == 0 {
			fmt.Println("Cache is already empty.")
			return
		}
		fmt.Printf("Clearing %d entries (%s)...\n", s.Entries, formatBytes(s.TotalBytes))
		if err := c.Clear(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cache cleared.")
	},
}

func init() {
	cacheCmd.AddCommand(cacheStatsCmd, cacheListCmd, cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(b)/1024/1024/1024)
	case b >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(b)/1024/1024)
	case b >= 1024:
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
