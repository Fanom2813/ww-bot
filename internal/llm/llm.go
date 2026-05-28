// Package llm is the free-first language-model layer. A Provider abstracts
// "given a prompt, return text". Two implementations exist: an OpenAI-compatible
// HTTP client (OpenRouter / Groq / Ollama / any compatible endpoint) and a
// CLI-agent client that shells out to an installed coding agent (Claude Code,
// Codex, Gemini) using the user's own subscription. A Registry tries providers
// in the user's preferred order and falls back on failure.
//
// The brain asks the model for a strict JSON decision; this layer only returns
// raw text, keeping it model-agnostic.
package llm

import (
	"context"
	"errors"
	"time"

	"wwbot/internal/dbg"
)

// Role identifies who authored a message.
type Role string

const (
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
)

// Message is one turn in a conversation.
type Message struct {
	Role    Role
	Content string
}

// Request is a completion request.
type Request struct {
	System      string    // optional system prompt
	Messages    []Message // conversation turns
	Temperature float64   // 0 = deterministic-ish
	MaxTokens   int       // 0 = provider default
	// JSON asks the provider to constrain output to a JSON object (OpenAI
	// json_object mode). Providers that don't support it ignore the hint.
	JSON bool
}

// Provider is a single LLM backend.
type Provider interface {
	// Name is a stable identifier (e.g. "groq", "ollama", "claude-code").
	Name() string
	// Available reports whether this provider can currently be used (configured
	// and, for CLIs, present on PATH). It must be cheap and must not error.
	Available(ctx context.Context) bool
	// Complete returns the model's text response.
	Complete(ctx context.Context, req Request) (string, error)
}

// ErrNoProvider is returned when no configured provider is available.
var ErrNoProvider = errors.New("llm: no available provider")

// truncate shortens s to at most n characters (used in error messages).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Attempt records one provider's outcome during a Complete call (for telemetry
// and richer error messages when the chain falls back).
type Attempt struct {
	Provider string
	Err      error
}

// FallbackHook is invoked after a Complete call that succeeded on a non-primary
// provider. `used` is the provider that answered; `attempts` lists the failed
// providers (in order) that came before it.
type FallbackHook func(used string, attempts []Attempt)

// Registry holds providers in preference order (highest priority first).
type Registry struct {
	providers         []Provider
	OnFallback        FallbackHook  // optional: called when a non-primary answered
	PerAttemptTimeout time.Duration // optional: per-provider deadline (default 60s)
}

// NewRegistry builds a registry from providers in preference order.
func NewRegistry(providers ...Provider) *Registry {
	return &Registry{providers: providers}
}

// Providers returns the configured providers in order.
func (r *Registry) Providers() []Provider { return r.providers }

// Available returns the subset of providers currently usable, in order.
func (r *Registry) Available(ctx context.Context) []Provider {
	var out []Provider
	for _, p := range r.providers {
		if p.Available(ctx) {
			out = append(out, p)
		}
	}
	return out
}

// Complete tries each available provider in order, returning the first success
// along with the provider name that answered. Each provider gets its own
// PerAttemptTimeout so one stuck backend can't block the chain. On every
// failure the attempt is logged (with the provider's name) and recorded; on
// success after one or more failures, OnFallback fires so the UI can surface
// it. If all available providers fail, the returned error wraps each attempt
// (via errors.Join). If none are available at all, returns ErrNoProvider.
func (r *Registry) Complete(ctx context.Context, req Request) (text, provider string, err error) {
	timeout := r.PerAttemptTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	var attempts []Attempt
	primary := ""

	for _, p := range r.providers {
		if !p.Available(ctx) {
			continue
		}
		if primary == "" {
			primary = p.Name()
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		out, perr := p.Complete(attemptCtx, req)
		cancel()

		if perr == nil {
			if p.Name() != primary && r.OnFallback != nil {
				r.OnFallback(p.Name(), attempts)
			}
			return out, p.Name(), nil
		}
		dbg.Printf("[llm] provider %q failed: %v", p.Name(), perr)
		attempts = append(attempts, Attempt{Provider: p.Name(), Err: perr})
	}

	if len(attempts) == 0 {
		return "", "", ErrNoProvider
	}
	// Wrap every attempt so the caller can see what each provider returned.
	errs := make([]error, len(attempts))
	for i, a := range attempts {
		errs[i] = &attemptError{Provider: a.Provider, Err: a.Err}
	}
	return "", "", errors.Join(errs...)
}

// attemptError tags an error with the provider name that produced it.
type attemptError struct {
	Provider string
	Err      error
}

func (e *attemptError) Error() string { return e.Provider + ": " + e.Err.Error() }
func (e *attemptError) Unwrap() error { return e.Err }
