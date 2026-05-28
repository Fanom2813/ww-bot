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
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"wwbot/internal/brain"
	"wwbot/internal/commands"
	"wwbot/internal/dbg"
	"wwbot/internal/inbox"
	"wwbot/internal/llm"
	"wwbot/internal/safety"
	"wwbot/internal/schedule"
	"wwbot/internal/store"
	"wwbot/internal/stt"
)

// Notifier pushes a user-facing notification (mapped to a UI event by the app).
type Notifier func(level, title, body string)

// UnknownNotifier is called the first time an unsaved number messages the user,
// so the app can prompt to save them. preview is a short excerpt of the message.
type UnknownNotifier func(jid, name, preview string)

// WorkingNotifier is called when the bot has taken a non-silent action on an
// inbound message (reply queued, draft created, or notify) — used by the UI to
// play a subtle sound. Ignored / silent decisions do NOT fire this.
type WorkingNotifier func(chatJID, action string)

// DraftNotifier is called whenever the pending-drafts queue changes (created,
// approved, or rejected) so the UI can refresh without polling.
type DraftNotifier func()

// ActivityNotifier is called whenever a new activity row has been logged, so
// the UI can append it live instead of polling.
type ActivityNotifier func()

// PendingContact is an unsaved number awaiting a save/ignore decision, surfaced
// in the UI as an "action needed".
type PendingContact struct {
	JID     string    `json:"jid"`
	Name    string    `json:"name"`
	Preview string    `json:"preview"`
	At      time.Time `json:"at"`
}

// SendFunc delivers a message to a JID (wraps wa.Client.SendText).
type SendFunc func(ctx context.Context, toJID, text string) error

// Inbound is a normalized incoming message handed to the core by the app.
type Inbound struct {
	ChatJID    string
	SenderJID  string
	PushName   string
	Text       string
	Kind       string // text | voice | image | other
	IsFromMe   bool
	IsGroup    bool
	MentionsMe bool // group messages: bot's own number was @-mentioned
	Timestamp  time.Time
	Audio      []byte // voice-note bytes (Kind == "voice")
}

// Core is the orchestrator.
type Core struct {
	db        *store.DB
	notify    Notifier
	scheduler *schedule.Scheduler

	mu       sync.RWMutex
	settings Settings
	stt      stt.Transcriber
	brn      *brain.Brain
	gate     *safety.Gate
	debounce *inbox.Debouncer

	// Notifier hooks are multi-listener: every OnXxx call appends, so several
	// subsystems (frontend events, dock badge, telemetry, …) can subscribe to
	// the same signal without overwriting each other.
	onUnknown      []UnknownNotifier
	onWorking      []WorkingNotifier
	onDraft        []DraftNotifier
	onActivity     []ActivityNotifier
	pendingUnknown map[string]PendingContact // unsaved numbers awaiting a save/ignore decision
	msgKey         []byte                    // AES key for encrypting stored message history

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
	c := &Core{
		db: db, notify: notify, scheduler: schedule.New(), settings: s, ctx: ctx,
		pendingUnknown: make(map[string]PendingContact),
		msgKey:         loadOrCreateMsgKey(),
	}
	c.build(s)
	return c, nil
}

// All On* helpers are multi-listener: each call appends. Registration is
// guarded by c.mu so it stays race-free even if a goroutine subscribes after
// the run loop starts. Emit paths take a snapshot under RLock and invoke
// listeners *outside* the lock so a slow handler can't block other emits.

// OnUnknownContact subscribes to the first-message-from-an-unsaved-number signal.
func (c *Core) OnUnknownContact(fn UnknownNotifier) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.onUnknown = append(c.onUnknown, fn)
	c.mu.Unlock()
}

// OnWorking subscribes to non-silent decisions (reply / draft / notify) so
// the UI can play a sound or update telemetry.
func (c *Core) OnWorking(fn WorkingNotifier) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.onWorking = append(c.onWorking, fn)
	c.mu.Unlock()
}

