// Package core is the central orchestrator that wires the domain layers
// together: it receives inbound messages, coalesces them (inbox), transcribes
// voice (stt), decides what to do (brain + guardrails), and routes the outcome
// through the safety gate or into the approvals queue (store). It also handles
// self-chat slash commands and scheduled proactive greetings, and exposes the
// operations the UI needs. It does not import wa or Wails — the app maps wa
// events into core.Inbound and injects the send function.
package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"wwbot/internal/brain"
	"wwbot/internal/commands"
	"wwbot/internal/inbox"
	"wwbot/internal/safety"
	"wwbot/internal/schedule"
	"wwbot/internal/stt"
	"wwbot/internal/store"
)

// Notifier pushes a user-facing notification (mapped to a UI event by the app).
type Notifier func(level, title, body string)

// SendFunc delivers a message to a JID (wraps wa.Client.SendText).
type SendFunc func(ctx context.Context, toJID, text string) error

// Inbound is a normalized incoming message handed to the core by the app.
type Inbound struct {
	ChatJID   string
	SenderJID string
	PushName  string
	Text      string
	Kind      string // text | voice | image | other
	IsFromMe  bool
	IsGroup   bool
	Timestamp time.Time
	Audio     []byte // voice-note bytes (Kind == "voice")
}

// Core is the orchestrator.
type Core struct {
	db       *store.DB
	notify   Notifier
	scheduler *schedule.Scheduler

	mu       sync.RWMutex
	settings Settings
	stt      stt.Transcriber
	brn      *brain.Brain
	gate     *safety.Gate
	debounce *inbox.Debouncer

	send    SendFunc
	selfJID string
	ctx     context.Context
}

// New builds a Core from persisted settings. notify may be nil.
func New(ctx context.Context, db *store.DB, notify Notifier) (*Core, error) {
	if notify == nil {
		notify = func(string, string, string) {}
	}
	s, err := loadSettings(ctx, db)
	if err != nil {
		return nil, err
	}
	c := &Core{db: db, notify: notify, scheduler: schedule.New(), settings: s, ctx: ctx}
	c.build(s)
	return c, nil
}

// build (re)constructs the LLM registry, transcriber, brain, gate, and inbox
// from settings. The gate uses a thunk so the sender can be attached later.
func (c *Core) build(s Settings) {
	reg := buildRegistry(s.Providers)
	c.brn = brain.New(reg, brainConfig(s))
	c.stt = buildSTT(s.STT)

	c.gate = safety.New(buildSafetyConfig(s.Safety),
		func(ctx context.Context, to, text string) error {
			if c.send == nil {
				return errors.New("core: no sender attached")
			}
			return c.send(ctx, to, text)
		},
		safety.Options{
			OnSent: func(to, text string) {
				_ = c.db.LogActivity(c.ctx, "sent", to, preview(text))
			},
			OnDrop: func(j safety.Job, reason string) {
				_ = c.db.LogActivity(c.ctx, "system", j.ToJID, "send suppressed: "+reason)
			},
		})

	c.debounce = inbox.New(inbox.Config{
		Quiet: 30 * time.Second, MaxWait: 3 * time.Minute, WindowSize: 10,
	}, c.handleBatch)
}

// AttachSender wires the function used to actually deliver messages.
func (c *Core) AttachSender(send SendFunc) { c.send = send }

// SetSelfJID records the user's own JID (to recognize self-chat commands).
func (c *Core) SetSelfJID(jid string) { c.selfJID = jid }

// Start launches the gate worker and the scheduler.
func (c *Core) Start(ctx context.Context) {
	c.ctx = ctx
	c.gate.Start(ctx)
	c.registerGreetings()
	c.scheduler.Start(ctx)
}

// Stop stops background workers.
func (c *Core) Stop() {
	c.gate.Stop()
	c.scheduler.Stop()
}

func (c *Core) registerGreetings() {
	c.mu.RLock()
	greetings := c.settings.Greetings
	c.mu.RUnlock()
	for _, g := range greetings {
		if !g.Enabled || g.ToJID == "" {
			continue
		}
		g := g
		c.scheduler.Daily(fmt.Sprintf("greet-%s", g.ToJID), g.Hour, g.Min, func(ctx context.Context) {
			if err := c.gate.Enqueue(safety.Job{ToJID: g.ToJID, Text: g.Text, BypassQuiet: true}); err == nil {
				_ = c.db.LogActivity(ctx, "proactive", g.ToJID, preview(g.Text))
			}
		})
	}
}

