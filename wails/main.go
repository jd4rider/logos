package main

import (
	"embed"
	"os"

	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/tts"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("API_BIBLE_KEY")
	piperModel := os.Getenv("PIPER_MODEL")

	client := api.NewClient(apiKey)
	ttsEng := tts.New(piperModel)
	app := NewApp(client, ttsEng)

	err := wails.Run(&options.App{
		Title:            "Bible Reader",
		Width:            1440,
		Height:           900,
		MinWidth:         1100,
		MinHeight:        720,
		StartHidden:      false,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 13, B: 26, A: 255},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "Bible Reader",
				Message: "A professional Bible reader desktop app",
			},
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
