// Package crypt seals the few secrets the server must hold for a while (a
// candidate's GitHub token between the OAuth callback and the CLI collecting
// it). AES-256-GCM with a random nonce; the key is V2_ENCRYPTION_KEY.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

// ErrOpen is returned when a sealed value can't be opened: the wrong key, or
// the value was altered.
var ErrOpen = errors.New("crypt: cannot open sealed value")

func gcm(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, errors.New("crypt: key must be 32 bytes")
	}
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// Seal encrypts plain; the nonce is the first bytes of the result. aad is
// authenticated but not stored (bind a token to its owner, so a sealed value
// copied to another row won't open).
func Seal(key, plain, aad []byte) ([]byte, error) {
	g, err := gcm(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, aad), nil
}

// Open reverses Seal.
func Open(key, sealed, aad []byte) ([]byte, error) {
	g, err := gcm(key)
	if err != nil {
		return nil, err
	}
	if len(sealed) < g.NonceSize() {
		return nil, ErrOpen
	}
	plain, err := g.Open(nil, sealed[:g.NonceSize()], sealed[g.NonceSize():], aad)
	if err != nil {
		return nil, ErrOpen
	}
	return plain, nil
}

// Derive makes a purpose-specific key from the master key, so the key that
// signs panel tokens is not the key that seals tokens.
func Derive(master []byte, purpose string) []byte {
	h := sha256.New()
	h.Write([]byte("mindfries-v2/"))
	h.Write([]byte(purpose))
	h.Write([]byte{0})
	h.Write(master)
	return h.Sum(nil)
}