// OnInbound is the pipeline entry point for an incoming message.
func (c *Core) OnInbound(in Inbound) {
	// Self-chat commands / daily notes.
	if c.selfJID != "" && in.IsFromMe && in.ChatJID == c.selfJID {
		c.handleSelfInput(in)
		return
	}

	text := in.Text
	if in.Kind == "voice" {
		text = c.transcribe(in)
		if text == "" {
			// Couldn't understand the audio confidently — stay silent, just note it.
			_ = c.db.LogActivity(c.ctx, "system", in.ChatJID, "voice note not understood — skipped")
			return
		}
	}

	c.debounce.Add(inbox.Msg{
		ChatJID: in.ChatJID, SenderJID: in.SenderJID, PushName: in.PushName,
		Text: text, Kind: in.Kind, IsFromMe: in.IsFromMe, IsGroup: in.IsGroup, Timestamp: in.Timestamp,
	})
}

func (c *Core) transcribe(in Inbound) string {
	c.mu.RLock()
	tr := c.stt
	minConf := c.settings.MinSTTConf
	c.mu.RUnlock()
	if tr == nil || len(in.Audio) == 0 {
		return ""
	}
	res, err := tr.Transcribe(c.ctx, bytes.NewReader(in.Audio), "voice.ogg")
	if err != nil || !res.Usable(minConf) {
		return ""
	}
	return res.Text
}

// handleBatch runs the brain on a coalesced batch and routes the outcome.
func (c *Core) handleBatch(b inbox.Batch) {
	ctx := c.ctx
	contact, _ := c.db.GetContact(ctx, b.ChatJID)
	tier := contact.Tier
	if tier == "" {
		tier = store.TierNotify
	}

	incoming, senderName := summarizeBurst(b, contact.Name)
	day := time.Now().Format("2006-01-02")
	daily, _ := c.db.GetDailyContext(ctx, day)

	in := brain.Input{
		SenderName:   senderName,
		IsGroup:      isGroup(b),
		GroupOptIn:   false, // group opt-in not yet exposed; default off
		Tier:         brain.Tier(tier),
		Incoming:     incoming,
		Window:       toTurns(b.Window),
		ContactName:  contact.Name,
		Language:     contact.Language,
		Style:        contact.Style,
		Summary:      contact.Summary,
		DailyContext: daily,
	}

	out, err := c.brn.Decide(ctx, in)
	if err != nil {
		c.notify("error", "AI error", err.Error())
		return
	}

	if out.MemoryUpdate != "" {
		_ = c.db.UpdateSummary(ctx, b.ChatJID, mergeSummary(contact.Summary, out.MemoryUpdate))
	}

	switch out.Action {
	case brain.ActSend:
		if err := c.gate.Enqueue(safety.Job{ToJID: b.ChatJID, Text: out.Text}); err != nil {
			// Paused/full — keep the reply as a draft so it isn't lost.
			c.queueDraft(b.ChatJID, in.SenderName, incoming, out.Text, "send blocked: "+err.Error(), out.Confidence)
		}
	case brain.ActDraft:
		c.queueDraft(b.ChatJID, in.SenderName, incoming, out.Text, out.Reason, out.Confidence)
	case brain.ActNotify:
		_ = c.db.LogActivity(ctx, "flag", b.ChatJID, out.Reason)
		body := out.Reason
		if out.Text != "" {
			body += "\nSuggested: " + out.Text
		}
		c.notify("notify", senderName, body)
	case brain.ActSilent:
		// nothing to do
	}
}

func (c *Core) queueDraft(chatJID, senderName, incoming, reply, reason string, conf float64) {
	_, _ = c.db.CreateDraft(c.ctx, store.Draft{
		ChatJID: chatJID, SenderJID: chatJID, SenderName: senderName,
		Incoming: incoming, Reply: reply, Reason: reason, Confidence: conf,
	})
	_ = c.db.LogActivity(c.ctx, "draft", chatJID, reason)
	c.notify("draft", "Draft needs review", fmt.Sprintf("%s: %s", senderName, preview(reply)))
}

// ---- self-chat commands -------------------------------------------------

func (c *Core) handleSelfInput(in Inbound) {
	cmd := commands.Parse(in.Text)
	switch cmd.Kind {
	case commands.Pause:
		c.Pause()
		c.confirmSelf("⏸️ Bot paused. /resume to continue.")
	case commands.Resume:
		c.Resume()
		c.confirmSelf("▶️ Bot resumed.")
	case commands.Status:
		c.confirmSelf(c.Status())
	case commands.Help:
		c.confirmSelf(helpText)
	case commands.Today:
		_ = c.db.SetDailyContext(c.ctx, time.Now().Format("2006-01-02"), cmd.Text)
		c.confirmSelf("📝 Got it — today's context updated.")
	case commands.Tier:
		c.applyTier(cmd.Target, cmd.Tier)
	case commands.Note:
		c.appendToday(cmd.Text)
		c.confirmSelf("📝 Noted for today.")
	case commands.Unknown:
		c.confirmSelf("Unknown command. " + helpText)
	}
}

