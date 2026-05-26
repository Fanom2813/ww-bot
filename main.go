package main

import (
	"context"
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wwbot/internal/core"
	"wwbot/internal/store"
)

// Wails embeds the built frontend (frontend/dist) into the binary.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register event types so the binding generator emits typed JS/TS APIs.
	application.RegisterEvent[WAEvent]("wa")
	application.RegisterEvent[Notice]("notice")
}

func main() {
	ctx := context.Background()

	// Open the app data store.
	db, err := store.Open(ctx, dataDSN())
	if err != nil {
		log.Fatalf("open data store: %v", err)
	}

	// Build the core orchestrator; notifications flow to the frontend.
	cr, err := core.New(ctx, db, func(level, title, body string) {
		if app := application.Get(); app != nil {
			app.Event.Emit("notice", Notice{Level: level, Title: title, Body: body})
		}
	})
	if err != nil {
		log.Fatalf("init core: %v", err)
	}

	app := application.New(application.Options{
		Name:        "ww-bot",
		Description: "WhatsApp AI reply assistant",
		Services: []application.Service{
			application.NewService(&WhatsAppService{core: cr}),
			application.NewService(&ContactsService{core: cr}),
			application.NewService(&ApprovalsService{core: cr}),
			application.NewService(&SettingsService{core: cr}),
			application.NewService(&ActivityService{core: cr}),
			application.NewService(&ControlService{core: cr}),
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
