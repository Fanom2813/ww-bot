package brain

import "strings"

import "testing"

func TestRedact(t *testing.T) {
	cases := []struct {
		name string
		in   string
		// substrings that must NOT appear in the output (the secret).
		gone []string
		// substrings that must still appear (context preserved).
		keep []string
	}{
		{"otp", "Your verification code is 482913", []string{"482913"}, []string{"verification code"}},
		{"password", "the password is hunter2 ok?", []string{"hunter2"}, []string{"password", "ok?"}},
		{"openai key", "use sk-abcdEFGH1234567890zzzz now", []string{"sk-abcdEFGH1234567890zzzz"}, []string{"use", "now"}},
		{"card", "card 4111111111111111 charged", []string{"4111111111111111"}, []string{"card", "charged"}},
		{"spaced card", "pay to 4111 1111 1111 1111 now", []string{"4111 1111 1111 1111"}, []string{"pay to", "now"}},
		{"entropy key", "here: Zx9Qা skip Kp3mR8vT2wLZ0aQ7nB4xY1cD now", []string{"Kp3mR8vT2wLZ0aQ7nB4xY1cD"}, []string{"here", "now"}},
		{"long word kept", "this is unbelievablyfantastictremendous news", nil, []string{"unbelievablyfantastictremendous"}},
		{"jwt", "token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c now", []string{"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"}, []string{"token", "now"}},
		{"clean", "hey can we meet at 5 today?", nil, []string{"meet at 5 today"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Redact(c.in)
			for _, g := range c.gone {
				if strings.Contains(got, g) {
					t.Errorf("secret %q leaked through: %q", g, got)
				}
			}
			for _, k := range c.keep {
				if !strings.Contains(got, k) {
					t.Errorf("context %q lost: %q", k, got)
				}
			}
		})
	}
}
