package main

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wwbot/internal/core"
	"wwbot/internal/wa"
)

// WAEvent is the connection-status payload pushed to the frontend on "wa".
type WAEvent struct {
	Type string `json:"type"` // qr | paired | connected | loggedout
	QR   string `json:"qr,omitempty"`
	JID  string `json:"jid,omitempty"`
}

// Notice is a user-facing notification pushed to the frontend on "notice".
type Notice struct {
	Level string `json:"level"` // info | draft | notify | error
	Title string `json:"title"`
	Body  string `json:"body"`
}

// WhatsAppService owns the WhatsApp connection and bridges wa events both to
// the frontend (connection state) and into the core pipeline (messages).
type WhatsAppService struct {
	core   *core.Core
	client *wa.Client
}

func (s *WhatsAppService) ServiceName() string { return "WhatsApp" }

// ServiceStartup creates the WhatsApp client, wires it into the core, starts the
// pipeline, and (if already paired) reconnects.
func (s *WhatsAppService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	client, err := wa.New(ctx, wa.Config{SessionDSN: sessionDSN(), LogLevel: "WARN"})
	if err != nil {
		return err
	}
	s.client = client

	// Outbound sends flow through the core's safety gate into the wa client.
	s.core.AttachSender(client.SendText)
	go s.bridge(client.Events())
	s.core.Start(ctx)

	if client.IsPaired() {
		s.core.SetSelfJID(client.SelfJID())
		return client.Start(ctx)
	}
	return nil
}

// ServiceShutdown stops the pipeline and disconnects.
func (s *WhatsAppService) ServiceShutdown() error {
	s.core.Stop()
	if s.client != nil {
		s.client.Stop()
	}
	return nil
}

// StartPairing begins the QR pairing flow (called from the Connect screen).
func (s *WhatsAppService) StartPairing() error {
	if s.client == nil {
		return nil
	}
	return s.client.Start(context.Background())
}

// IsPaired reports whether a session already exists.
func (s *WhatsAppService) IsPaired() bool { return s.client != nil && s.client.IsPaired() }

// bridge forwards wa events: connection state to the frontend, messages to core.
func (s *WhatsAppService) bridge(events <-chan wa.Event) {
	for e := range events {
		switch ev := e.(type) {
		case wa.QR:
			emitWA(WAEvent{Type: "qr", QR: ev.Code})
		case wa.Paired:
			s.core.SetSelfJID(s.client.SelfJID())
			emitWA(WAEvent{Type: "paired", JID: ev.JID})
		case wa.Connected:
			s.core.SetSelfJID(s.client.SelfJID())
			emitWA(WAEvent{Type: "connected"})
		case wa.LoggedOut:
			emitWA(WAEvent{Type: "loggedout"})
		case wa.Message:
			s.core.OnInbound(core.Inbound{
				ChatJID: ev.ChatJID, SenderJID: ev.SenderJID, PushName: ev.PushName,
				Text: ev.Text, Kind: string(ev.Kind), IsFromMe: ev.IsFromMe,
				IsGroup: ev.IsGroup, Timestamp: ev.Timestamp, Audio: ev.AudioData,
			})
		case wa.Call:
			emitNotice(Notice{Level: "notify", Title: "Incoming call", Body: ev.FromJID})
		}
	}
}

func emitWA(ev WAEvent) {
	if app := application.Get(); app != nil {
		app.Event.Emit("wa", ev)
	}
}

func emitNotice(n Notice) {
	if app := application.Get(); app != nil {
		app.Event.Emit("notice", n)
	}
}
