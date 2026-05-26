package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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
// /chat/completions API.
type OpenAICompatible struct {
	cfg    OpenAIConfig
	client *http.Client
}

// NewOpenAICompatible constructs a provider from cfg.
func NewOpenAICompatible(cfg OpenAIConfig) *OpenAICompatible {
	return &OpenAICompatible{
		cfg:    cfg,
		client: &http.Client{Timeout: 120 * time.Second},
	}
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

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends the request and returns the assistant's text.
func (o *OpenAICompatible) Complete(ctx context.Context, req Request) (string, error) {
	msgs := make([]chatMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, chatMessage{Role: string(System), Content: req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, chatMessage{Role: string(m.Role), Content: m.Content})
	}

	body, err := json.Marshal(chatRequest{
		Model:       o.cfg.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("llm %s: marshal: %w", o.cfg.Name, err)
	}

	url := strings.TrimRight(o.cfg.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm %s: request: %w", o.cfg.Name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llm %s: do: %w", o.cfg.Name, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm %s: status %d: %s", o.cfg.Name, resp.StatusCode, truncate(string(data), 300))
	}

	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return "", fmt.Errorf("llm %s: decode: %w", o.cfg.Name, err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("llm %s: %s", o.cfg.Name, cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("llm %s: empty response", o.cfg.Name)
	}
	return strings.TrimSpace(cr.Choices[0].Message.Content), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
