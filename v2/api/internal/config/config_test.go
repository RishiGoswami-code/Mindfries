package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func env(m map[string]string) Getenv { return func(k string) string { return m[k] } }

func TestDefaultsAndRequiredDB(t *testing.T) {
	if _, err := Load(env(nil), true); err == nil || !strings.Contains(err.Error(), "V2_DATABASE_URL") {
		t.Fatalf("want a missing-database error, got %v", err)
	}
	c, err := Load(env(nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != "8080" || c.PoolSize != 3 || c.RetentionDays != 90 || c.StorageBucket != "v2-evidence" {
		t.Fatalf("defaults: %+v", c)
	}
	if c.HasGitHub() || c.HasGemini() || c.HasStorage() || c.HasSandbox() {
		t.Fatal("nothing is configured, nothing should report as configured")
	}
}

func TestKeysAreValidated(t *testing.T) {
	seed := base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize))
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	c, err := Load(env(map[string]string{
		"V2_DATABASE_URL": "postgres://x", "V2_SIGNING_PRIVATE_KEY": seed, "V2_ENCRYPTION_KEY": key,
		"V2_ALLOWED_ORIGINS": "https://a.example/, https://b.example",
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	if c.SigningKey == nil || len(c.EncryptionKey) != 32 || len(c.AllowedOrigins) != 2 || c.AllowedOrigins[0] != "https://a.example" {
		t.Fatalf("%+v", c)
	}
	_, err = Load(env(map[string]string{"V2_SIGNING_PRIVATE_KEY": "not-base64!", "V2_ENCRYPTION_KEY": "AAAA", "V2_POOL_SIZE": "x"}), false)
	if err == nil {
		t.Fatal("bad values should be reported")
	}
	for _, want := range []string{"V2_SIGNING_PRIVATE_KEY", "V2_ENCRYPTION_KEY", "V2_POOL_SIZE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %s: %v", want, err)
		}
	}
}
