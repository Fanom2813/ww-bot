// Package brain is the decision orchestrator. Given a coalesced batch of
// incoming messages plus the contact's profile/summary and the user's daily
// context, it applies guardrails, asks the LLM for a strict JSON decision, and
// returns an Outcome telling the caller what to do: send, draft-for-approval,
// notify, or stay silent. It depends only on the llm package so it stays
// model-agnostic and unit-testable.
package brain

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"wwbot/internal/llm"
)

// Tier mirrors the contact trust tiers (values match internal/store).
type Tier string

const (
	TierAuto   Tier = "auto"
	TierDraft  Tier = "draft"
	TierNotify Tier = "notify"
)

// Turn is one message in the recent context window.
type Turn struct {
	FromMe bool
	Name   string
	Text   string
	At     time.Time // when the message was sent (zero if unknown)
}

// Input is everything the brain needs to make a decision for one batch.
type Input struct {
	SenderName   string
	IsGroup      bool
	GroupOptIn   bool
	Tier         Tier
	Incoming     string // the coalesced burst to respond to
	Window       []Turn // recent context (most recent last)
	ContactName  string
	Language     string
	Style        string
	Summary      string // rolling per-contact memory
	DailyContext string // the user's plan for today
}

// Action is what the caller should do with the outcome.
type Action string

const (
	ActSend   Action = "send"   // auto-send Text
	ActDraft  Action = "draft"  // queue Text for human approval
	ActNotify Action = "notify" // notify the user; do not reply
	ActSilent Action = "silent" // do nothing
)

// Outcome is the brain's verdict for a batch.
type Outcome struct {
	Action       Action
	Text         string  // reply text (send/draft)
	Reason       string  // why (draft/notify/silent)
	Confidence   float64 // model confidence 0..1
	MemoryUpdate string  // summary delta to persist (if any)
	Flags        []string
}

// Config tunes the brain.
type Config struct {
	MinConfidence float64 // below this, replies are escalated to a draft (default 0.7)
	Persona       string  // user-editable system prompt; falls back to DefaultPersona
	Proactive     string  // user-editable proactive prompt; falls back to DefaultProactivePrompt
}

// DefaultProactivePrompt guides bot-initiated (proactive) messages. Editable in
// settings; the persona still applies on top.
const DefaultProactivePrompt = `You are PROACTIVELY starting or continuing this WhatsApp conversation right now — the owner asked you to reach out; the other person did NOT just message you.

- Look at the conversation history and the current date/time.
- If you've never talked, open with a short, natural greeting.
- If it's been a while, pick up naturally — you may lightly acknowledge the gap, but don't make it weird or overly about dates.
- Keep it casual, warm, and human, in the owner's voice — a real opener, not a generic "Hello, how are you?".`

// DefaultPersona is the built-in writing/behavior guidance. It governs HOW the
// bot writes (natural, in the user's voice, plain WhatsApp text). The strict
// JSON output contract is appended separately and is NOT user-editable, so
// customizing this can never break response parsing.
const DefaultPersona = `You are replying to WhatsApp messages ON BEHALF OF the account owner — as if you are them, not an assistant.

Write like a real person texting:
- Sound completely natural and human. Never robotic, formal, or "AI-like".
- Mirror how this contact and the owner actually text: tone, language, length, punctuation, emoji, slang, capitalization.
- Keep it short — usually one or two lines, like a real WhatsApp reply.
- Reply in the same language the contact is using.
- Output PLAIN TEXT only. No markdown headings, code blocks, JSON, or bullet lists unless the person texts that way. Light WhatsApp formatting (*bold*, _italic_) is fine, sparingly.
- NEVER reveal or imply you are an AI, bot, or automated — unless the per-contact instructions explicitly tell you to.

Stay safe:
- If you don't have enough information to answer accurately, don't guess.
- Never make real-world commitments (meetings, money, yes/no to invitations) on the owner's behalf.
- If a message looks like a scam, phishing, or impersonation, flag it.
- It's better to stay silent or escalate than to send something wrong.`

// Brain makes reply decisions using an LLM registry.
type Brain struct {
	reg *llm.Registry
	cfg Config
}

// New constructs a Brain.
func New(reg *llm.Registry, cfg Config) *Brain {
	if cfg.MinConfidence == 0 {
		cfg.MinConfidence = 0.7
	}
	return &Brain{reg: reg, cfg: cfg}
}

// decision is the strict JSON contract the model must return.
type decision struct {
	Action       string   `json:"action"` // reply | draft | notify | silent (escalate = draft)
	Text         string   `json:"text"`
	Confidence   float64  `json:"confidence"`
	MemoryUpdate string   `json:"memory_update"`
	Reason       string   `json:"reason"`
	Flags        []string `json:"flags"`
}

