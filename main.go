package main

import (
	"context"
	"embed"
	_ "embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"

	"wwbot/internal/core"
	"wwbot/internal/store"
)

// Wails embeds the built frontend (frontend/dist) into the binary.

//go:embed all:frontend/dist
var assets embed.FS

// version is set via -ldflags "-X main.version=..." at build time.
var version = "0.0.1"

func init() {
	// Register event types so the binding generator emits typed JS/TS APIs.
	application.RegisterEvent[WAEvent]("wa")
	application.RegisterEvent[Notice]("notice")
	application.RegisterEvent[UnknownContact]("unknown")
	application.RegisterEvent[string]("tray:nav")
	application.RegisterEvent[ReachOut]("tray:reach-out")
}

func main() {
	setupFileLogging()

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

	// Native OS notifications (new-number prompts). Wired to the core below.
	ns := notifications.New()

	// Dock badge: counts pending approvals + unsaved-contact prompts so the
	// user sees something needs attention without opening the window.
	dk := dock.New()

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
			application.NewService(&ScheduleService{core: cr}),
			application.NewService(&GroupsService{core: cr}),
			application.NewService(&UpdaterService{version: version}),
			application.NewService(ns),
			application.NewService(dk),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// Keep the bot running in the background when the window is closed.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	// Auto-update: check GitHub Releases for newer versions. We do a single
	// check shortly after launch (no background polling — desktop apps are
	// usually open for hours and one GitHub call per launch is enough). The
	// wails:updater:update-available event the check emits is what the
	// Settings page subscribes to for the install prompt; the manual
	// "Check for updates" button just calls Updater.Check() again.
	gh, err := github.New(github.Config{Repository: "fanom2813/ww-bot"})
	if err != nil {
		log.Printf("updater: %v", err)
	} else {
		app.Updater.Init(updater.Config{
			CurrentVersion: version,
			Providers:      []updater.Provider{gh},
		})
	}

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WW Bot",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			// Solid (non-vibrancy) backdrop: animating the sidebar width over a
			// translucent backdrop forces macOS to re-blur every frame, which makes
			// the collapse animation stutter.
			Backdrop: application.MacBackdropNormal,
			TitleBar: application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(10, 10, 10),
		URL:              "/",
	})

	// Closing the window hides it (the bot keeps running in the tray) rather
	// than quitting; Quit is explicit via the tray menu. This MUST be a hook,
	// not OnWindowEvent: only hooks run synchronously before the default close
	// and have their Cancel() honored — listeners run after the close fires.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		log.Println("[tray] window close intercepted — hiding to tray, bot keeps running")
		win.Hide()
		e.Cancel()
	})

	// Bridge the core's unknown-contact prompt to in-app + native notifications,
	// and ask for notification permission once the run loop is up.
	wireNotifications(cr, ns)

	// Dock badge tracker: refresh on every queue/contact change so it always
	// matches what the user sees inside the app. The initial sync waits for
	// ApplicationStarted since NSApp must be running for the badge call to land.
	syncBadge := wireDockBadge(cr, dk)
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if ok, err := ns.RequestNotificationAuthorization(); err != nil || !ok {
			log.Printf("notification authorization: granted=%v err=%v", ok, err)
		}
		syncBadge()
		// One-shot update check shortly after launch. Delayed a couple of
		// seconds so it doesn't compete with the QR/connect path on startup;
		// the frontend listens for wails:updater:update-available.
		go func() {
			defer recoverPanic("updater startup check")
			time.Sleep(3 * time.Second)
			if _, cerr := app.Updater.Check(context.Background()); cerr != nil {
				log.Printf("updater: startup check: %v", cerr)
			}
		}()
	})

	popover := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "WW Quick",
		Width:            360,
		Height:           560,
		MinWidth:         360,
		MinHeight:        560,
		MaxWidth:         360,
		MaxHeight:        560,
		Frameless:        true,
		Hidden:           true,
		BackgroundColour: application.NewRGB(10, 10, 10),
		URL:              "/#/tray",
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropNormal,
		},
	})

	setupTray(app, win, popover, cr)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// setupTray adds a menu-bar tray icon with Open / Pause / Quit so the bot can
// run resident in the background.
func setupTray(app *application.App, win, popover *application.WebviewWindow, cr *core.Core) {
	tray := app.SystemTray.New()
	tray.SetLabel("WW")

	open := func(path string) {
		win.Show()
		win.Focus()
		if path != "" {
			app.Event.Emit("tray:nav", path)
		}
	}

	menu := app.NewMenu()
	menu.Add("Open full app").OnClick(func(*application.Context) { open("/") })
	menu.AddSeparator()

	pauseItem := menu.Add("Pause bot")
	if cr.Paused() {
		pauseItem.SetLabel("Resume bot")
	}
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
	menu.Add("Check for Updates…").OnClick(func(*application.Context) {
		app.Updater.Check(context.Background())
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })

	tray.SetMenu(menu)
	// Left-click toggles the popover; right-click shows the menu above.
	tray.AttachWindow(popover)
}
