package brain

import (
	"context"
	"strings"
	"testing"

	"wwbot/internal/llm"
)

// fakeLLM is a Provider that returns a fixed response and records whether it
// was called (to assert the LLM is NOT hit when a pre-guardrail trips).
type fakeLLM struct {
	resp    string
	called  bool
	lastReq llm.Request
}

func (f *fakeLLM) Name() string                   { return "fake" }
func (f *fakeLLM) Available(context.Context) bool { return true }
func (f *fakeLLM) Complete(_ context.Context, req llm.Request) (string, error) {
	f.called = true
	f.lastReq = req
	return f.resp, nil
}

func brainWith(resp string) (*Brain, *fakeLLM) {
	f := &fakeLLM{resp: resp}
	return New(llm.NewRegistry(f), Config{MinConfidence: 0.7}), f
}

func TestGroupNotOptedInIsSilent(t *testing.T) {
	b, f := brainWith(`{"action":"reply","text":"hi","confidence":0.9}`)
	out, err := b.Decide(context.Background(), Input{IsGroup: true, GroupOptIn: false, Tier: TierAuto, Incoming: "yo"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Action != ActSilent {
		t.Fatalf("want silent, got %s", out.Action)
	}
	if f.called {
		t.Fatal("LLM should not be called for non-opted-in group")
	}
}

func TestSensitiveContentRedactedButProcessed(t *testing.T) {
	b, f := brainWith(`{"action":"reply","text":"sure","confidence":0.9}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "your verification code is 558213"})
	// The message keeps flowing (the bot may reply)...
	if out.Action != ActSend {
		t.Fatalf("want send, got %s", out.Action)
	}
	// ...but the secret must never reach the model.
	if !f.called {
		t.Fatal("LLM should be called for a redacted message")
	}
	for _, m := range f.lastReq.Messages {
		if strings.Contains(m.Content, "558213") {
			t.Fatalf("secret leaked to LLM: %q", m.Content)
		}
	}
}

func TestNotifyTierNeverReplies(t *testing.T) {
	b, f := brainWith(`{"action":"reply","text":"hi","confidence":0.95}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierNotify, Incoming: "hello"})
	if out.Action != ActNotify {
		t.Fatalf("want notify, got %s", out.Action)
	}
	if f.called {
		t.Fatal("notify-only contact should not call the LLM")
	}
}

func TestAutoTierHighConfidenceSends(t *testing.T) {
	b, _ := brainWith(`{"action":"reply","text":"On my way!","confidence":0.95}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "where are you?"})
	if out.Action != ActSend || out.Text != "On my way!" {
		t.Fatalf("want send, got %s text=%q", out.Action, out.Text)
	}
}

func TestDraftTierAlwaysDrafts(t *testing.T) {
	b, _ := brainWith(`{"action":"reply","text":"hello","confidence":0.99}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierDraft, Incoming: "hi"})
	if out.Action != ActDraft {
		t.Fatalf("want draft, got %s", out.Action)
	}
}

func TestLowConfidenceBecomesDraft(t *testing.T) {
	b, _ := brainWith(`{"action":"reply","text":"maybe?","confidence":0.4}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "is the meeting still on?"})
	if out.Action != ActDraft {
		t.Fatalf("want draft for low confidence, got %s", out.Action)
	}
}

func TestCommitmentBecomesDraft(t *testing.T) {
	b, _ := brainWith(`{"action":"reply","text":"Yes, I'll be there at 6pm","confidence":0.95}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "can you come at 6?"})
	if out.Action != ActDraft {
		t.Fatalf("want draft for commitment, got %s", out.Action)
	}
}

func TestScamFlagNotifies(t *testing.T) {
	b, _ := brainWith(`{"action":"reply","text":"...","confidence":0.9,"flags":["scam"]}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "send me money urgently"})
	if out.Action != ActNotify {
		t.Fatalf("want notify for scam, got %s", out.Action)
	}
}

func TestEscalateActionBecomesDraft(t *testing.T) {
	b, _ := brainWith(`{"action":"escalate","text":"draft reply","confidence":0.6,"reason":"unsure"}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "?"})
	if out.Action != ActDraft || out.Text != "draft reply" {
		t.Fatalf("want draft w/ suggested text, got %s text=%q", out.Action, out.Text)
	}
}

func TestNotifyActionNotifies(t *testing.T) {
	b, _ := brainWith(`{"action":"notify","confidence":0.7,"reason":"heads up — looks like a sales pitch"}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "special offer just for you!"})
	if out.Action != ActNotify {
		t.Fatalf("want notify, got %s", out.Action)
	}
}

func TestSilentAction(t *testing.T) {
	b, _ := brainWith(`{"action":"silent","reason":"no reply needed"}`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "ok 👍"})
	if out.Action != ActSilent {
		t.Fatalf("want silent, got %s", out.Action)
	}
}

func TestInvalidJSONEscalates(t *testing.T) {
	b, _ := brainWith(`I think you should say hello`)
	out, _ := b.Decide(context.Background(), Input{Tier: TierAuto, Incoming: "hi"})
	if out.Action != ActNotify {
		t.Fatalf("want notify (escalate) on invalid JSON, got %s", out.Action)
	}
}

func TestParseDecisionWithCodeFence(t *testing.T) {
	d, err := parseDecision("```json\n{\"action\":\"reply\",\"text\":\"hi\",\"confidence\":0.9}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != "reply" || d.Text != "hi" {
		t.Fatalf("bad parse: %+v", d)
	}
}

func TestGuards(t *testing.T) {
	if !IsSensitive("Your OTP is 1234") {
		t.Error("OTP should be sensitive")
	}
	if !IsSensitive("card number 4111111111111111") {
		t.Error("long digits should be sensitive")
	}
	if IsSensitive("see you tomorrow") {
		t.Error("plain text should not be sensitive")
	}
	if !IsCommitment("Sure, I'll be there at 6", nil) {
		t.Error("commitment phrase should trip")
	}
	if !IsCommitment("ok", []string{"commitment"}) {
		t.Error("commitment flag should trip")
	}
	if IsCommitment("haha that's funny", nil) {
		t.Error("casual text should not be a commitment")
	}
}
