package core

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/zalando/go-keyring"

	"wwbot/internal/brain"
	"wwbot/internal/llm"
	"wwbot/internal/safety"
	"wwbot/internal/stt"
	"wwbot/internal/store"
)

// settingsKey is where the JSON Settings blob lives in the store.
const settingsKey = "config"

// ProviderSetting describes one LLM backend in the user-ordered list.
type ProviderSetting struct {
	Kind        string `json:"kind"` // "openai" | "anthropic" | "cli"
	Name        string `json:"name"`
	BaseURL     string `json:"baseUrl,omitempty"`
	APIKey      string `json:"apiKey,omitempty"`
	Model       string `json:"model,omitempty"`
	RequiresKey bool   `json:"requiresKey,omitempty"`
	Bin         string `json:"bin,omitempty"` // custom CLI command, e.g. "claude"
	// Args are the flags for a custom CLI, e.g. ["--model","sonnet","-p"].
	Args []string `json:"args,omitempty"`
	// AppendPrompt: false → pipe the prompt to the CLI's stdin (e.g. claude -p);
	// true → append it as the final argument (e.g. gemini -p <prompt>).
	AppendPrompt bool `json:"appendPrompt,omitempty"`
	Enabled      bool `json:"enabled"`
	// HasKey is set on read to tell the UI a key exists in the keychain (the key
	// itself is never sent to the frontend or persisted in the settings DB).
	HasKey bool `json:"hasKey,omitempty"`
}

// STTSetting configures voice transcription.
type STTSetting struct {
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
	Model    string `json:"model"`
	Language string `json:"language"`
	Enabled  bool   `json:"enabled"`
}

// SafetySetting configures the outbound gate (durations in seconds, hours 0..24).
type SafetySetting struct {
	MinDelaySec           int `json:"minDelaySec"`
	MaxDelaySec           int `json:"maxDelaySec"`
	PerMinute             int `json:"perMinute"`
	PerDay                int `json:"perDay"`
	PerContactCooldownSec int `json:"perContactCooldownSec"`
	QuietStart            int `json:"quietStart"`
	QuietEnd              int `json:"quietEnd"`
	// QuietHoursOff disables quiet hours entirely. Inverted so existing settings
	// (field absent → false) keep quiet hours enabled.
	QuietHoursOff bool `json:"quietHoursOff,omitempty"`
}

// ScheduledTask is a recurring proactive job (a "cron"): every day at Hour:Min
// the bot reaches out to each contact in Contacts following Prompt. Not limited
// to greetings — Prompt is any instruction (morning dua, evening check-in, …).
type ScheduledTask struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Hour     int      `json:"hour"`
	Min      int      `json:"min"`
	Contacts []string `json:"contacts"`
	Prompt   string   `json:"prompt"`
	Enabled  bool     `json:"enabled"`
}

// Settings is the full app configuration, stored as one JSON blob.
type Settings struct {
	Providers     []ProviderSetting `json:"providers"`
	STT           STTSetting        `json:"stt"`
	Safety        SafetySetting     `json:"safety"`
	Schedules     []ScheduledTask   `json:"schedules"`
	MinConfidence float64           `json:"minConfidence"`
	MinSTTConf    float64           `json:"minSttConfidence"`
	// GuestMode, when true, lets the bot reply to 1-1 messages from people who
	// are NOT in your contacts (never groups). When false, an unknown sender only
	// triggers the "save this contact?" prompt and gets no reply.
	GuestMode bool `json:"guestMode"`
	// GuestTier is how the bot handles those non-whitelisted senders while
	// GuestMode is on: "auto" (reply), "draft" (queue for approval), or "notify".
	GuestTier string `json:"guestTier"`
	// SystemPrompt is the user-editable persona/system prompt. Empty means use
	// the built-in default (brain.DefaultPersona).
	SystemPrompt string `json:"systemPrompt"`
	// ProactivePrompt is the user-editable prompt for bot-initiated messages.
	// Empty means use the built-in default (brain.DefaultProactivePrompt).
	ProactivePrompt string `json:"proactivePrompt"`
}

// DefaultSettings returns sensible defaults. Only the auto-discovered CLI agents
// are listed (disabled until the user turns one on). Hosted OpenAI-compatible and
// Anthropic providers are added by the user via the Settings dialog.
func DefaultSettings() Settings {
	return Settings{
		Providers: []ProviderSetting{
			{Kind: "cli", Name: "claude-code", Enabled: false},
			{Kind: "cli", Name: "codex", Enabled: false},
			{Kind: "cli", Name: "gemini-cli", Enabled: false},
		},
		STT: STTSetting{
			BaseURL: "https://api.groq.com/openai/v1",
			Model:   "whisper-large-v3-turbo",
			Enabled: false,
		},
		Safety: SafetySetting{
			MinDelaySec: 3, MaxDelaySec: 12, PerMinute: 6, PerDay: 200,
			PerContactCooldownSec: 15, QuietStart: 23, QuietEnd: 7,
		},
		MinConfidence: 0.7,
		MinSTTConf:    0.5,
		GuestMode:     false,
		GuestTier:     string(store.TierAuto),
	}
}

