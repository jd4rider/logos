package cmd

import (
"fmt"
"strings"

"github.com/jd4rider/logos/internal/tts"

"github.com/spf13/cobra"
)

var readCmd = &cobra.Command{
Use:   "read [reference]",
Short: "Read a Bible chapter or verse",
Long: `Read a Bible chapter or verse by reference.

Examples:
  logos read GEN.1       # Genesis chapter 1
  logos read GEN.1.1     # Genesis 1:1
  logos read REV.22`,
Args: cobra.ExactArgs(1),
RunE: func(cmd *cobra.Command, args []string) error {
ref := args[0]
bibleID, _ := cmd.Flags().GetString("bible")
if bibleID == "" {
bibleID = "de4e12af7f28f599-02" // KJV default
}

parts := strings.Split(ref, ".")
switch len(parts) {
case 2:
// Chapter
ch, err := client.GetChapter(bibleID, ref)
if err != nil {
return fmt.Errorf("fetching chapter: %w", err)
}
fmt.Printf("=== %s ===\n\n", ch.Reference)
fmt.Println(tts.StripVerseMarkers(ch.Content))
fmt.Printf("\n— %s\n", ch.Copyright)

case 3:
// Verse
v, err := client.GetVerse(bibleID, ref)
if err != nil {
return fmt.Errorf("fetching verse: %w", err)
}
fmt.Printf("%s\n%s\n", v.Reference, tts.StripVerseMarkers(v.Content))

default:
return fmt.Errorf("invalid reference %q: use format BOOK.CHAPTER or BOOK.CHAPTER.VERSE", ref)
}
return nil
},
}

func init() {
readCmd.Flags().StringP("bible", "b", "", "Bible ID (default: KJV)")
rootCmd.AddCommand(readCmd)
}
