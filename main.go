package main

import (
	"context"
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

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
			// Keep the bot running in the background when the window is closed.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WW Bot",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(10, 10, 10),
		URL:              "/",
	})

	// Closing the window hides it (the bot keeps running in the tray) rather
	// than quitting; Quit is explicit via the tray menu.
	win.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})

	setupTray(app, win, cr)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupTray adds a menu-bar tray icon with Open / Pause / Quit so the bot can
// run resident in the background.
func setupTray(app *application.App, win *application.WebviewWindow, cr *core.Core) {
	tray := app.SystemTray.New()
	tray.SetLabel("WW")

	menu := app.NewMenu()
	menu.Add("Open WW Bot").OnClick(func(*application.Context) { win.Show() })
	menu.AddSeparator()

	pauseItem := menu.Add("Pause bot")
	pauseItem.OnClick(func(*application.Context) {
		if cr.Paused() {
			cr.Resume()
			pauseItem.SetLabel("Pause bot")
		} else {
			cr.Pause()
			pauseItem.SetLabel("Resume bot")
		}
	})

	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })

	tray.SetMenu(menu)
	tray.AttachWindow(win)
}