// OnDraftQueue subscribes to draft-queue changes (created / approved / rejected).
func (c *Core) OnDraftQueue(fn DraftNotifier) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.onDraft = append(c.onDraft, fn)
	c.mu.Unlock()
}

// OnActivity subscribes to new-audit-row events so the UI can append live.
func (c *Core) OnActivity(fn ActivityNotifier) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	c.onActivity = append(c.onActivity, fn)
	c.mu.Unlock()
}

func (c *Core) emitDraftQueue() {
	c.mu.RLock()
	subs := append([]DraftNotifier(nil), c.onDraft...)
	c.mu.RUnlock()
	for _, fn := range subs {
		fn()
	}
}

// logActivity wraps store.LogActivity to also fire the live-update hook. ALL
// new audit writes should go through this so the Activity page (and Dashboard
// "Recent activity") refresh in real time.
func (c *Core) logActivity(ctx context.Context, kind, chatJID, summary string) error {
	if err := c.db.LogActivity(ctx, kind, chatJID, summary); err != nil {
		return err
	}
	c.mu.RLock()
	subs := append([]ActivityNotifier(nil), c.onActivity...)
	c.mu.RUnlock()
	for _, fn := range subs {
		fn()
	}
	return nil
}

// build (re)constructs the LLM registry, transcriber, brain, gate, and inbox
// from settings. The gate uses a thunk so the sender can be attached later.
func (c *Core) build(s Settings) {
	reg := buildRegistry(hydrateKeys(s.Providers))
	c.wireRegistry(reg)
	c.brn = brain.New(reg, brainConfig(s))
	c.stt = buildSTT(s.STT)
	c.gate = c.newGate(s)
	c.debounce = inbox.New(inbox.Config{
		Quiet: 30 * time.Second, MaxWait: 3 * time.Minute,
	}, c.handleBatch)
}

// newGate builds a safety gate from the current settings.
func (c *Core) newGate(s Settings) *safety.Gate {
	return safety.New(buildSafetyConfig(s.Safety),
		func(ctx context.Context, to, text string) error {
			if c.send == nil {
				return errors.New("core: no sender attached")
			}
			return c.send(ctx, to, text)
		},
		safety.Options{
			OnSent: func(to, text string) {
				dbg.Printf("[core] SENT → %q: %q", to, preview(text))
				// Persist our own reply to history so the bot remembers what it
				// already said across restarts (WhatsApp won't echo it to us).
				c.storeMessage(to, "", text, true)
				_ = c.logActivity(c.ctx, "sent", to, preview(text))
			},
			OnDrop: func(j safety.Job, reason string) {
				dbg.Printf("[core] send suppressed → %q: %s", j.ToJID, reason)
				_ = c.logActivity(c.ctx, "system", j.ToJID, "send suppressed: "+reason)
			},
		})
}

// wireRegistry attaches the fallback hook so a non-primary provider answering
// shows up in the Activity log (and the user knows they're being protected by
// the fallback chain). Logs are also surfaced via stdout for diagnostics.
func (c *Core) wireRegistry(reg *llm.Registry) {
	reg.OnFallback = func(used string, attempts []llm.Attempt) {
		if len(attempts) == 0 {
			return
		}
		primary := attempts[0].Provider
		dbg.Printf("[core] llm fallback: %s failed (%v) → answered by %s", primary, attempts[0].Err, used)
		_ = c.logActivity(c.ctx, "system", "",
			fmt.Sprintf("LLM fallback: %s failed → %s answered", primary, used))
	}
}

// safetyGate returns the current gate (which may be swapped on settings reload).
func (c *Core) safetyGate() *safety.Gate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.gate
}

