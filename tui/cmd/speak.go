package cmd

import (
"fmt"
"strings"
"time"

"github.com/jd4rider/logos/internal/tts"

"github.com/spf13/cobra"
)

var speakCmd = &cobra.Command{
Use:   "speak [reference]",
Short: "Speak a Bible verse or chapter aloud",
Long: `Speak a Bible verse or chapter using the system TTS engine.

Examples:
  logos speak GEN.1.1
  logos speak JHN.3.16`,
Args: cobra.ExactArgs(1),
RunE: func(cmd *cobra.Command, args []string) error {
if !ttsEngine.Available() {
return fmt.Errorf("no TTS engine available (tried say, espeak-ng, espeak, piper)")
}

ref := args[0]
bibleID, _ := cmd.Flags().GetString("bible")
if bibleID == "" {
bibleID = "de4e12af7f28f599-02"
}

parts := strings.Split(ref, ".")
var text string
var reference string

switch len(parts) {
case 2:
ch, err := client.GetChapter(bibleID, ref)
if err != nil {
return fmt.Errorf("fetching chapter: %w", err)
}
text = tts.CleanForTTS(ch.Content)
reference = ch.Reference
case 3:
v, err := client.GetVerse(bibleID, ref)
if err != nil {
return fmt.Errorf("fetching verse: %w", err)
}
text = tts.CleanForTTS(v.Content)
reference = v.Reference
default:
return fmt.Errorf("invalid reference: use BOOK.CHAPTER or BOOK.CHAPTER.VERSE")
}

fmt.Printf("Speaking %s via %s…\n", reference, ttsEngine.EngineName())
_, err := ttsEngine.Speak(text)
	if err != nil {
return fmt.Errorf("TTS error: %w", err)
}

// Wait for speech to finish
for ttsEngine.IsPlaying() {
time.Sleep(200 * time.Millisecond)
}
return nil
},
}

func init() {
speakCmd.Flags().StringP("bible", "b", "", "Bible ID (default: KJV)")
rootCmd.AddCommand(speakCmd)
}
