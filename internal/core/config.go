package core

import (
	"context"
	"encoding/json"
	"strings"
	"time"

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
	Kind        string `json:"kind"` // "openai" | "cli"
	Name        string `json:"name"`
	BaseURL     string `json:"baseUrl,omitempty"`
	APIKey      string `json:"apiKey,omitempty"`
	Model       string `json:"model,omitempty"`
	RequiresKey bool   `json:"requiresKey,omitempty"`
	Bin         string `json:"bin,omitempty"` // for custom cli kind
	Enabled     bool   `json:"enabled"`
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
}

// GreetingSetting is a scheduled proactive message.
type GreetingSetting struct {
	Hour    int    `json:"hour"`
	Min     int    `json:"min"`
	ToJID   string `json:"toJid"`
	Text    string `json:"text"`
	Enabled bool   `json:"enabled"`
}

// Settings is the full app configuration, stored as one JSON blob.
type Settings struct {
	Providers     []ProviderSetting `json:"providers"`
	STT           STTSetting        `json:"stt"`
	Safety        SafetySetting     `json:"safety"`
	Greetings     []GreetingSetting `json:"greetings"`
	MinConfidence float64           `json:"minConfidence"`
	MinSTTConf    float64           `json:"minSttConfidence"`
	// GuestMode, when true, lets the bot reply to 1-1 messages from people who
	// are NOT in your contacts (never groups). When false, an unknown sender only
	// triggers the "save this contact?" prompt and gets no reply.
	GuestMode bool `json:"guestMode"`
	// GuestTier is how the bot handles those non-whitelisted senders while
	// GuestMode is on: "auto" (reply), "draft" (queue for approval), or "notify".
	GuestTier string `json:"guestTier"`
}

// DefaultSettings returns sensible free-first defaults. Local Ollama is enabled;
// CLI agents and hosted providers ship disabled until the user turns them on
// (CLI agents only actually run if present on PATH).
func DefaultSettings() Settings {
	return Settings{
		Providers: []ProviderSetting{
			{Kind: "cli", Name: "claude-code", Enabled: false},
			{Kind: "cli", Name: "codex", Enabled: false},
			{Kind: "cli", Name: "gemini-cli", Enabled: false},
			{Kind: "openai", Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "llama3.2", RequiresKey: false, Enabled: true},
			{Kind: "openai", Name: "openrouter", BaseURL: "https://openrouter.ai/api/v1", Model: "meta-llama/llama-3.3-70b-instruct:free", RequiresKey: true, Enabled: false},
			{Kind: "openai", Name: "groq", BaseURL: "https://api.groq.com/openai/v1", Model: "llama-3.3-70b-versatile", RequiresKey: true, Enabled: false},
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

// buildRegistry constructs the LLM registry from the ordered provider list.
func buildRegistry(ps []ProviderSetting) *llm.Registry {
	var provs []llm.Provider
	for _, p := range ps {
		if !p.Enabled {
			continue
		}
		switch p.Kind {
		case "openai":
			provs = append(provs, llm.NewOpenAICompatible(llm.OpenAIConfig{
				Name: p.Name, BaseURL: p.BaseURL, APIKey: p.APIKey, Model: p.Model, RequiresKey: p.RequiresKey,
			}))
		case "cli":
			switch strings.ToLower(p.Name) {
			case "claude-code", "claude":
				provs = append(provs, llm.ClaudeCLI())
			case "codex":
				provs = append(provs, llm.CodexCLI())
			case "gemini-cli", "gemini":
				provs = append(provs, llm.GeminiCLI())
			default:
				if p.Bin != "" {
					provs = append(provs, llm.NewCLIAgent(p.Name, p.Bin, nil, true))
				}
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
	return safety.Config{
		MinDelay:           time.Duration(s.MinDelaySec) * time.Second,
		MaxDelay:           time.Duration(s.MaxDelaySec) * time.Second,
		PerMinute:          s.PerMinute,
		PerDay:             s.PerDay,
		PerContactCooldown: time.Duration(s.PerContactCooldownSec) * time.Second,
		QuietStart:         s.QuietStart,
		QuietEnd:           s.QuietEnd,
	}
}

func brainConfig(s Settings) brain.Config {
	return brain.Config{MinConfidence: s.MinConfidence}
}
