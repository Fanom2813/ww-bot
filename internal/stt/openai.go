package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Config configures an OpenAI-compatible transcriber.
type Config struct {
	Name              string  // e.g. "groq"
	BaseURL           string  // e.g. "https://api.groq.com/openai/v1"
	APIKey            string  // bearer token
	Model             string  // e.g. "whisper-large-v3-turbo"
	Language          string  // optional ISO code; empty = auto-detect
	NoSpeechThreshold float64 // mean no_speech_prob above this => NoSpeech (default 0.6)
}

// OpenAITranscriber posts to an OpenAI-compatible /audio/transcriptions endpoint.
type OpenAITranscriber struct {
	cfg    Config
	client *http.Client
}

// NewOpenAITranscriber constructs a transcriber from cfg.
func NewOpenAITranscriber(cfg Config) *OpenAITranscriber {
	if cfg.NoSpeechThreshold == 0 {
		cfg.NoSpeechThreshold = 0.6
	}
	return &OpenAITranscriber{cfg: cfg, client: &http.Client{Timeout: 120 * time.Second}}
}

// GroqWhisper returns the default free-tier transcriber (Groq whisper-large-v3-turbo).
func GroqWhisper(apiKey string) *OpenAITranscriber {
	return NewOpenAITranscriber(Config{
		Name:    "groq",
		BaseURL: "https://api.groq.com/openai/v1",
		APIKey:  apiKey,
		Model:   "whisper-large-v3-turbo",
	})
}

func (t *OpenAITranscriber) Available() bool {
	return t.cfg.BaseURL != "" && t.cfg.Model != "" && t.cfg.APIKey != ""
}

type verboseResponse struct {
	Text     string `json:"text"`
	Language string `json:"language"`
	Segments []struct {
		AvgLogprob   float64 `json:"avg_logprob"`
		NoSpeechProb float64 `json:"no_speech_prob"`
	} `json:"segments"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Transcribe uploads the audio and returns text plus a derived confidence.
func (t *OpenAITranscriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (Result, error) {
	if filename == "" {
		filename = "audio.ogg"
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		return Result{}, fmt.Errorf("stt %s: form file: %w", t.cfg.Name, err)
	}
	if _, err := io.Copy(fw, audio); err != nil {
		return Result{}, fmt.Errorf("stt %s: copy audio: %w", t.cfg.Name, err)
	}
	_ = mw.WriteField("model", t.cfg.Model)
	_ = mw.WriteField("response_format", "verbose_json")
	if t.cfg.Language != "" {
		_ = mw.WriteField("language", t.cfg.Language)
	}
	if err := mw.Close(); err != nil {
		return Result{}, fmt.Errorf("stt %s: close form: %w", t.cfg.Name, err)
	}

	url := strings.TrimRight(t.cfg.BaseURL, "/") + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return Result{}, fmt.Errorf("stt %s: request: %w", t.cfg.Name, err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if t.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.cfg.APIKey)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("stt %s: do: %w", t.cfg.Name, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("stt %s: status %d: %s", t.cfg.Name, resp.StatusCode, truncate(string(data), 300))
	}

	var vr verboseResponse
	if err := json.Unmarshal(data, &vr); err != nil {
		return Result{}, fmt.Errorf("stt %s: decode: %w", t.cfg.Name, err)
	}
	if vr.Error != nil {
		return Result{}, fmt.Errorf("stt %s: %s", t.cfg.Name, vr.Error.Message)
	}

	return t.toResult(vr), nil
}

// toResult derives a confidence (mean exp(avg_logprob)) and NoSpeech (mean
// no_speech_prob over threshold) from the segments.
func (t *OpenAITranscriber) toResult(vr verboseResponse) Result {
	res := Result{Text: strings.TrimSpace(vr.Text), Language: vr.Language}
	if len(vr.Segments) == 0 {
		// No segment stats; trust presence of text with neutral confidence.
		if res.Text != "" {
			res.Confidence = 0.5
		}
		return res
	}
	var sumProb, sumNoSpeech float64
	for _, s := range vr.Segments {
		sumProb += math.Exp(s.AvgLogprob) // logprob (<=0) -> 0..1
		sumNoSpeech += s.NoSpeechProb
	}
	n := float64(len(vr.Segments))
	res.Confidence = sumProb / n
	res.NoSpeech = (sumNoSpeech / n) > t.cfg.NoSpeechThreshold
	return res
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
