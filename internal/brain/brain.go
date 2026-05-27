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
	"strings"

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
}

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
	Action       string   `json:"action"` // reply | escalate | silent
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
	case "escalate":
		out.Action = ActNotify
		out.Text = dec.Text
		out.Reason = orDefault(dec.Reason, "model was unsure")
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

// ask builds the prompt, calls the model, and parses the JSON decision.
func (b *Brain) ask(ctx context.Context, in Input) (decision, error) {
	// Defense-in-depth: scrub credential-looking tokens from everything that
	// reaches the model — the incoming burst, the context window, and the
	// rolling memory. in is a value copy, so this never affects callers/storage.
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

	system := buildSystemPrompt(in)
	user := buildUserPrompt(in)

	raw, _, err := b.reg.Complete(ctx, llm.Request{
		System:      system,
		Messages:    []llm.Message{{Role: llm.User, Content: user}},
		Temperature: 0.3,
		MaxTokens:   600,
	})
	if err != nil {
		return decision{}, err
	}
	dec, err := parseDecision(raw)
	if err != nil {
		// If the model didn't return valid JSON, escalate rather than guess.
		return decision{Action: "escalate", Reason: "model response was not valid JSON"}, nil
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
