package brain

import (
	"fmt"
	"strings"
	"time"
)

// buildSystemPrompt renders the persona (user-editable) + per-contact context +
// memory, then pins the strict JSON output contract (fixed, not user-editable).
func buildSystemPrompt(in Input, persona string) string {
	var b strings.Builder
	b.WriteString(persona)

	if in.ContactName != "" {
		fmt.Fprintf(&b, "\n\nYou are talking to: %s.", in.ContactName)
	}
	if in.Language != "" {
		fmt.Fprintf(&b, " Reply in this language: %s.", in.Language)
	}
	if in.Style != "" {
		fmt.Fprintf(&b, "\nReply style for this contact: %s", in.Style)
	}
	if in.Summary != "" {
		fmt.Fprintf(&b, "\n\nWhat you remember about this person:\n%s", in.Summary)
	}
	if in.DailyContext != "" {
		fmt.Fprintf(&b, "\n\nThe user's plan today (use it to answer accurately):\n%s", in.DailyContext)
	}

	b.WriteString("\n\nDecide the action yourself:")
	b.WriteString("\n- reply: you're confident and it's safe to send now.")
	b.WriteString("\n- draft: you have a reply but the owner should review/edit/send it (a decision, a commitment, or you're unsure). Put your best-guess reply in \"text\" (empty if you truly can't answer).")
	b.WriteString("\n- notify: no reply is appropriate — just flag it for the owner's awareness (scam/heads-up/FYI, or they must act themselves).")
	b.WriteString("\n- silent: nothing needs a reply or a notification.")
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
