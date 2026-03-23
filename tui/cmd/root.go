package cmd

import (
"fmt"
"os"

"github.com/jd4rider/logos/internal/api"
"github.com/jd4rider/logos/internal/tts"
"github.com/jd4rider/logos/tui/ui"

"github.com/joho/godotenv"
"github.com/spf13/cobra"
)

var (
client    *api.Client
ttsEngine *tts.Engine
)

var rootCmd = &cobra.Command{
Use:   "logos",
Short: "A beautiful terminal Bible reader",
Long: `Logos AI — Read, search, and listen to the Bible in your terminal.
Powered by API.Bible.`,
RunE: func(cmd *cobra.Command, args []string) error {
return ui.Run(client, ttsEngine)
},
}

// Execute runs the root command
func Execute() {
if err := rootCmd.Execute(); err != nil {
fmt.Fprintln(os.Stderr, err)
os.Exit(1)
}
}

func init() {
cobra.OnInitialize(initConfig)
}

func initConfig() {
	// Load from ~/.config/logos/.env, ~/.logos.env, and CWD .env (in priority order)
	home, _ := os.UserHomeDir()
	candidates := []string{
		home + "/.config/logos/.env",
		home + "/.logos.env",
		".env",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
		}
	}

	apiKey := os.Getenv("API_BIBLE_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: API_BIBLE_KEY not set. Add it to ~/.config/logos/.env")
		os.Exit(1)
	}

	client = api.NewClient(apiKey)
	piperModel := os.Getenv("PIPER_MODEL")
	ttsEngine = tts.New(piperModel)
}
