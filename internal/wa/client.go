// Package wa is a thin, UI-agnostic wrapper around the whatsmeow WhatsApp
// client. It owns the connection and session store, and surfaces normalized
// events (QR, Paired, Message, Call, ...) on a single channel. Nothing outside
// this package imports whatsmeow, so the rest of the app — terminal harness or
// Wails GUI alike — depends only on these stable types.
package wa

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // registers the "sqlite3" database/sql driver

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	"wwbot/internal/dbg"
)

// WAContact is a synced address-book contact from WhatsApp.
type WAContact struct {
	JID  string `json:"jid"`
	Name string `json:"name"`
}

// WAGroup is a group chat the account is a member of.
type WAGroup struct {
	JID          string `json:"jid"`
	Name         string `json:"name"`
	Participants int    `json:"participants"`
}

// Config configures a Client.
type Config struct {
	// SessionDSN is the SQLite DSN for the encrypted session store, e.g.
	// "file:wwbot-session.db?_foreign_keys=on".
	SessionDSN string
	// LogLevel for whatsmeow's internal loggers: DEBUG, INFO, WARN, or ERROR.
	// Defaults to WARN.
	LogLevel string
}

// Client wraps a whatsmeow client and emits normalized events on Events().
type Client struct {
	wm        *whatsmeow.Client
	container *sqlstore.Container
	events    chan Event
	closeOnce sync.Once
}

// New opens the session store and constructs a Client. It does not connect;
// call Start for that.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.SessionDSN == "" {
		return nil, errors.New("wa: SessionDSN is required")
	}
	level := cfg.LogLevel
	if level == "" {
		level = "WARN"
	}

	container, err := sqlstore.New(ctx, "sqlite3", cfg.SessionDSN, waLog.Stdout("wa-db", level, true))
	if err != nil {
		return nil, fmt.Errorf("wa: open session store: %w", err)
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("wa: load device: %w", err)
	}

	c := &Client{
		wm:        whatsmeow.NewClient(device, waLog.Stdout("wa", level, true)),
		container: container,
		events:    make(chan Event, 256),
	}
	// Retry a failed *initial* connect in the background (e.g. no network at
	// launch) instead of returning an error. whatsmeow dispatches Disconnected
	// and keeps retrying, then Connected once the network is back.
	c.wm.InitialAutoReconnect = true
	c.wm.AddEventHandler(c.onEvent)
	return c, nil
}

// Events returns the channel of normalized events. A single consumer should
// range over it; it is closed by Stop.
func (c *Client) Events() <-chan Event { return c.events }

// IsPaired reports whether a session already exists (i.e. no QR is needed).
func (c *Client) IsPaired() bool { return c.wm.Store.ID != nil }

// SelfJID returns the user's own (non-device) JID once paired, else "".
func (c *Client) SelfJID() string {
	if c.wm.Store.ID == nil {
		return ""
	}
	return c.wm.Store.ID.ToNonAD().String()
}

// Start connects to WhatsApp. If not yet paired, it begins the QR pairing flow,
// emitting QR events (and a Paired event on success). If already paired, it
// reconnects using the stored session — no QR.
func (c *Client) Start(ctx context.Context) error {
	// Disconnect first if already connected (needed for retries after timeout).
	if c.wm.IsConnected() {
		c.wm.Disconnect()
	}

	if c.IsPaired() {
		if err := c.wm.Connect(); err != nil {
			return fmt.Errorf("wa: connect: %w", err)
		}
		return nil
	}

	qrChan, err := c.wm.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("wa: qr channel: %w", err)
	}
	if err := c.wm.Connect(); err != nil {
		return fmt.Errorf("wa: connect: %w", err)
	}
	go c.pumpQR(qrChan)
	return nil
}

// SendText sends a plain text message to the given JID string (e.g.
// "12345@s.whatsapp.net"). In the full app, every outbound message funnels
// through the safety gate before reaching this method.
func (c *Client) SendText(ctx context.Context, to, text string) error {
	jid, err := types.ParseJID(to)
	if err != nil {
		return fmt.Errorf("wa: parse jid %q: %w", to, err)
	}
	if _, err := c.wm.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)}); err != nil {
		return fmt.Errorf("wa: send: %w", err)
	}
	return nil
}

