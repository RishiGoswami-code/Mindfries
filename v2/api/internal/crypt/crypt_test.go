package crypt

import (
	"bytes"
	"testing"
)

func TestSealOpen(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	sealed, err := Seal(key, []byte("gho_secret"), []byte("cand-1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("gho_secret")) {
		t.Fatal("the secret must not appear in the sealed value")
	}
	got, err := Open(key, sealed, []byte("cand-1"))
	if err != nil || string(got) != "gho_secret" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := Open(key, sealed, []byte("cand-2")); err != ErrOpen {
		t.Fatal("a sealed value bound to one owner must not open for another")
	}
	if _, err := Open(bytes.Repeat([]byte{2}, 32), sealed, []byte("cand-1")); err != ErrOpen {
		t.Fatal("the wrong key must not open it")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := Open(key, sealed, []byte("cand-1")); err != ErrOpen {
		t.Fatal("an altered value must not open")
	}
	if _, err := Open(key, []byte("short"), nil); err != ErrOpen {
		t.Fatal("a too-short value must not open")
	}
	if _, err := Seal([]byte("short key"), []byte("x"), nil); err == nil {
		t.Fatal("a key that isn't 32 bytes is refused")
	}
	b, _ := Seal(key, []byte("same"), nil)
	c, _ := Seal(key, []byte("same"), nil)
	if bytes.Equal(b, c) {
		t.Fatal("each seal uses a fresh nonce")
	}
}

func TestDeriveSeparatesPurposes(t *testing.T) {
	m := bytes.Repeat([]byte{9}, 32)
	if bytes.Equal(Derive(m, "panel"), Derive(m, "github")) || len(Derive(m, "panel")) != 32 {
		t.Fatal("different purposes need different keys")
	}
	if !bytes.Equal(Derive(m, "panel"), Derive(m, "panel")) {
		t.Fatal("derivation is deterministic")
	}
}