// Decide runs the guardrails and the model, returning what to do.
func (b *Brain) Decide(ctx context.Context, in Input) (Outcome, error) {
	// Pre-model guardrails (cheap, never call the LLM when they trip).
	if in.IsGroup && !in.GroupOptIn {
		return Outcome{Action: ActSilent, Reason: "group chat (not opted in)"}, nil
	}
	// Note: secrets are stripped from the text (see ask → Redact), so the message
	// is processed normally — only the credential itself never reaches the model.
	if in.Tier == TierNotify {
		return Outcome{Action: ActNotify, Reason: "contact is notify-only"}, nil
	}

	dec, err := b.ask(ctx, in)
	if err != nil {
		return Outcome{}, err
	}

	out := Outcome{Confidence: dec.Confidence, MemoryUpdate: dec.MemoryUpdate, Flags: dec.Flags}

	switch strings.ToLower(strings.TrimSpace(dec.Action)) {
	case "silent":
		out.Action = ActSilent
		out.Reason = orDefault(dec.Reason, "model chose silence")
		return out, nil
	case "notify":
		// The model decided this only needs the owner's awareness (FYI / heads-up),
		// not a reply. Any suggested text rides along in the notification.
		out.Action = ActNotify
		out.Text = dec.Text
		out.Reason = orDefault(dec.Reason, "flagged for your attention")
		return out, nil
	case "escalate", "draft":
		// The model wants the user to handle it (unsure, or needs info only the
		// owner has). Queue it as a draft so it lands in the in-app approvals
		// queue where the user can answer/edit and send — not just a fleeting
		// notification.
		out.Action = ActDraft
		out.Text = dec.Text
		out.Reason = orDefault(dec.Reason, "needs your input")
		return out, nil
	}

	// action == "reply": apply post-model guardrails.
	out.Text = dec.Text

	if hasFlag(dec.Flags, "scam") {
		out.Action = ActNotify
		out.Reason = "possible scam flagged"
		return out, nil
	}
	if IsCommitment(dec.Text, dec.Flags) {
		out.Action = ActDraft
		out.Reason = "possible commitment on your behalf — confirm before sending"
		return out, nil
	}
	if dec.Confidence < b.cfg.MinConfidence {
		out.Action = ActDraft
		out.Reason = fmt.Sprintf("low confidence (%.2f)", dec.Confidence)
		return out, nil
	}

	// High-confidence, safe reply: route by trust tier.
	if in.Tier == TierAuto {
		out.Action = ActSend
	} else {
		out.Action = ActDraft
		out.Reason = "draft-and-approve contact"
	}
	return out, nil
}

// redactInput scrubs credential-looking tokens from everything that reaches the
// model (incoming, window, memory, daily plan). in is a value copy.
func redactInput(in Input) Input {
	in.Incoming = Redact(in.Incoming)
	in.Summary = Redact(in.Summary)
	in.DailyContext = Redact(in.DailyContext)
	if len(in.Window) > 0 {
		w := make([]Turn, len(in.Window))
		for i, t := range in.Window {
			t.Text = Redact(t.Text)
			w[i] = t
		}
		in.Window = w
	}
	return in
}

// Initiate generates a proactive opener for a contact — either a free opener
// (topic == "") or one about the given topic. Returns the message text (which
// may contain --- split markers); the caller paces/sends it.
func (b *Brain) Initiate(ctx context.Context, in Input, topic string) (string, error) {
	in = redactInput(in)
	persona := strings.TrimSpace(b.cfg.Persona)
	if persona == "" {
		persona = DefaultPersona
	}
	proactive := strings.TrimSpace(b.cfg.Proactive)
	if proactive == "" {
		proactive = DefaultProactivePrompt
	}
	system := buildProactiveSystem(in, persona, proactive, topic)
	user := buildProactiveUser(in, topic)
	raw, _, err := b.reg.Complete(ctx, llm.Request{
		System:      system,
		Messages:    []llm.Message{{Role: llm.User, Content: user}},
		Temperature: 0.6,
		MaxTokens:   400,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(raw), nil
}

// ask builds the prompt, calls the model, and parses the JSON decision.
func (b *Brain) ask(ctx context.Context, in Input) (decision, error) {
	// Defense-in-depth: scrub credential-looking tokens from everything that
	// reaches the model — the incoming burst, the context window, and the
	// rolling memory. in is a value copy, so this never affects callers/storage.
	in = redactInput(in)

	persona := strings.TrimSpace(b.cfg.Persona)
	if persona == "" {
		persona = DefaultPersona
	}
	system := buildSystemPrompt(in, persona)
	user := buildUserPrompt(in)

	raw, provider, err := b.reg.Complete(ctx, llm.Request{
		System:      system,
		Messages:    []llm.Message{{Role: llm.User, Content: user}},
		Temperature: 0.3,
		JSON:        true,
		MaxTokens:   600,
	})
	if err != nil {
		return decision{}, err
	}
	dec, err := parseDecision(raw)
	if err != nil {
		snippet := raw
		if len(snippet) > 800 {
			snippet = snippet[:800] + "…"
		}
		log.Printf("[brain] non-JSON model response from %q (%d chars): %q", provider, len(raw), snippet)
		// Unparseable output → there's no usable reply, so just notify (don't
		// create an empty draft or guess).
		return decision{Action: "notify", Reason: "model response was not valid JSON"}, nil
	}
	return dec, nil
}

// parseDecision extracts and unmarshals the JSON object from a model response,
// tolerating surrounding prose or markdown code fences.
func parseDecision(raw string) (decision, error) {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			s = s[i : j+1]
		}
	}
	var d decision
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return decision{}, err
	}
	return d, nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
