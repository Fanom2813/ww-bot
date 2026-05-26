package main

import (
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails embeds the built frontend (frontend/dist) into the binary.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register the "wa" event type so the binding generator emits a strongly
	// typed JS/TS API for WhatsApp events pushed from the backend.
	application.RegisterEvent[WAEvent]("wa")
}

// main is the application entry point: it constructs the app with the WhatsApp
// service bound, opens the main window, and runs until exit.
func main() {
	app := application.New(application.Options{
		Name:        "ww-bot",
		Description: "WhatsApp AI reply assistant",
		Services: []application.Service{
			application.NewService(&WhatsAppService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WW Bot",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(10, 10, 10),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
