// Package stt is the speech-to-text layer for transcribing WhatsApp voice
// notes. A Transcriber turns audio bytes into text plus a confidence/no-speech
// signal so the brain can stay silent when the audio isn't understood. The
// default implementation targets any OpenAI-compatible /audio/transcriptions
// endpoint (Groq Whisper by default, free tier).
package stt

import (
	"context"
	"io"
)

// Result is a transcription outcome.
type Result struct {
	Text       string  // the transcribed text
	Language   string  // detected language (if provided)
	Confidence float64 // 0..1, derived from segment log-probs
	NoSpeech   bool    // true when the audio likely contains no clear speech
}

// Usable reports whether the result is confident enough to act on, given a
// minimum confidence threshold. The brain uses this to decide whether to reply.
func (r Result) Usable(minConfidence float64) bool {
	return !r.NoSpeech && r.Text != "" && r.Confidence >= minConfidence
}

// Transcriber turns audio into text.
type Transcriber interface {
	// Available reports whether the transcriber is configured.
	Available() bool
	// Transcribe reads audio (e.g. WhatsApp OGG/Opus) and returns a Result.
	// filename should carry the correct extension (e.g. "voice.ogg").
	Transcribe(ctx context.Context, audio io.Reader, filename string) (Result, error)
}
