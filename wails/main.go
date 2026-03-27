package main

import (
	"embed"
	"log"
	"runtime"

	"github.com/jd4rider/logos/internal/api"
	"github.com/jd4rider/logos/internal/tts"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const (
	popupMinWidth  = 980
	popupMinHeight = 680
	startupMargin  = 20
)

func sizeWindowToPrimaryWorkArea(app *application.App, window *application.WebviewWindow) {
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return
	}

	work := screen.WorkArea
	width := work.Width - (startupMargin * 2)
	height := work.Height - (startupMargin * 2)
	if width < popupMinWidth {
		width = popupMinWidth
	}
	if height < popupMinHeight {
		height = popupMinHeight
	}
	if width > work.Width {
		width = work.Width
	}
	if height > work.Height {
		height = work.Height
	}

	x := work.X + ((work.Width - width) / 2)
	y := work.Y + ((work.Height - height) / 2)
	window.SetBounds(application.Rect{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	})
}

func main() {
	prepareDesktopRuntime()

	service := NewLogosService(
		api.NewClient(runtimeEnv("API_BIBLE_KEY")),
		tts.New(runtimeEnv("PIPER_MODEL")),
	)

	app := application.New(application.Options{
		Name:        "Logos AI",
		Description: "Menu bar Bible reader powered by the Logos backend",
		Services: []application.Service{
			application.NewService(service),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
	})
	if len(appIcon) > 0 {
		app.SetIcon(appIcon)
	}

	popup := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "logos-popup",
		Title:            "Logos AI",
		URL:              "/",
		Width:            1180,
		Height:           820,
		MinWidth:         popupMinWidth,
		MinHeight:        popupMinHeight,
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

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		sizeWindowToPrimaryWorkArea(app, popup)
		popup.Show()
		popup.Focus()
	})

	tray := app.SystemTray.New()
	tray.SetTooltip("Logos AI")

	if runtime.GOOS == "darwin" {
		tray.SetLabel("W")
	} else if len(appIcon) > 0 {
		tray.SetIcon(appIcon)
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
