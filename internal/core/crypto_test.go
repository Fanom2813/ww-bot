package core

import (
	"crypto/rand"
	"io"
	"testing"
)

func TestEncryptRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatal(err)
	}
	plain := "are you at home? +255712345678"

	enc := encryptText(key, plain)
	if enc == plain {
		t.Fatal("ciphertext should differ from plaintext")
	}
	if got := decryptText(key, enc); got != plain {
		t.Fatalf("round-trip mismatch: got %q", got)
	}
	// Wrong key must not decrypt to the plaintext.
	other := make([]byte, 32)
	_, _ = io.ReadFull(rand.Reader, other)
	if got := decryptText(other, enc); got == plain {
		t.Fatal("decrypted with wrong key")
	}
	// Legacy/plaintext (no key, or non-enc value) passes through unchanged.
	if got := decryptText(key, plain); got != plain {
		t.Fatalf("plaintext passthrough broken: %q", got)
	}
	if got := encryptText(nil, plain); got != plain {
		t.Fatalf("no-key should store plaintext, got %q", got)
	}
}
