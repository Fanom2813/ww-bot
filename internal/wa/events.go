package wa

import "time"

// Event is a normalized WhatsApp event emitted on Client.Events(). It is
// deliberately decoupled from whatsmeow's own types so the rest of the app
// (and the UI) never imports whatsmeow directly.
type Event interface{ isWAEvent() }

// QR is emitted repeatedly while waiting to be paired. Render Code as a QR
// code for the user to scan with their phone.
type QR struct{ Code string }

// Paired is emitted once the device successfully links to an account.
type Paired struct{ JID string }

// Connected is emitted when the socket connects to WhatsApp servers. JID carries
// the paired account's own number JID so the UI can show it on reconnect (the
// Paired event only fires during a fresh QR scan).
type Connected struct{ JID string }

// Disconnected is emitted when the socket drops (network loss, server close).
// whatsmeow auto-reconnects; a Connected event follows once it recovers.
type Disconnected struct{}

// LoggedOut is emitted when the session is invalidated; the user must re-pair.
type LoggedOut struct{}

// PairingExpired is emitted when the QR channel closes without a successful
// scan (timeout or error). The frontend should show a retry button.
type PairingExpired struct{}

// MessageKind is a coarse classification of an inbound message.
type MessageKind string

const (
	KindText  MessageKind = "text"
	KindVoice MessageKind = "voice"
	KindImage MessageKind = "image"
	KindOther MessageKind = "other"
)

// Message is a normalized inbound message. Raw message content is NOT persisted
// by this package; consumers decide what to keep (per the privacy model).
type Message struct {
	ChatJID   string
	SenderJID string
	PushName  string
	Kind      MessageKind
	Text      string
	Timestamp time.Time
	IsFromMe  bool
	IsGroup   bool
	// MentionsMe is true when the message text @-mentions the bot's own number.
	// Used to honour the "mentions only" group opt-in mode.
	MentionsMe bool
	// AudioData holds the decrypted bytes of a voice note (Kind == KindVoice),
	// downloaded eagerly so consumers can transcribe it. Empty otherwise.
	AudioData []byte
	AudioMime string
}

// Call is a normalized inbound call offer. WhatsApp calls cannot be answered by
// a linked device, so this exists only to notify the user.
type Call struct {
	FromJID string
	Video   bool
	Group   bool
}

func (QR) isWAEvent()             {}
func (Paired) isWAEvent()         {}
func (Connected) isWAEvent()      {}
func (Disconnected) isWAEvent()   {}
func (LoggedOut) isWAEvent()      {}
func (PairingExpired) isWAEvent() {}
func (Message) isWAEvent()        {}
func (Call) isWAEvent()           {}