// loadSettings reads the Settings blob, falling back to defaults (and persisting
// them) when none exist.
func loadSettings(ctx context.Context, db *store.DB) (Settings, error) {
	raw, err := db.GetSetting(ctx, settingsKey)
	if err != nil {
		return Settings{}, err
	}
	if strings.TrimSpace(raw) == "" {
		s := DefaultSettings()
		_ = saveSettings(ctx, db, s)
		return s, nil
	}
	var s Settings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return DefaultSettings(), nil
	}
	return s, nil
}

func saveSettings(ctx context.Context, db *store.DB, s Settings) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return db.SetSetting(ctx, settingsKey, string(b))
}

// keyringService namespaces this app's secrets in the OS keychain.
const keyringService = "ww-bot"

func storeKey(name, key string) {
	if err := keyring.Set(keyringService, name, key); err != nil {
		log.Printf("[core] keychain SET failed for %q: %v (key not saved)", name, err)
	}
}
func loadKey(name string) string {
	v, err := keyring.Get(keyringService, name)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		log.Printf("[core] keychain GET failed for %q: %v", name, err)
	}
	return v
}
func deleteKey(name string) { _ = keyring.Delete(keyringService, name) }
func hasKey(name string) bool { return loadKey(name) != "" }

// hydrateKeys returns a copy of the providers with API keys pulled from the OS
// keychain. Keys are only ever materialized here (to build the registry) — they
// are never stored in the settings DB or returned to the UI.
func hydrateKeys(ps []ProviderSetting) []ProviderSetting {
	out := make([]ProviderSetting, len(ps))
	for i, p := range ps {
		if p.RequiresKey && p.APIKey == "" {
			p.APIKey = loadKey(p.Name)
		}
		out[i] = p
	}
	return out
}

// buildRegistry constructs the LLM registry from the ordered provider list.
func buildRegistry(ps []ProviderSetting) *llm.Registry {
	var provs []llm.Provider
	for _, p := range ps {
		if !p.Enabled {
			continue
		}
		switch p.Kind {
		case "openai":
			log.Printf("[core] provider %q (openai) enabled: model=%q hasKey=%v requiresKey=%v", p.Name, p.Model, p.APIKey != "", p.RequiresKey)
			provs = append(provs, llm.NewOpenAICompatible(llm.OpenAIConfig{
				Name: p.Name, BaseURL: p.BaseURL, APIKey: p.APIKey, Model: p.Model, RequiresKey: p.RequiresKey,
			}))
		case "anthropic":
			log.Printf("[core] provider %q (anthropic) enabled: model=%q hasKey=%v", p.Name, p.Model, p.APIKey != "")
			provs = append(provs, llm.NewAnthropic(llm.AnthropicConfig{
				Name: p.Name, APIKey: p.APIKey, Model: p.Model, BaseURL: p.BaseURL,
			}))
		case "cli":
			// A custom CLI (any command that accepts a prompt) is fully described
			// by its binary + args; the prompt goes to stdin unless AppendPrompt.
			if p.Bin != "" {
				provs = append(provs, llm.NewCLIAgent(p.Name, p.Bin, p.Args, !p.AppendPrompt))
				break
			}
			switch strings.ToLower(p.Name) {
			case "claude-code", "claude":
				provs = append(provs, llm.ClaudeCLI())
			case "codex":
				provs = append(provs, llm.CodexCLI())
			case "gemini-cli", "gemini":
				provs = append(provs, llm.GeminiCLI())
			}
		}
	}
	return llm.NewRegistry(provs...)
}

// buildSTT returns a transcriber, or nil when voice is disabled/unconfigured.
func buildSTT(s STTSetting) stt.Transcriber {
	if !s.Enabled || s.APIKey == "" || s.BaseURL == "" || s.Model == "" {
		return nil
	}
	return stt.NewOpenAITranscriber(stt.Config{
		Name: "voice", BaseURL: s.BaseURL, APIKey: s.APIKey, Model: s.Model, Language: s.Language,
	})
}

// buildSafetyConfig maps the user's safety settings to the gate config.
func buildSafetyConfig(s SafetySetting) safety.Config {
	qStart, qEnd := s.QuietStart, s.QuietEnd
	if s.QuietHoursOff {
		qStart, qEnd = 0, 0 // start==end disables quiet hours in the gate
	}
	return safety.Config{
		MinDelay:           time.Duration(s.MinDelaySec) * time.Second,
		MaxDelay:           time.Duration(s.MaxDelaySec) * time.Second,
		PerMinute:          s.PerMinute,
		PerDay:             s.PerDay,
		PerContactCooldown: time.Duration(s.PerContactCooldownSec) * time.Second,
		QuietStart:         qStart,
		QuietEnd:           qEnd,
	}
}

func brainConfig(s Settings) brain.Config {
	return brain.Config{MinConfidence: s.MinConfidence, Persona: s.SystemPrompt, Proactive: s.ProactivePrompt}
}
