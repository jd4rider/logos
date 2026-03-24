package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	_ = godotenv.Load()

	service := NewLogosService(
		api.NewClient(os.Getenv("API_BIBLE_KEY")),
		tts.New(os.Getenv("PIPER_MODEL")),
	)

	app := application.New(application.Options{
		Name:        "Logos AI",
		Description: "Menu bar Bible reader powered by the Logos backend",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
	})

	popup := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "logos-popup",
		Title:            "Logos AI",
		URL:              "/",
		Width:            1180,
		Height:           820,
		MinWidth:         980,
		MinHeight:        680,
		AlwaysOnTop:      true,
		Hidden:           true,
		HideOnEscape:     true,
		HideOnFocusLost:  true,
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGB(8, 17, 28),
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 54,
			WindowLevel:             application.MacWindowLevelPopUpMenu,
			CollectionBehavior: application.MacWindowCollectionBehaviorMoveToActiveSpace |
				application.MacWindowCollectionBehaviorTransient |
				application.MacWindowCollectionBehaviorFullScreenAuxiliary,
		},
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: true,
		},
	})

	popup.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		popup.Hide()
		event.Cancel()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip("Logos AI")

	if runtime.GOOS == "darwin" {
		tray.SetTemplateIcon(icons.SystrayMacTemplate)
	}

	menu := app.NewMenu()
	menu.Add("Open Logos AI").OnClick(func(*application.Context) {
		popup.Show()
		popup.Focus()
	})
	menu.Add("Hide").OnClick(func(*application.Context) {
		popup.Hide()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) {
		app.Quit()
	})

	tray.SetMenu(menu)
	tray.AttachWindow(popup).WindowOffset(6)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
