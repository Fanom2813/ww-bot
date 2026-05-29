package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// OpenAIConfig configures an OpenAI-compatible provider.
type OpenAIConfig struct {
	Name        string // stable id, e.g. "groq", "openrouter", "ollama"
	BaseURL     string // e.g. "https://api.groq.com/openai/v1"
	APIKey      string // bearer token; empty for local endpoints
	Model       string // e.g. "llama-3.3-70b-versatile"
	RequiresKey bool   // hosted providers need a key; local (Ollama) do not
}

// OpenAICompatible talks to any endpoint implementing the OpenAI
// /chat/completions API, via the official openai-go SDK with a custom base URL.
type OpenAICompatible struct {
	cfg    OpenAIConfig
	client openai.Client
}

// NewOpenAICompatible constructs a provider from cfg.
func NewOpenAICompatible(cfg OpenAIConfig) *OpenAICompatible {
	opts := []option.RequestOption{option.WithRequestTimeout(120 * time.Second)}
	if cfg.BaseURL != "" {
		// The SDK appends "chat/completions" to the base, so keep the trailing slash.
		opts = append(opts, option.WithBaseURL(strings.TrimRight(cfg.BaseURL, "/")+"/"))
	}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	return &OpenAICompatible{cfg: cfg, client: openai.NewClient(opts...)}
}

func (o *OpenAICompatible) Name() string { return o.cfg.Name }

// Available reports whether the provider is configured enough to call.
func (o *OpenAICompatible) Available(_ context.Context) bool {
	if o.cfg.BaseURL == "" || o.cfg.Model == "" {
		return false
	}
	if o.cfg.RequiresKey && o.cfg.APIKey == "" {
		return false
	}
	return true
}

// Complete sends the request and returns the assistant's text.
func (o *OpenAICompatible) Complete(ctx context.Context, req Request) (string, error) {
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openai.SystemMessage(req.System))
	}
	for _, m := range req.Messages {
		if m.Role == Assistant {
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		} else {
			msgs = append(msgs, openai.UserMessage(m.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(o.cfg.Model),
		Messages: msgs,
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}
	if req.MaxTokens > 0 {
		// max_completion_tokens replaces max_tokens; the older field is
		// deprecated and is silently *ignored* by o-series reasoning models,
		// which is why we were seeing empty replies + finish_reason=length on
		// OpenRouter when the route picked a reasoning model.
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.JSON {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: &shared.ResponseFormatJSONObjectParam{},
		}
	}

	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("llm %s: %w", o.cfg.Name, err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("llm %s: no choices in response", o.cfg.Name)
	}
	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	finish := resp.Choices[0].FinishReason

	// Empty content with finish_reason=length almost always means the model
	// either doesn't understand JSON mode (and burned its budget producing
	// nothing visible) or hit the cap mid-thought. If we asked for JSON,
	// retry once without it — the brain's parser tolerates surrounding
	// prose and code fences, so a free-form response still parses.
	if content == "" && req.JSON && (finish == "length" || finish == "") {
		params.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{}
		retry, rerr := o.client.Chat.Completions.New(ctx, params)
		if rerr == nil && len(retry.Choices) > 0 {
			content = strings.TrimSpace(retry.Choices[0].Message.Content)
			finish = retry.Choices[0].FinishReason
		}
	}

	if content == "" {
		return "", fmt.Errorf("llm %s: empty content (finish_reason=%q) — model returned nothing (may not support JSON mode, or hit the token cap)",
			o.cfg.Name, finish)
	}
	return content, nil
}