// applySettings rebuilds everything settings-derived and applies it LIVE — no
// restart needed: the LLM registry/brain, STT, the safety gate (pacing/quiet
// hours), and the scheduled proactive tasks are all re-created from s.
func (c *Core) applySettings(s Settings) {
	reg := buildRegistry(hydrateKeys(s.Providers))
	c.wireRegistry(reg)
	newGate := c.newGate(s)

	c.mu.Lock()
	c.brn = brain.New(reg, brainConfig(s))
	c.stt = buildSTT(s.STT)
	oldGate := c.gate
	wasPaused := oldGate != nil && oldGate.Paused()
	c.gate = newGate
	oldScheduler := c.scheduler
	c.scheduler = schedule.New()
	c.mu.Unlock()

	// Swap the gate: start the new one (preserving pause state), stop the old.
	if c.ctx != nil {
		newGate.Start(c.ctx)
	}
	if wasPaused {
		newGate.Pause()
	}
	if oldGate != nil {
		oldGate.Stop()
	}

	// Re-register greetings on a fresh scheduler.
	if oldScheduler != nil {
		oldScheduler.Stop()
	}
	c.registerSchedules()
	if c.ctx != nil {
		c.scheduler.Start(c.ctx)
	}
}

// AttachSender wires the function used to actually deliver messages.
func (c *Core) AttachSender(send SendFunc) { c.send = send }

// SetSelfJID records the user's own JID (to recognize self-chat commands).
func (c *Core) SetSelfJID(jid string) { c.selfJID = jid }

// Start launches the gate worker and the scheduler.
func (c *Core) Start(ctx context.Context) {
	c.ctx = ctx
	c.gate.Start(ctx)
	c.registerSchedules()
	c.scheduler.Start(ctx)
}

// Stop stops background workers.
func (c *Core) Stop() {
	c.gate.Stop()
	c.scheduler.Stop()
}

func (c *Core) registerSchedules() {
	c.mu.RLock()
	schedules := c.settings.Schedules
	c.mu.RUnlock()
	for i, t := range schedules {
		if !t.Enabled || strings.TrimSpace(t.Prompt) == "" || len(t.Contacts) == 0 {
			continue
		}
		t := t
		name := t.ID
		if name == "" {
			name = fmt.Sprintf("sched-%d", i)
		}
		c.scheduler.Daily(name, t.Hour, t.Min, func(ctx context.Context) {
			dbg.Printf("[core] scheduled task %q firing → %d contact(s)", t.Label, len(t.Contacts))
			for _, jid := range t.Contacts {
				if strings.TrimSpace(jid) == "" {
					continue
				}
				if err := c.StartProactive(jid, t.Prompt); err != nil {
					log.Printf("[core] scheduled %q → %q failed: %v", t.Label, jid, err)
				}
			}
		})
	}
}