// Contacts returns the user's WhatsApp address book (synced from the phone) as
// individual user contacts with a display name, sorted by name. Returns nil if
// not paired.
func (c *Client) Contacts(ctx context.Context) ([]WAContact, error) {
	if c.wm.Store.ID == nil {
		return nil, nil
	}
	all, err := c.wm.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, fmt.Errorf("wa: get contacts: %w", err)
	}
	self := c.wm.Store.ID.ToNonAD().String()
	out := make([]WAContact, 0, len(all))
	for jid, info := range all {
		if jid.Server != types.DefaultUserServer {
			continue // skip groups, LID-only, broadcast, etc.
		}
		nonAD := jid.ToNonAD().String()
		if nonAD == self {
			continue
		}
		name := firstNonEmpty(info.FullName, info.FirstName, info.PushName, info.BusinessName, jid.User)
		out = append(out, WAContact{JID: nonAD, Name: name})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// Groups returns the WhatsApp groups the account is a member of, sorted by
// name. Returns nil if not paired.
func (c *Client) Groups(ctx context.Context) ([]WAGroup, error) {
	if c.wm.Store.ID == nil {
		return nil, nil
	}
	gs, err := c.wm.GetJoinedGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("wa: get joined groups: %w", err)
	}
	out := make([]WAGroup, 0, len(gs))
	for _, g := range gs {
		name := g.Name
		if strings.TrimSpace(name) == "" {
			name = g.JID.User
		}
		out = append(out, WAGroup{
			JID:          g.JID.String(),
			Name:         name,
			Participants: g.ParticipantCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Stop disconnects from WhatsApp and closes the events channel.
func (c *Client) Stop() {
	c.wm.Disconnect()
	c.closeOnce.Do(func() { close(c.events) })
}

// Logout disconnects from WhatsApp and deletes the session so the next Start
// begins a fresh QR pairing flow.
func (c *Client) Logout(ctx context.Context) error {
	c.wm.Disconnect()
	if c.wm.Store.ID != nil {
		if err := c.container.DeleteDevice(ctx, c.wm.Store); err != nil {
			return fmt.Errorf("wa: delete session: %w", err)
		}
	}
	c.emit(LoggedOut{})
	return nil
}

// pumpQR forwards whatsmeow's QR-channel items as normalized events.
// If the channel closes without a successful scan, PairingExpired is emitted
// so the frontend can show a retry button.
func (c *Client) pumpQR(qrChan <-chan whatsmeow.QRChannelItem) {
	paired := false
	for item := range qrChan {
		switch item.Event {
		case "code":
			c.emit(QR{Code: item.Code})
		case "success":
			paired = true
			var id string
			if c.wm.Store.ID != nil {
				id = c.wm.Store.ID.String()
			}
			c.emit(Paired{JID: id})
		}
	}
	if !paired {
		c.emit(PairingExpired{})
	}
}

// onEvent translates whatsmeow events into our normalized ones. It runs on
// whatsmeow's dispatch goroutine, so it must not block for long; emit uses a
// generously buffered channel.
func (c *Client) onEvent(raw any) {
	switch v := raw.(type) {
	case *events.Message:
		// Ignore Status updates, broadcast lists, and newsletters — the bot never
		// replies to these, only to real 1-1 and group chats.
		if s := v.Info.Chat.Server; s == types.BroadcastServer || s == types.NewsletterServer {
			return
		}
		dbg.Printf("[wa] message chat=%q sender=%q fromMe=%v type=%s",
			v.Info.Chat.String(), v.Info.Sender.String(), v.Info.IsFromMe, v.Info.Type)
		self := ""
		if c.wm.Store.ID != nil {
			self = c.wm.Store.ID.ToNonAD().String()
		}
		m := toMessage(v, self)
		if am := v.Message.GetAudioMessage(); am != nil {
			// Download voice-note bytes so consumers can transcribe them.
			dctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			if data, err := c.wm.Download(dctx, am); err == nil {
				m.AudioData = data
				m.AudioMime = am.GetMimetype()
			}
			cancel()
		}
		c.emit(m)
	case *events.CallOffer:
		c.emit(Call{FromJID: v.From.String(), Group: v.GroupJID.Server != ""})
	case *events.CallOfferNotice:
		c.emit(Call{FromJID: v.From.String(), Video: v.Media == "video", Group: v.Type == "group"})
	case *events.Connected:
		c.emit(Connected{JID: c.SelfJID()})
	case *events.Disconnected:
		c.emit(Disconnected{})
	case *events.KeepAliveTimeout:
		// Pings stopped getting through (silent internet drop): show offline.
		// whatsmeow keeps the session and reconnects; this is NOT a logout.
		c.emit(Disconnected{})
	case *events.KeepAliveRestored:
		c.emit(Connected{JID: c.SelfJID()})
	case *events.LoggedOut:
		// Genuine server-side logout only (401/403/406 or stream error) — never
		// fired for network loss. Log the reason so it's clear why re-pair is needed.
		dbg.Printf("[wa] LOGGED OUT by WhatsApp (onConnect=%v reason=%v) — session invalidated", v.OnConnect, v.Reason)
		c.emit(LoggedOut{})
	}
}

func (c *Client) emit(e Event) { c.events <- e }

func toMessage(v *events.Message, selfJID string) Message {
	text := v.Message.GetConversation()
	var mentioned []string
	if ext := v.Message.GetExtendedTextMessage(); ext != nil {
		text = ext.GetText()
		if ci := ext.GetContextInfo(); ci != nil {
			mentioned = ci.GetMentionedJID()
		}
	}
	kind := KindOther
	switch {
	case text != "":
		kind = KindText
	case v.Message.GetAudioMessage() != nil:
		kind = KindVoice
	case v.Message.GetImageMessage() != nil:
		kind = KindImage
	}
	mentionsMe := false
	if selfJID != "" {
		// WhatsApp puts mentions in the @s.whatsapp.net form; normalize to NonAD
		// (number@server) on both sides for the comparison.
		for _, m := range mentioned {
			if mj, err := types.ParseJID(m); err == nil && mj.ToNonAD().String() == selfJID {
				mentionsMe = true
				break
			}
		}
	}
	return Message{
		ChatJID:    v.Info.Chat.String(),
		SenderJID:  v.Info.Sender.String(),
		PushName:   v.Info.PushName,
		Kind:       kind,
		Text:       text,
		Timestamp:  v.Info.Timestamp,
		IsFromMe:   v.Info.IsFromMe,
		IsGroup:    v.Info.IsGroup,
		MentionsMe: mentionsMe,
	}
}
