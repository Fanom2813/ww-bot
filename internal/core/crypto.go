package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"

	"github.com/zalando/go-keyring"
)

// Stored message history is encrypted at rest with AES-256-GCM. The key lives in
// the OS keychain (never on disk), so a stolen DB file alone reveals nothing.

const (
	dbKeyAccount = "__db_encryption_key__"
	encPrefix    = "enc:"
)

// loadOrCreateMsgKey returns the 32-byte key for encrypting stored messages,
// generating and persisting one in the OS keychain on first use. Returns nil if
// no key could be obtained (encryption then degrades to plaintext storage).
func loadOrCreateMsgKey() []byte {
	if v, err := keyring.Get(keyringService, dbKeyAccount); err == nil && v != "" {
		if k, err := base64.StdEncoding.DecodeString(v); err == nil && len(k) == 32 {
			return k
		}
	}
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil
	}
	if err := keyring.Set(keyringService, dbKeyAccount, base64.StdEncoding.EncodeToString(k)); err != nil {
		return nil // can't persist the key → don't encrypt (we'd never decrypt)
	}
	return k
}

// encryptText returns "enc:"+base64(nonce||ciphertext); returns plain unchanged
// if no key is available.
func encryptText(key []byte, plain string) string {
	gcm := newGCM(key)
	if gcm == nil {
		return plain
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plain
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ct)
}

// decryptText reverses encryptText; returns the input unchanged if it isn't an
// encrypted value (so any legacy plaintext rows still read).
func decryptText(key []byte, s string) string {
	if !strings.HasPrefix(s, encPrefix) {
		return s
	}
	gcm := newGCM(key)
	if gcm == nil {
		return s
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, encPrefix))
	if err != nil || len(raw) < gcm.NonceSize() {
		return s
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return s
	}
	return string(pt)
}

func newGCM(key []byte) cipher.AEAD {
	if len(key) != 32 {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil
	}
	return gcm
}
