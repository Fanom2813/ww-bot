// Package commands parses control input the user sends to themselves (the
// WhatsApp "Message Yourself" chat). Slash commands change the bot's behavior;
// anything else is treated as a daily-context note ("today I'm doing X"). This
// package is a pure parser — execution (touching the store / gate) lives in the
// wiring layer.
package commands

import "strings"

// Kind enumerates the recognized commands.
type Kind string

const (
	Pause   Kind = "pause"   // /pause — stop all outbound
	Resume  Kind = "resume"  // /resume — resume outbound
	Status  Kind = "status"  // /status — report bot status
	Help    Kind = "help"    // /help — list commands
	Tier    Kind = "tier"    // /tier <target> <auto|draft|notify>
	Today   Kind = "today"   // /today <text> — set today's context
	Note    Kind = "note"    // non-slash text -> daily-context note
	Unknown Kind = "unknown" // unrecognized slash command
)

// Command is a parsed control input.
type Command struct {
	Kind   Kind
	Raw    string
	Target string // for Tier: the contact (name or JID)
	Tier   string // for Tier: auto|draft|notify
	Text   string // for Today/Note
}

// Parse interprets a self-chat message.
func Parse(input string) Command {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Command{Kind: Note, Raw: raw}
	}
	if !strings.HasPrefix(raw, "/") {
		// Not a command — treat as a note for today's context.
		return Command{Kind: Note, Raw: raw, Text: raw}
	}

	fields := strings.Fields(raw)
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	rest := strings.TrimSpace(strings.TrimPrefix(raw, fields[0]))

	switch cmd {
	case "pause":
		return Command{Kind: Pause, Raw: raw}
	case "resume":
		return Command{Kind: Resume, Raw: raw}
	case "status":
		return Command{Kind: Status, Raw: raw}
	case "help":
		return Command{Kind: Help, Raw: raw}
	case "today":
		return Command{Kind: Today, Raw: raw, Text: rest}
	case "tier":
		// /tier <target...> <tier>
		if len(fields) >= 3 {
			tier := strings.ToLower(fields[len(fields)-1])
			if isValidTier(tier) {
				target := strings.Join(fields[1:len(fields)-1], " ")
				return Command{Kind: Tier, Raw: raw, Target: target, Tier: tier}
			}
		}
		return Command{Kind: Unknown, Raw: raw}
	default:
		return Command{Kind: Unknown, Raw: raw}
	}
}

func isValidTier(t string) bool {
	switch t {
	case "auto", "draft", "notify":
		return true
	}
	return false
}
