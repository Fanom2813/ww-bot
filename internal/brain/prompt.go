package brain

import (
	"fmt"
	"strings"
	"time"
)

// writeContext appends the per-contact context (name, language, style, memory,
// daily plan) shared by the reply and proactive prompts.
func writeContext(b *strings.Builder, in Input) {
	if in.ContactName != "" {
		fmt.Fprintf(b, "\n\nYou are talking to: %s.", in.ContactName)
	}
	if in.Language != "" {
		fmt.Fprintf(b, " Reply in this language: %s.", in.Language)
	}
	if in.Style != "" {
		fmt.Fprintf(b, "\nReply style for this contact: %s", in.Style)
	}
	if in.Summary != "" {
		fmt.Fprintf(b, "\n\nWhat you remember about this person:\n%s", in.Summary)
	}
	if in.DailyContext != "" {
		fmt.Fprintf(b, "\n\nThe user's plan today (use it to answer accurately):\n%s", in.DailyContext)
	}
}

// buildSystemPrompt renders the persona (user-editable) + per-contact context +
// memory, then pins the strict JSON output contract (fixed, not user-editable).
func buildSystemPrompt(in Input, persona string) string {
	var b strings.Builder
	b.WriteString(persona)
	writeContext(&b, in)

	b.WriteString("\n\nDecide the action yourself:")
	b.WriteString("\n- reply: you're confident and it's safe to send now.")
	b.WriteString("\n- draft: you have a reply but the owner should review/edit/send it (a decision, a commitment, or you're unsure). Put your best-guess reply in \"text\" (empty if you truly can't answer).")
	b.WriteString("\n- notify: no reply is appropriate — just flag it for the owner's awareness (scam/heads-up/FYI, or they must act themselves).")
	b.WriteString("\n- silent: nothing needs a reply or a notification.")
	b.WriteString("\n\nIf a reply is long or naturally breaks into a couple of texts, you may split it into 2–3 separate messages by putting a line containing only --- between them; they'll be sent as separate WhatsApp messages with a short pause. Keep most replies a single message — don't overuse this.")
	b.WriteString("\n\nRespond with ONLY a JSON object, no other text, in exactly this shape.")
	b.WriteString(" The \"text\" field is the message that will be sent verbatim on WhatsApp, so it must be natural plain text:")
	b.WriteString(`
{
  "action": "reply | draft | notify | silent",
  "text": "the reply (when action is reply, or a suggested draft when draft)",
  "confidence": 0.0,
  "memory_update": "a short note to remember about this person, or empty",
  "reason": "short reason when draft/notify/silent",
  "flags": ["scam", "commitment", ...]
}`)
	b.WriteString("\n- confidence is your 0..1 certainty the reply is correct and appropriate.")
	return b.String()
}

// buildUserPrompt renders the recent context window (with timing) and the new burst.
func buildUserPrompt(in Input) string {
	const tsFmt = "2006-01-02 3:04 PM"
	var b strings.Builder
	fmt.Fprintf(&b, "Right now it is %s.\n\n", time.Now().Format(tsFmt))
	if len(in.Window) > 0 {
		b.WriteString("Recent conversation (oldest first):\n")
		for _, t := range in.Window {
			who := t.Name
			if t.FromMe {
				who = "You"
			} else if who == "" {
				who = in.ContactName
			}
			if who == "" {
				who = "Them"
			}
			when := ""
			if !t.At.IsZero() {
				when = " [" + t.At.Format(tsFmt) + "]"
			}
			fmt.Fprintf(&b, "%s%s: %s\n", who, when, t.Text)
		}
		b.WriteString("\n")
	}
	b.WriteString("New message(s) to respond to (just now):\n")
	b.WriteString(in.Incoming)
	return b.String()
}

// buildProactiveSystem renders the persona + proactive instructions + context
// for a bot-initiated message. No JSON contract — it returns the message text.
func buildProactiveSystem(in Input, persona, proactive, instruction string) string {
	var b strings.Builder
	b.WriteString(persona)
	b.WriteString("\n\n")
	b.WriteString(proactive)
	if strings.TrimSpace(instruction) != "" {
		fmt.Fprintf(&b, "\n\nWhat to say/do this time: %s", strings.TrimSpace(instruction))
	}
	writeContext(&b, in)
	b.WriteString("\n\nReply with ONLY the message text to send on WhatsApp (natural plain text). ")
	b.WriteString("You may split into 2–3 messages with a line containing only --- if it feels natural; keep it short.")
	return b.String()
}

// buildProactiveUser renders the recent history + the current time so the model
// can open naturally and date-aware.
func buildProactiveUser(in Input, instruction string) string {
	const tsFmt = "2006-01-02 3:04 PM"
	var b strings.Builder
	fmt.Fprintf(&b, "Right now it is %s.\n\n", time.Now().Format(tsFmt))
	if len(in.Window) > 0 {
		b.WriteString("Conversation so far (oldest first):\n")
		for _, t := range in.Window {
			who := t.Name
			if t.FromMe {
				who = "You"
			} else if who == "" {
				who = in.ContactName
			}
			if who == "" {
				who = "Them"
			}
			when := ""
			if !t.At.IsZero() {
				when = " [" + t.At.Format(tsFmt) + "]"
			}
			fmt.Fprintf(&b, "%s%s: %s\n", who, when, t.Text)
		}
	} else {
		b.WriteString("You have never talked before.")
	}
	if strings.TrimSpace(instruction) != "" {
		fmt.Fprintf(&b, "\nDo this now: %s", strings.TrimSpace(instruction))
	} else {
		b.WriteString("\nStart (or pick up) the conversation naturally.")
	}
	return b.String()
}
