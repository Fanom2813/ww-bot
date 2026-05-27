package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicConfig configures a native Anthropic (Claude) API provider.
type AnthropicConfig struct {
	Name      string // stable id, e.g. "claude"
	APIKey    string // required
	Model     string // e.g. "claude-sonnet-4-5"
	MaxTokens int    // default response cap (Anthropic requires one)
	BaseURL   string // optional override
}

// Anthropic talks to the Claude Messages API via the official anthropic-sdk-go.
type Anthropic struct {
	cfg    AnthropicConfig
	client anthropic.Client
}

// NewAnthropic constructs a provider from cfg.
func NewAnthropic(cfg AnthropicConfig) *Anthropic {
	opts := []option.RequestOption{option.WithRequestTimeout(120 * time.Second)}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &Anthropic{cfg: cfg, client: anthropic.NewClient(opts...)}
}

func (a *Anthropic) Name() string { return a.cfg.Name }

// Available reports whether the provider is configured (key + model).
func (a *Anthropic) Available(_ context.Context) bool {
	return a.cfg.APIKey != "" && a.cfg.Model != ""
}

// Complete sends the request and returns Claude's text response.
func (a *Anthropic) Complete(ctx context.Context, req Request) (string, error) {
	msgs := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		block := anthropic.NewTextBlock(m.Content)
		if m.Role == Assistant {
			msgs = append(msgs, anthropic.NewAssistantMessage(block))
		} else {
			msgs = append(msgs, anthropic.NewUserMessage(block))
		}
	}

	maxTok := int64(a.cfg.MaxTokens)
	if req.MaxTokens > 0 {
		maxTok = int64(req.MaxTokens)
	}
	if maxTok <= 0 {
		maxTok = 1024
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(a.cfg.Model),
		MaxTokens: maxTok,
		Messages:  msgs,
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("llm %s: %w", a.cfg.Name, err)
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		sb.WriteString(block.Text)
	}
	return strings.TrimSpace(sb.String()), nil
}
