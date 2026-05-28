package brain

import (
	"math"
	"regexp"
	"strings"
)

// Redaction strips credential/secret-looking tokens from text before it is ever
// sent to an LLM. It's defense-in-depth alongside IsSensitive: IsSensitive
// blocks a whole message that's clearly a code/credential delivery, while Redact
// scrubs stray secrets from messages (and context) that are otherwise fine to
// process.
//
// It combines three techniques (the same ones secret scanners like gitleaks use):
//  1. known token shapes (API keys, JWTs, bearer tokens),
//  2. labeled secrets and OTP/code phrasing,
//  3. Shannon-entropy analysis to catch any high-entropy random token, plus
//     Luhn-validated card numbers — so it generalizes beyond known formats.

const redactMark = "[redacted]"

var (
	// Well-known secret token shapes.
	redactTokenPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9_-]{16,}`),                                  // OpenAI-style keys
		regexp.MustCompile(`(?i)\b(?:sk|pk|rk)_(?:live|test)_[A-Za-z0-9]{16,}`),            // Stripe keys
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),                                         // AWS access key id
		regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{16,}`),                                     // Google API key
		regexp.MustCompile(`(?i)\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`),               // GitHub tokens
		regexp.MustCompile(`(?i)\bxox[baprs]-[A-Za-z0-9-]{10,}`),                           // Slack tokens
		regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`), // JWT
		regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._-]{16,}`),                           // bearer tokens
		regexp.MustCompile(`(?i)-----BEGIN[^-]+PRIVATE KEY-----`),                          // PEM private key header
	}
	// "password is X", "otp: 123456", "api key = X" → keep the label, drop value.
	redactLabelValue = regexp.MustCompile(`(?i)\b(password|passcode|passphrase|pwd|pin|otp|cvv|cvc|secret|token|api[ _-]?key|seed phrase|private key)\b\s*(?:is\s+|are\s+|:\s*|=\s*|->\s*|=>\s*)([^\s,;]+)`)
	// A digit run near code/verification language (OTP/2FA codes).
	redactCodeDigits = regexp.MustCompile(`(?i)\b(code|otp|verification|verify|passcode|pin|2fa|password)\b([^0-9\n]{0,20}?)(\d{3,12})\b`)
	// Long bare digit sequences (account numbers, IBANs).
	redactLongDigits = regexp.MustCompile(`\b\d{12,}\b`)
	// Card-shaped sequences (13–19 digits, optional spaces/dashes) — Luhn-checked.
	redactCardLike = regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)
	// Candidate high-entropy tokens (keys/secrets that match no known pattern).
	redactEntropyCandidate = regexp.MustCompile(`[A-Za-z0-9+/=_-]{20,}`)
)

// Redact replaces credential-looking substrings with a placeholder. The rest of
// the text (and its meaning) is preserved so the bot can still respond.
func Redact(text string) string {
	if text == "" {
		return text
	}
	for _, re := range redactTokenPatterns {
		text = re.ReplaceAllString(text, redactMark)
	}
	text = redactLabelValue.ReplaceAllString(text, "${1}: "+redactMark)
	text = redactCodeDigits.ReplaceAllString(text, "${1}${2}"+redactMark)
	text = redactCardLike.ReplaceAllStringFunc(text, func(m string) string {
		if luhnValid(m) {
			return redactMark
		}
		return m
	})
	text = redactLongDigits.ReplaceAllString(text, redactMark)
	// Entropy pass last: catch any remaining random-looking secret.
	text = redactEntropyCandidate.ReplaceAllStringFunc(text, func(tok string) string {
		if looksSecret(tok) {
			return redactMark
		}
		return tok
	})
	return text
}

// looksSecret reports whether a token is likely a machine-generated secret: long,
// high-entropy, and mixing character classes (so ordinary long words pass).
func looksSecret(tok string) bool {
	if len(tok) < 20 {
		return false
	}
	var hasLower, hasUpper, hasDigit, hasSym bool
	for _, r := range tok {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSym = true
		}
	}
	classes := 0
	for _, b := range []bool{hasLower, hasUpper, hasDigit, hasSym} {
		if b {
			classes++
		}
	}
	// A plain long word (single class) is not a secret; require ≥2 classes.
	if classes < 2 {
		return false
	}
	return shannonEntropy(tok) >= 3.5
}

// shannonEntropy returns the per-character Shannon entropy (bits) of s.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	freq := make(map[rune]float64, len(s))
	for _, r := range s {
		freq[r]++
	}
	n := float64(len([]rune(s)))
	var e float64
	for _, c := range freq {
		p := c / n
		e -= p * math.Log2(p)
	}
	return e
}

// luhnValid reports whether the digits in s (ignoring spaces/dashes) pass the
// Luhn checksum used by payment card numbers.
func luhnValid(s string) bool {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	var sum int
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}
