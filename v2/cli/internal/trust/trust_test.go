package trust

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/mindfries/v2/cli/internal/api"
)

func manifest() *api.BundleManifest {
	return &api.BundleManifest{
		TaskVersion: "v1", ArchiveSHA256: "ab", ArchiveBytes: 10,
		Files: []api.BundleFile{{Path: "TASK.md", SHA256: "cd", Bytes: 3, Locked: true}},
		Git:   api.BundleGit{Branch: "b", AuthorName: "n", AuthorEmail: "e@x"},
	}
}

func raw(m *api.BundleManifest) []byte {
	b, _ := json.Marshal(m)
	return b
}

func TestDevKeyOnlyFromLocalhost(t *testing.T) {
	m := manifest()
	if err := SignManifest(m, DevKeyID, DevPrivateKey()); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"http://localhost:8787/v2/cli", "http://127.0.0.1:5/v2/cli", "http://[::1]:5/x"} {
		if err := VerifyManifest(raw(m), u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	if err := VerifyManifest(raw(m), "https://api.mindfries.example/v2/cli"); err == nil {
		t.Error("dev key accepted from a remote API")
	}
}

func TestTamperedManifestRejected(t *testing.T) {
	m := manifest()
	_ = SignManifest(m, DevKeyID, DevPrivateKey())
	m.Files[0].SHA256 = "ef"
	if err := VerifyManifest(raw(m), "http://localhost/x"); err == nil {
		t.Fatal("tampered manifest accepted")
	}
}

func TestUnsignedAndUnknownKey(t *testing.T) {
	m := manifest()
	if err := VerifyManifest(raw(m), "http://localhost/x"); err == nil {
		t.Fatal("unsigned manifest accepted")
	}
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	_ = SignManifest(m, "someone-else", priv)
	if err := VerifyManifest(raw(m), "http://localhost/x"); err == nil {
		t.Fatal("unknown key accepted")
	}
}

func TestBuiltInKey(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	old := ManifestKeys
	ManifestKeys = "prod-2026:" + base64.StdEncoding.EncodeToString(pub)
	defer func() { ManifestKeys = old }()
	m := manifest()
	_ = SignManifest(m, "prod-2026", priv)
	if err := VerifyManifest(raw(m), "https://api.mindfries.example/v2/cli"); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownFieldIsCovered(t *testing.T) {
	m := manifest()
	_ = SignManifest(m, DevKeyID, DevPrivateKey())
	var obj map[string]any
	_ = json.Unmarshal(raw(m), &obj)
	obj["future_field"] = "x"
	b, _ := json.Marshal(obj)
	if err := VerifyManifest(b, "http://localhost/x"); err == nil {
		t.Fatal("a field added after signing went unnoticed")
	}
}
