package brain

import (
	"fmt"
	"strings"
)

// buildSystemPrompt renders the instructions + persona + memory for the model,
// and pins the strict JSON output contract.
func buildSystemPrompt(in Input) string {
	var b strings.Builder
	b.WriteString("You are replying to a WhatsApp message ON BEHALF OF the user (the account owner). ")
	b.WriteString("Write as if you are the user — natural, brief, and in their voice. ")

	if in.ContactName != "" {
		fmt.Fprintf(&b, "\n\nYou are talking to: %s.", in.ContactName)
	}
	if in.Language != "" {
		fmt.Fprintf(&b, " Reply in this language: %s.", in.Language)
	} else {
		b.WriteString(" Reply in the same language the contact is using.")
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

	b.WriteString("\n\nRules:")
	b.WriteString("\n- If you don't have enough information to answer accurately, do NOT guess.")
	b.WriteString("\n- Never make real-world commitments (meetings, money, yes/no to invitations) — those need the user.")
	b.WriteString("\n- If the message looks like a scam, phishing, or impersonation, flag it.")
	b.WriteString("\n- It is better to stay silent or escalate than to send wrong information.")

	b.WriteString("\n\nRespond with ONLY a JSON object, no other text, in exactly this shape:")
	b.WriteString(`
{
  "action": "reply | escalate | silent",
  "text": "the reply to send (when action is reply or a suggested draft when escalate)",
  "confidence": 0.0,
  "memory_update": "a short note to remember about this person, or empty",
  "reason": "short reason when escalating or staying silent",
  "flags": ["scam", "commitment", ...]
}`)
	b.WriteString("\n- action=reply: you are confident and it's safe to send.")
	b.WriteString("\n- action=escalate: you have a draft but the user should review (unsure / sensitive / a decision).")
	b.WriteString("\n- action=silent: nothing needs a reply.")
	b.WriteString("\n- confidence is your 0..1 certainty the reply is correct and appropriate.")
	return b.String()
}

// buildUserPrompt renders the recent context window and the new burst.
func buildUserPrompt(in Input) string {
	var b strings.Builder
	if len(in.Window) > 0 {
		b.WriteString("Recent conversation:\n")
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
			fmt.Fprintf(&b, "%s: %s\n", who, t.Text)
		}
		b.WriteString("\n")
	}
	b.WriteString("New message(s) to respond to:\n")
	b.WriteString(in.Incoming)
	return b.String()
}
