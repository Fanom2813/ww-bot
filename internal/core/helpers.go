package core

import (
	"strings"

	"wwbot/internal/brain"
	"wwbot/internal/inbox"
)

// summarizeBurst joins a coalesced burst into one incoming string and derives a
// sender display name.
func summarizeBurst(b inbox.Batch, contactName string) (incoming, senderName string) {
	var lines []string
	for _, m := range b.Messages {
		if m.Text != "" {
			lines = append(lines, m.Text)
		}
		if senderName == "" {
			senderName = m.PushName
		}
	}
	if senderName == "" {
		senderName = contactName
	}
	if senderName == "" {
		senderName = "Them"
	}
	return strings.Join(lines, "\n"), senderName
}

// isGroup reports whether the batch is from a group chat.
func isGroup(b inbox.Batch) bool {
	for _, m := range b.Messages {
		if m.IsGroup {
			return true
		}
	}
	return false
}

// toTurns maps the ephemeral context window to brain turns.
func toTurns(window []inbox.Msg) []brain.Turn {
	turns := make([]brain.Turn, 0, len(window))
	for _, m := range window {
		turns = append(turns, brain.Turn{FromMe: m.IsFromMe, Name: m.PushName, Text: m.Text})
	}
	return turns
}

// mergeSummary appends a memory update to the existing summary, capping length.
func mergeSummary(existing, update string) string {
	update = strings.TrimSpace(update)
	if update == "" {
		return existing
	}
	merged := strings.TrimSpace(existing + "\n" + update)
	const maxLen = 2000
	if len(merged) > maxLen {
		merged = merged[len(merged)-maxLen:]
	}
	return merged
}

// preview shortens text to a single-line snippet for logs/notifications.
func preview(text string) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\n", " ")
	const max = 80
	if len(text) > max {
		return text[:max] + "…"
	}
	return text
}