// OnInbound is the pipeline entry point for an incoming message.
func (c *Core) OnInbound(in Inbound) {
	dbg.Printf("[core] inbound chat=%q sender=%q fromMe=%v group=%v kind=%s self=%q text=%q",
		in.ChatJID, in.SenderJID, in.IsFromMe, in.IsGroup, in.Kind, c.selfJID, preview(brain.Redact(in.Text)))

	// Self-chat commands / daily notes. A self-chat is "note to self": you sent
	// it (IsFromMe) and the chat is your own number. We accept either a match
	// against the stored self-JID or chat==sender, which is robust to JID-format
	// differences (e.g. LID vs phone-number addressing).
	if in.IsFromMe && in.ChatJID != "" &&
		(in.ChatJID == c.selfJID || in.ChatJID == in.SenderJID) {
		dbg.Printf("[core] -> self-command")
		c.handleSelfInput(in)
		return
	}

	text := in.Text
	if in.Kind == "voice" {
		text = c.transcribe(in)
		if text == "" {
			// Couldn't understand the audio confidently — stay silent, just note it.
			_ = c.logActivity(c.ctx, "system", in.ChatJID, "voice note not understood — skipped")
			return
		}
	}

	// ---- Credential gate -------------------------------------------------
	// Single chokepoint for every incoming message: strip any secrets (codes,
	// keys, cards, …) HERE so no raw credential can reach the brain, drafts,
	// activity log, memory, or notifications downstream. When something is
	// stripped we notify you, then the message keeps flowing normally — only the
	// secret itself is replaced with [redacted].
	if clean := brain.Redact(text); clean != text {
		if !in.IsFromMe {
			name := in.PushName
			if name == "" {
				name = "Unknown number"
			}
			_ = c.logActivity(c.ctx, "flag", in.ChatJID, "sensitive data stripped before processing")
			c.notify("notify", name, "Removed sensitive data (codes/credentials) from this message before processing.")
		}
		text = clean
	}

	// Persist to the rolling (encrypted) history so context survives restarts —
	// for 1-1 chats, including the owner's own manual replies (for takeover context).
	if !in.IsGroup && in.ChatJID != "" {
		name := in.PushName
		if in.IsFromMe {
			name = ""
		}
		c.storeMessage(in.ChatJID, name, text, in.IsFromMe)
	}

	// Surface unknown 1-1 senders immediately (don't wait for the reply debounce)
	// so the "save this contact?" action shows up the moment they message.
	if !in.IsFromMe && !in.IsGroup && in.ChatJID != "" {
		if _, err := c.db.GetContact(c.ctx, in.ChatJID); errors.Is(err, store.ErrNotFound) {
			c.flagUnknown(in.ChatJID, in.PushName, text)
		}
	}

	c.debounce.Add(inbox.Msg{
		ChatJID: in.ChatJID, SenderJID: in.SenderJID, PushName: in.PushName,
		Text: text, Kind: in.Kind, IsFromMe: in.IsFromMe,
		IsGroup: in.IsGroup, MentionsMe: in.MentionsMe, Timestamp: in.Timestamp,
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

// historyLimit caps how many recent messages we keep/load per chat, bounding
// both storage and how much context goes to the model.
const historyLimit = 30

// storeMessage appends a message to the chat's rolling, encrypted history.
func (c *Core) storeMessage(chatJID, name, text string, fromMe bool) {
	if chatJID == "" {
		return
	}
	_ = c.db.AddMessage(c.ctx, store.Message{
		ChatJID: chatJID, FromMe: fromMe, Name: name,
		Text: encryptText(c.msgKey, text), Timestamp: time.Now(),
	}, historyLimit)
}

// recentTurns loads and decrypts the recent conversation for a chat as context.
func (c *Core) recentTurns(chatJID string) []brain.Turn {
	msgs, _ := c.db.RecentMessages(c.ctx, chatJID, historyLimit)
	turns := make([]brain.Turn, 0, len(msgs))
	for _, m := range msgs {
		turns = append(turns, brain.Turn{FromMe: m.FromMe, Name: m.Name, Text: decryptText(c.msgKey, m.Text), At: m.Timestamp})
	}
	return turns
}

// handleBatch runs the brain on a coalesced batch and routes the outcome.
func (c *Core) handleBatch(b inbox.Batch) {
	ctx := c.ctx
	contact, err := c.db.GetContact(ctx, b.ChatJID)
	unknown := errors.Is(err, store.ErrNotFound)

	c.mu.RLock()
	guestMode := c.settings.GuestMode
	guestTier := c.settings.GuestTier
	c.mu.RUnlock()

	dbg.Printf("[core] handleBatch chat=%q msgs=%d unknown=%v group=%v guestMode=%v",
		b.ChatJID, len(b.Messages), unknown, isGroup(b), guestMode)

	if unknown && !isGroup(b) {
		// The save prompt was already queued in OnInbound; this is a safety net.
		// Whether the bot also REPLIES depends on guest mode: off → stop here;
		// on → fall through and handle them at the configured guest tier.
		var name string
		for _, m := range b.Messages {
			if m.PushName != "" {
				name = m.PushName
				break
			}
		}
		incoming, _ := summarizeBurst(b, "")
		c.flagUnknown(b.ChatJID, name, incoming)
		if !guestMode {
			dbg.Printf("[core] chat=%q unknown sender + guest mode OFF → no reply (save prompt only)", b.ChatJID)
			return
		}
		dbg.Printf("[core] chat=%q unknown sender, guest mode ON → replying at tier=%s", b.ChatJID, guestTier)
	}

	tier := contact.Tier
	if tier == "" {
		if unknown && !isGroup(b) {
			tier = store.TrustTier(guestTier) // guest-mode reply tier for strangers
		} else {
			tier = store.TierNotify
		}
	}
	if tier == "" {
		tier = store.TierNotify
	}

	incoming, senderName := summarizeBurst(b, contact.Name)
	day := time.Now().Format("2006-01-02")
	daily, _ := c.db.GetDailyContext(ctx, day)

	// For group chats, look up the per-group opt-in (default: off). The brain's
	// pre-model guardrail stays silent unless this is true, so a fresh group
	// you've never enabled is silently ignored. "mentions" requires that one of
	// the messages in the burst @-mentioned the bot's own number.
	groupOptIn := false
	if isGroup(b) {
		switch c.db.GetGroupMode(ctx, b.ChatJID) {
		case store.GroupAlways:
			groupOptIn = true
		case store.GroupMentions:
			for _, m := range b.Messages {
				if m.MentionsMe {
					groupOptIn = true
					break
				}
			}
		}
	}

	in := brain.Input{
		SenderName:   senderName,
		IsGroup:      isGroup(b),
		GroupOptIn:   groupOptIn,
		Tier:         brain.Tier(tier),
		Incoming:     incoming,
		Window:       c.recentTurns(b.ChatJID),
		ContactName:  contact.Name,
		Language:     contact.Language,
		Style:        contact.Style,
		Summary:      contact.Summary,
		DailyContext: daily,
	}

	dbg.Printf("[core] deciding chat=%q tier=%s incoming=%q", b.ChatJID, tier, preview(incoming))
	out, err := c.brn.Decide(ctx, in)
	if err != nil {
		log.Printf("[core] brain error chat=%q: %v (no provider enabled?)", b.ChatJID, err)
		c.notify("error", "AI error", err.Error())
		return
	}
	dbg.Printf("[core] decision chat=%q action=%s confidence=%.2f reason=%q", b.ChatJID, out.Action, out.Confidence, out.Reason)

	if out.MemoryUpdate != "" {
		_ = c.db.UpdateSummary(ctx, b.ChatJID, mergeSummary(contact.Summary, out.MemoryUpdate))
	}

	// Fire the "working" hook for any non-silent action so the UI can play a
	// subtle sound. Silent / ignored decisions don't fire.
	if out.Action != brain.ActSilent {
		c.mu.RLock()
		subs := append([]WorkingNotifier(nil), c.onWorking...)
		c.mu.RUnlock()
		for _, fn := range subs {
			fn(b.ChatJID, string(out.Action))
		}
	}

	switch out.Action {
	case brain.ActSend:
		if err := c.safetyGate().Enqueue(safety.Job{ToJID: b.ChatJID, Parts: splitMessages(out.Text)}); err != nil {
			// Paused/full — keep the reply as a draft so it isn't lost.
			log.Printf("[core] send blocked chat=%q: %v → queued as draft", b.ChatJID, err)
			c.queueDraft(b.ChatJID, in.SenderName, incoming, out.Text, "send blocked: "+err.Error(), out.Confidence)
		} else {
			dbg.Printf("[core] reply queued for paced send → chat=%q", b.ChatJID)
		}
	case brain.ActDraft:
		c.queueDraft(b.ChatJID, in.SenderName, incoming, out.Text, out.Reason, out.Confidence)
	case brain.ActNotify:
		_ = c.logActivity(ctx, "flag", b.ChatJID, out.Reason)
		body := out.Reason
		if out.Text != "" {
			body += "\nSuggested: " + out.Text
		}
		c.notify("notify", senderName, body)
	case brain.ActSilent:
		reason := out.Reason
		if reason == "" {
			reason = "decided no reply was needed"
		}
		_ = c.logActivity(ctx, "silent", b.ChatJID, reason)
	}
}

// promptUnknown fires the save-this-contact prompt the first time an unsaved
// number messages the user. It dedupes per JID so repeated messages from the
// same unknown number don't re-prompt.
// flagUnknown records an unsaved number as a pending save/ignore action and
// notifies the UI, once per number (deduped while it stays pending).
func (c *Core) flagUnknown(jid, name, text string) {
	c.mu.Lock()
	_, already := c.pendingUnknown[jid]
	c.mu.Unlock()
	if already {
		return
	}
	pc := PendingContact{JID: jid, Name: name, Preview: preview(text), At: time.Now()}

	c.mu.Lock()
	c.pendingUnknown[jid] = pc
	c.mu.Unlock()

	_ = c.logActivity(c.ctx, "flag", jid, "new number — save prompt")
	dbg.Printf("[core] new-number prompt queued for %s", jid)
	c.mu.RLock()
	subs := append([]UnknownNotifier(nil), c.onUnknown...)
	c.mu.RUnlock()
	for _, fn := range subs {
		fn(pc.JID, pc.Name, pc.Preview)
	}
}

// PendingContacts returns unsaved numbers awaiting a save/ignore decision,
// newest first.
func (c *Core) PendingContacts() []PendingContact {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PendingContact, 0, len(c.pendingUnknown))
	for _, p := range c.pendingUnknown {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out
}

// DismissContact removes a number from the pending list without saving it.
func (c *Core) DismissContact(jid string) {
	c.mu.Lock()
	delete(c.pendingUnknown, jid)
	c.mu.Unlock()
}

func (c *Core) queueDraft(chatJID, senderName, incoming, reply, reason string, conf float64) {
	_, _ = c.db.CreateDraft(c.ctx, store.Draft{
		ChatJID: chatJID, SenderJID: chatJID, SenderName: senderName,
		Incoming: incoming, Reply: reply, Reason: reason, Confidence: conf,
	})
	_ = c.logActivity(c.ctx, "draft", chatJID, reason)
	c.notify("draft", "Draft needs review", fmt.Sprintf("%s: %s", senderName, preview(reply)))
	c.emitDraftQueue()
}

// ---- self-chat commands -------------------------------------------------

func (c *Core) handleSelfInput(in Inbound) {
	cmd := commands.Parse(in.Text)
	var summary string
	switch cmd.Kind {
	case commands.Pause:
		c.Pause()
		c.confirmSelf("⏸️ Bot paused. /resume to continue.")
		summary = "/pause — auto-replies stopped"
	case commands.Resume:
		c.Resume()
		c.confirmSelf("▶️ Bot resumed.")
		summary = "/resume — auto-replies on"
	case commands.Status:
		c.confirmSelf(c.Status())
		summary = "/status"
	case commands.Help:
		c.confirmSelf(helpText)
		summary = "/help"
	case commands.Today:
		c.SetToday(cmd.Text)
		c.confirmSelf("📝 Got it — today's context updated.")
		summary = "/today — set: " + preview(brain.Redact(cmd.Text))
	case commands.Tier:
		c.applyTier(cmd.Target, cmd.Tier)
		summary = fmt.Sprintf("/tier %s → %s", cmd.Target, cmd.Tier)
	case commands.Note:
		c.appendToday(cmd.Text)
		c.confirmSelf("📝 Noted for today.")
		summary = "note added to today: " + preview(brain.Redact(cmd.Text))
	case commands.Unknown:
		c.confirmSelf("Unknown command. " + helpText)
		summary = "unknown command: " + preview(in.Text)
	}
	if summary != "" {
		_ = c.logActivity(c.ctx, "command", c.selfJID, summary)
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
	text = brain.Redact(text)
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
	if strings.TrimSpace(text) == "" {
		return errors.New("core: cannot send an empty reply — type a message first")
	}
	// User explicitly approved -> bypass quiet hours.
	if err := c.safetyGate().Enqueue(safety.Job{ToJID: dr.ChatJID, Parts: splitMessages(text), BypassQuiet: true}); err != nil {
		return err
	}
	_ = c.db.SetDraftStatus(c.ctx, id, store.DraftSent, edited)
	_ = c.logActivity(c.ctx, "sent", dr.ChatJID, "approved draft sent")
	c.emitDraftQueue()
	return nil
}

// ApproveDraftsFromContact approves every pending draft for a chat in order
// (oldest first), letting the safety gate pace them. Returns the count sent.
func (c *Core) ApproveDraftsFromContact(chatJID string) (int, error) {
	all, err := c.db.ListDrafts(c.ctx, store.DraftPending)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, d := range all {
		if d.ChatJID != chatJID {
			continue
		}
		if err := c.ApproveDraft(d.ID, ""); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// RejectDraft discards a draft.
func (c *Core) RejectDraft(id int64) error {
	err := c.db.SetDraftStatus(c.ctx, id, store.DraftRejected, "")
	if err == nil {
		c.emitDraftQueue()
	}
	return err
}

// ---- passthrough accessors for the UI -----------------------------------

func (c *Core) ListContacts() ([]store.Contact, error) { return c.db.ListContacts(c.ctx) }
func (c *Core) UpsertContact(ct store.Contact) error {
	c.DismissContact(ct.JID) // saving resolves any pending "new number" prompt
	return c.db.UpsertContact(c.ctx, ct)
}
func (c *Core) ListActivity(limit int) ([]store.Activity, error) {
	return c.db.ListActivity(c.ctx, limit)
}

// ListGroups returns the saved per-group opt-in rows.
func (c *Core) ListGroups() ([]store.Group, error) { return c.db.ListGroups(c.ctx) }

// SaveGroup upserts a group's opt-in mode (off / always). Empty mode → off.
func (c *Core) SaveGroup(g store.Group) error { return c.db.UpsertGroup(c.ctx, g) }

// CountActivityToday returns the number of audit entries logged since local midnight.
func (c *Core) CountActivityToday() (int, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return c.db.CountActivitySince(c.ctx, start)
}

// SetToday sets today's context from the UI (with any secrets scrubbed).
func (c *Core) SetToday(text string) error {
	return c.db.SetDailyContext(c.ctx, time.Now().Format("2006-01-02"), brain.Redact(text))
}

// DefaultSystemPrompt returns the built-in persona used when none is set, so the
// UI can show/restore it.
func (c *Core) DefaultSystemPrompt() string { return brain.DefaultPersona }

// DefaultProactivePrompt returns the built-in proactive prompt.
func (c *Core) DefaultProactivePrompt() string { return brain.DefaultProactivePrompt }

// Schedules returns the configured scheduled proactive tasks.
func (c *Core) Schedules() []ScheduledTask {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings.Schedules
}

// SaveSchedules replaces the scheduled tasks and re-registers them live.
func (c *Core) SaveSchedules(ts []ScheduledTask) error {
	c.mu.RLock()
	s := c.settings
	c.mu.RUnlock()
	s.Schedules = ts
	return c.SaveSettings(s)
}

// StartProactive has the bot reach out to a contact: a free opener (topic == "")
// or one about topic. It reads the chat's history/profile, asks the model for an
// opener, and sends it (paced, may be multi-part). User-triggered, so it bypasses
// quiet hours.
func (c *Core) StartProactive(chatJID, topic string) error {
	if strings.TrimSpace(chatJID) == "" {
		return errors.New("core: proactive requires a contact")
	}
	contact, _ := c.db.GetContact(c.ctx, chatJID)
	daily, _ := c.db.GetDailyContext(c.ctx, time.Now().Format("2006-01-02"))
	in := brain.Input{
		SenderName:   contact.Name,
		ContactName:  contact.Name,
		Language:     contact.Language,
		Style:        contact.Style,
		Summary:      contact.Summary,
		DailyContext: daily,
		Window:       c.recentTurns(chatJID),
	}
	text, err := c.brn.Initiate(c.ctx, in, topic)
	if err != nil {
		log.Printf("[core] proactive error chat=%q: %v", chatJID, err)
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("core: model returned an empty proactive message")
	}
	dbg.Printf("[core] proactive chat=%q topic=%q -> %q", chatJID, topic, preview(text))
	if err := c.safetyGate().Enqueue(safety.Job{ToJID: chatJID, Parts: splitMessages(text), BypassQuiet: true}); err != nil {
		return err
	}
	_ = c.logActivity(c.ctx, "proactive", chatJID, preview(text))
	return nil
}

// Today returns today's saved context (empty if none).
func (c *Core) Today() string {
	txt, _ := c.db.GetDailyContext(c.ctx, time.Now().Format("2006-01-02"))
	return txt
}

// GetSettings returns the current settings for the UI. API keys are never
// included — instead each provider's HasKey reports whether a key exists in the
// OS keychain.
func (c *Core) GetSettings() Settings {
	c.mu.RLock()
	s := c.settings
	c.mu.RUnlock()
	ps := make([]ProviderSetting, len(s.Providers))
	for i, p := range s.Providers {
		p.APIKey = ""
		p.HasKey = p.RequiresKey && hasKey(p.Name)
		ps[i] = p
	}
	s.Providers = ps
	return s
}

// SaveSettings persists settings and rebuilds the affected components. API keys
// are written to the OS keychain (keyed by provider name) and stripped from the
// blob saved to the settings DB.
func (c *Core) SaveSettings(s Settings) error {
	c.mu.RLock()
	old := c.settings.Providers
	c.mu.RUnlock()

	newNames := make(map[string]bool, len(s.Providers))
	for i := range s.Providers {
		p := &s.Providers[i]
		newNames[p.Name] = true
		if p.APIKey != "" {
			storeKey(p.Name, p.APIKey) // new/replacement key → keychain
		}
		p.APIKey = "" // never persist the key in the settings DB
		p.HasKey = false
	}
	// Forget keychain entries for providers the user removed.
	for _, p := range old {
		if p.RequiresKey && !newNames[p.Name] {
			deleteKey(p.Name)
		}
	}

	if err := saveSettings(c.ctx, c.db, s); err != nil {
		return err
	}
	c.mu.Lock()
	c.settings = s
	c.mu.Unlock()
	// Apply everything live — providers/model, persona, STT, safety pacing,
	// quiet hours, and greetings all take effect immediately (no restart).
	c.applySettings(s)
	return nil
}

// Pause / Resume / Paused proxy the gate kill-switch.
func (c *Core) Pause()       { c.safetyGate().Pause() }
func (c *Core) Resume()      { c.safetyGate().Resume() }
func (c *Core) Paused() bool { return c.safetyGate().Paused() }

// Status returns a short human-readable status line.
func (c *Core) Status() string {
	state := "active"
	if c.safetyGate().Paused() {
		state = "paused"
	}
	return fmt.Sprintf("Bot is %s", state)
}

const helpText = "🤖 *WW Bot — commands*\n" +
	"(only work here, in your own chat)\n\n" +
	"/help — show this menu\n" +
	"/status — show bot status\n" +
	"/pause — stop auto-replies\n" +
	"/resume — resume auto-replies\n" +
	"/today <text> — set today's context\n" +
	"/tier <name> <auto|draft|notify> — set how the bot handles a contact\n\n" +
	"Any other text here becomes a note for today."
