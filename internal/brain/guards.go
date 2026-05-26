package brain

import (
	"regexp"
	"strings"
)

// These guardrails are deliberately conservative: when in doubt they cause the
// bot to escalate/notify rather than auto-send. They protect against leaking
// codes, and against the bot making real-world commitments on the user's behalf.

var (
	// sensitiveKeywords mark messages that must never be sent to an LLM or
	// auto-answered (one-time codes, credentials, banking).
	sensitiveKeywords = []string{
		"otp", "one-time", "one time password", "verification code", "verify code",
		"2fa", "two-factor", "two factor", "auth code", "security code", "passcode",
		"password", "pin code", "cvv", "card number", "account number", "iban",
		"seed phrase", "private key", "login code",
	}
	// reCodeWord: a short digit run near code/verification language.
	reCodeWord = regexp.MustCompile(`(?i)\b(code|otp|verification|verify|passcode)\b[^0-9]{0,20}\b\d{4,8}\b`)
	reCodeWord2 = regexp.MustCompile(`(?i)\b\d{4,8}\b[^0-9]{0,20}\b(code|otp|verification|verify|passcode)\b`)
	// reLongDigits: long digit sequences (cards/accounts).
	reLongDigits = regexp.MustCompile(`\b\d{12,}\b`)

	// commitmentKeywords suggest a real-world promise the user must confirm.
	commitmentKeywords = []string{
		"i'll be there", "i will be there", "see you at", "i'll come", "i will come",
		"i promise", "i'll pay", "i will pay", "i'll send the money", "transfer the money",
		"i'll transfer", "deposit", "i agree to", "it's confirmed", "consider it done",
		"i'll meet you", "let's meet at", "book it", "i'll attend", "count me in",
		"yes i'll do it", "deal", "i accept",
	}
)

// IsSensitive reports whether text contains one-time codes, credentials, or
// banking details that must never be sent to an LLM or auto-answered.
func IsSensitive(text string) bool {
	lower := strings.ToLower(text)
	for _, k := range sensitiveKeywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	if reCodeWord.MatchString(text) || reCodeWord2.MatchString(text) {
		return true
	}
	if reLongDigits.MatchString(text) {
		return true
	}
	return false
}

// IsCommitment reports whether a proposed reply makes a real-world commitment
// (meeting, money, yes/no to an invitation) that should be confirmed by the
// user. It considers both model-supplied flags and the reply text.
func IsCommitment(reply string, flags []string) bool {
	for _, f := range flags {
		switch strings.ToLower(strings.TrimSpace(f)) {
		case "commitment", "commit", "promise", "appointment", "money", "payment":
			return true
		}
	}
	lower := strings.ToLower(reply)
	for _, k := range commitmentKeywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

// hasFlag reports whether flags contains name (case-insensitive).
func hasFlag(flags []string, name string) bool {
	for _, f := range flags {
		if strings.EqualFold(strings.TrimSpace(f), name) {
			return true
		}
	}
	return false
}
