package stt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranscribeVerbose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/audio/transcriptions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("want multipart, got %q", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		if r.FormValue("response_format") != "verbose_json" {
			t.Errorf("want verbose_json, got %q", r.FormValue("response_format"))
		}
		if _, _, err := r.FormFile("file"); err != nil {
			t.Errorf("missing file field: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"hello world","language":"english","segments":[{"avg_logprob":-0.2,"no_speech_prob":0.01}]}`))
	}))
	defer srv.Close()

	tr := NewOpenAITranscriber(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Model: "whisper"})
	res, err := tr.Transcribe(context.Background(), strings.NewReader("fakeaudio"), "voice.ogg")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "hello world" {
		t.Fatalf("text: %q", res.Text)
	}
	if res.NoSpeech {
		t.Fatal("should not be flagged no-speech")
	}
	if res.Confidence < 0.7 { // exp(-0.2) ≈ 0.82
		t.Fatalf("confidence too low: %v", res.Confidence)
	}
	if !res.Usable(0.5) {
		t.Fatal("should be usable")
	}
}

func TestTranscribeNoSpeech(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"text":"","segments":[{"avg_logprob":-3.0,"no_speech_prob":0.9}]}`))
	}))
	defer srv.Close()

	tr := NewOpenAITranscriber(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Model: "whisper"})
	res, err := tr.Transcribe(context.Background(), strings.NewReader("x"), "voice.ogg")
	if err != nil {
		t.Fatal(err)
	}
	if !res.NoSpeech {
		t.Fatal("want NoSpeech true for high no_speech_prob")
	}
	if res.Usable(0.5) {
		t.Fatal("no-speech result should not be usable")
	}
}

func TestTranscribeErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	tr := NewOpenAITranscriber(Config{Name: "t", BaseURL: srv.URL, APIKey: "k", Model: "whisper"})
	if _, err := tr.Transcribe(context.Background(), strings.NewReader("x"), "voice.ogg"); err == nil {
		t.Fatal("want error on 401")
	}
}

func TestAvailability(t *testing.T) {
	if (&OpenAITranscriber{cfg: Config{BaseURL: "x", Model: "m"}}).Available() {
		t.Fatal("missing key should be unavailable")
	}
	if !GroqWhisper("key").Available() {
		t.Fatal("groq with key should be available")
	}
}
