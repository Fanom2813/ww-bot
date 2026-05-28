// Package inbox coalesces bursts of incoming WhatsApp messages so the bot reacts
// to a complete thought rather than each message. For every chat it buffers
// messages and flushes the batch once the sender has been quiet for Quiet
// (capped at MaxWait since the first message). If the user replies manually
// (IsFromMe), any pending bot response for that chat is cancelled (human
// takeover). Conversation history/context lives in the store, not here.
package inbox

import (
	"sync"
	"time"
)

// Msg is a normalized inbound message handed to the debouncer.
type Msg struct {
	ChatJID    string
	SenderJID  string
	PushName   string
	Text       string
	Kind       string // text | voice | image | other
	IsFromMe   bool
	IsGroup    bool
	MentionsMe bool // group messages: bot's own number was @-mentioned
	Timestamp  time.Time
}

// Batch is a coalesced burst of messages to respond to.
type Batch struct {
	ChatJID  string
	Messages []Msg
}

// Config tunes the debouncer.
type Config struct {
	Quiet   time.Duration // wait this long after the last message
	MaxWait time.Duration // never hold longer than this since the first message
}

// Debouncer coalesces messages per chat and invokes onFlush with each batch.
type Debouncer struct {
	cfg     Config
	onFlush func(Batch)

	mu    sync.Mutex
	chats map[string]*chatState
}

type chatState struct {
	buffer  []Msg
	timer   *time.Timer
	firstAt time.Time
}

// New creates a debouncer. onFlush is called (on its own goroutine) for each
// coalesced batch. Zero config values fall back to sensible defaults.
func New(cfg Config, onFlush func(Batch)) *Debouncer {
	if cfg.Quiet <= 0 {
		cfg.Quiet = 30 * time.Second
	}
	if cfg.MaxWait <= 0 {
		cfg.MaxWait = 3 * time.Minute
	}
	return &Debouncer{cfg: cfg, onFlush: onFlush, chats: make(map[string]*chatState)}
}

// Add records an incoming message, (re)scheduling the chat's flush.
func (d *Debouncer) Add(m Msg) {
	d.mu.Lock()
	defer d.mu.Unlock()

	st := d.chats[m.ChatJID]
	if st == nil {
		st = &chatState{}
		d.chats[m.ChatJID] = st
	}

	// Human takeover: the user replied themselves — drop any pending response.
	if m.IsFromMe {
		st.buffer = nil
		st.firstAt = time.Time{}
		if st.timer != nil {
			st.timer.Stop()
			st.timer = nil
		}
		return
	}

	st.buffer = append(st.buffer, m)
	if st.firstAt.IsZero() {
		st.firstAt = time.Now()
	}

	// Flush after Quiet, but never later than MaxWait since the first message.
	delay := d.cfg.Quiet
	if remain := time.Until(st.firstAt.Add(d.cfg.MaxWait)); remain < delay {
		if remain < 0 {
			remain = 0
		}
		delay = remain
	}
	if st.timer != nil {
		st.timer.Stop()
	}
	chat := m.ChatJID
	st.timer = time.AfterFunc(delay, func() { d.flush(chat) })
}

// flush emits the buffered batch for a chat, if any.
func (d *Debouncer) flush(chatJID string) {
	d.mu.Lock()
	st := d.chats[chatJID]
	if st == nil || len(st.buffer) == 0 {
		d.mu.Unlock()
		return
	}
	batch := Batch{ChatJID: chatJID, Messages: st.buffer}
	st.buffer = nil
	st.firstAt = time.Time{}
	st.timer = nil
	d.mu.Unlock()

	d.onFlush(batch)
}