func (c *Core) applyTier(target, tier string) {
	jid := c.resolveContact(target)
	contact, err := c.db.GetContact(c.ctx, jid)
	if err != nil {
		contact = store.Contact{JID: jid, Name: target}
	}
	contact.Tier = store.TrustTier(tier)
	if err := c.db.UpsertContact(c.ctx, contact); err != nil {
		c.confirmSelf("Couldn't update tier: " + err.Error())
		return
	}
	c.confirmSelf(fmt.Sprintf("✅ %s set to %s.", target, tier))
}

// resolveContact maps a name to a JID (case-insensitive) or returns the input
// unchanged if it already looks like a JID.
func (c *Core) resolveContact(target string) string {
	if strings.Contains(target, "@") {
		return target
	}
	contacts, _ := c.db.ListContacts(c.ctx)
	for _, ct := range contacts {
		if strings.EqualFold(ct.Name, target) {
			return ct.JID
		}
	}
	return target
}

func (c *Core) appendToday(text string) {
	day := time.Now().Format("2006-01-02")
	existing, _ := c.db.GetDailyContext(c.ctx, day)
	if existing != "" {
		text = existing + "\n" + text
	}
	_ = c.db.SetDailyContext(c.ctx, day, text)
}

func (c *Core) confirmSelf(text string) {
	if c.selfJID == "" || c.send == nil {
		return
	}
	_ = c.send(c.ctx, c.selfJID, text)
}

// ---- approvals ----------------------------------------------------------

// ListDrafts returns pending drafts.
func (c *Core) ListDrafts() ([]store.Draft, error) {
	return c.db.ListDrafts(c.ctx, store.DraftPending)
}

// ApproveDraft sends a draft (optionally edited) and marks it sent.
func (c *Core) ApproveDraft(id int64, edited string) error {
	dr, err := c.db.GetDraft(c.ctx, id)
	if err != nil {
		return err
	}
	text := dr.Reply
	if strings.TrimSpace(edited) != "" {
		text = edited
	}
	// User explicitly approved -> bypass quiet hours.
	if err := c.gate.Enqueue(safety.Job{ToJID: dr.ChatJID, Text: text, BypassQuiet: true}); err != nil {
		return err
	}
	_ = c.db.SetDraftStatus(c.ctx, id, store.DraftSent, edited)
	_ = c.db.LogActivity(c.ctx, "sent", dr.ChatJID, "approved draft sent")
	return nil
}

// RejectDraft discards a draft.
func (c *Core) RejectDraft(id int64) error {
	return c.db.SetDraftStatus(c.ctx, id, store.DraftRejected, "")
}

// ---- passthrough accessors for the UI -----------------------------------

func (c *Core) ListContacts() ([]store.Contact, error)      { return c.db.ListContacts(c.ctx) }
func (c *Core) UpsertContact(ct store.Contact) error        { return c.db.UpsertContact(c.ctx, ct) }
func (c *Core) ListActivity(limit int) ([]store.Activity, error) {
	return c.db.ListActivity(c.ctx, limit)
}

// SetToday sets today's context from the UI.
func (c *Core) SetToday(text string) error {
	return c.db.SetDailyContext(c.ctx, time.Now().Format("2006-01-02"), text)
}

// GetSettings returns the current settings.
func (c *Core) GetSettings() Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings
}

// SaveSettings persists settings and rebuilds the affected components.
func (c *Core) SaveSettings(s Settings) error {
	if err := saveSettings(c.ctx, c.db, s); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = s
	c.mu.Unlock()
	// Rebuild registry/stt/brain; the running gate keeps its config until restart.
	reg := buildRegistry(s.Providers)
	c.mu.Lock()
	c.brn = brain.New(reg, brainConfig(s))
	c.stt = buildSTT(s.STT)
	c.mu.Unlock()
	return nil
}

// Pause / Resume / Paused proxy the gate kill-switch.
func (c *Core) Pause()       { c.gate.Pause() }
func (c *Core) Resume()      { c.gate.Resume() }
func (c *Core) Paused() bool { return c.gate.Paused() }

// Status returns a short human-readable status line.
func (c *Core) Status() string {
	state := "active"
	if c.gate.Paused() {
		state = "paused"
	}
	pending, _ := c.db.ListDrafts(c.ctx, store.DraftPending)
	return fmt.Sprintf("Bot is %s · %d draft(s) awaiting review.", state, len(pending))
}

const helpText = "Commands: /pause, /resume, /status, /today <text>, /tier <name> <auto|draft|notify>. " +
	"Any other message becomes a note for today."
