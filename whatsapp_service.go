package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wwbot/internal/wa"
)

// WAEvent is the payload pushed to the frontend on the "wa" event. It is a flat,
// JSON-friendly view of the normalized events from internal/wa.
type WAEvent struct {
	Type    string      `json:"type"` // qr | paired | connected | loggedout | message | call
	QR      string      `json:"qr,omitempty"`
	JID     string      `json:"jid,omitempty"`
	Message *wa.Message `json:"message,omitempty"`
	Call    *wa.Call    `json:"call,omitempty"`
}

// WhatsAppService is the Wails-bound service that owns the WhatsApp connection.
// It exposes methods to the frontend and forwards internal/wa events onto the
// Wails event bus as "wa" events. The UI never touches whatsmeow directly.
type WhatsAppService struct {
	mu      sync.Mutex
	client  *wa.Client
	started bool
}

// ServiceName sets the bound service name used in generated bindings.
func (s *WhatsAppService) ServiceName() string { return "WhatsApp" }

// ServiceStartup is invoked by Wails during application startup. It opens the
// session store and, if already paired, reconnects automatically.
func (s *WhatsAppService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	dsn, err := sessionDSN()
	if err != nil {
		return err
	}
	client, err := wa.New(ctx, wa.Config{SessionDSN: dsn, LogLevel: "WARN"})
	if err != nil {
		return err
	}
	s.client = client

	go s.forwardEvents(client.Events())

	if client.IsPaired() {
		return s.connect(ctx)
	}
	return nil
}

// ServiceShutdown is invoked by Wails during application shutdown.
func (s *WhatsAppService) ServiceShutdown() error {
	if s.client != nil {
		s.client.Stop()
	}
	return nil
}

// StartPairing begins the QR pairing flow. The frontend calls this when the
// user chooses to link their WhatsApp. It is safe to call more than once.
func (s *WhatsAppService) StartPairing() error {
	return s.connect(context.Background())
}

// IsPaired reports whether a session already exists.
func (s *WhatsAppService) IsPaired() bool {
	return s.client != nil && s.client.IsPaired()
}

// connect starts the WhatsApp connection exactly once.
func (s *WhatsAppService) connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return errors.New("whatsapp service not initialised")
	}
	if s.started {
		return nil
	}
	if err := s.client.Start(ctx); err != nil {
		return err
	}
	s.started = true
	return nil
}

// forwardEvents maps internal/wa events to WAEvent and emits them to the frontend.
func (s *WhatsAppService) forwardEvents(events <-chan wa.Event) {
	for e := range events {
		var dto WAEvent
		switch ev := e.(type) {
		case wa.QR:
			dto = WAEvent{Type: "qr", QR: ev.Code}
		case wa.Paired:
			dto = WAEvent{Type: "paired", JID: ev.JID}
		case wa.Connected:
			dto = WAEvent{Type: "connected"}
		case wa.LoggedOut:
			dto = WAEvent{Type: "loggedout"}
		case wa.Message:
			m := ev
			dto = WAEvent{Type: "message", Message: &m}
		case wa.Call:
			c := ev
			dto = WAEvent{Type: "call", Call: &c}
		default:
			continue
		}
		if app := application.Get(); app != nil {
			app.Event.Emit("wa", dto)
		}
	}
}

// sessionDSN returns the SQLite DSN for the encrypted session store, located in
// the per-user application config directory (e.g. ~/Library/Application Support
// on macOS). The session DB is never placed in the repo.
func sessionDSN() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "ww-bot")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return "", err
	}
	return "file:" + filepath.Join(appDir, "session.db") + "?_foreign_keys=on", nil
}
