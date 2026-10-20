package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"testing"

	"github.com/mindfries/v2/api/internal/delivery"
	"github.com/mindfries/v2/contracts/canonical"
)

func sampleWS() delivery.Workspace {
	return delivery.Workspace{
		Files: map[string]string{
			"TASK.md":            "# Do the thing",
			"internal/x/main.go": "package x\n",
			"go.mod":             "module x\n",
		},
		Locked:      []string{"TASK.md"},
		Check:       []string{"go", "test", "./..."},
		Runtimes:    []string{"go"},
		Branch:      "mindfries/assessment",
		TaskVersion: "task_abc123",
	}
}

func TestBuildManifestHashesAndMetadata(t *testing.T) {
	ws := sampleWS()
	m, archive, err := Build(ws, DevKeyID, DevKey())
	if err != nil {
		t.Fatal(err)
	}

	if m.TaskVersion != ws.TaskVersion {
		t.Fatalf("task version = %q", m.TaskVersion)
	}
	if m.ArchiveBytes != int64(len(archive)) {
		t.Fatalf("archive_bytes %d != archive len %d", m.ArchiveBytes, len(archive))
	}
	wantArchive := sha256.Sum256(archive)
	if m.ArchiveSHA256 != hex.EncodeToString(wantArchive[:]) {
		t.Fatal("archive_sha256 mismatch")
	}
	if len(m.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(m.Files))
	}
	// Files are sorted; each hash matches its content; TASK.md is locked.
	var sawLocked bool
	for _, f := range m.Files {
		want := sha256.Sum256([]byte(ws.Files[f.Path]))
		if f.SHA256 != hex.EncodeToString(want[:]) {
			t.Fatalf("file %s hash mismatch", f.Path)
		}
		if f.Path == "TASK.md" && f.Locked {
			sawLocked = true
		}
	}
	if !sawLocked {
		t.Fatal("TASK.md should be marked locked")
	}
	if len(m.RequiredRuntimes) != 1 || m.RequiredRuntimes[0].Name != "go" {
		t.Fatalf("runtimes = %+v", m.RequiredRuntimes)
	}
}

// The signature must verify exactly as the CLI's trust.VerifyManifest does:
// Ed25519 over the RFC 8785 canonical JSON of the manifest minus "signature",
// using the shared canonical package.
func TestSignatureVerifiesLikeTheCLI(t *testing.T) {
	m, _, err := Build(sampleWS(), DevKeyID, DevKey())
	if err != nil {
		t.Fatal(err)
	}
	if m.Signature == nil || m.Signature.KeyID != DevKeyID {
		t.Fatalf("missing/incorrect signature: %+v", m.Signature)
	}

	raw, _ := json.Marshal(m)
	var asMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatal(err)
	}
	delete(asMap, "signature")
	msg, err := canonical.Marshal(asMap)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature.Value)
	if err != nil {
		t.Fatal(err)
	}
	pub := DevKey().Public().(ed25519.PublicKey)
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("signature does not verify the way the CLI would — bundles would be rejected")
	}

	// Tampering with a field must break verification.
	asMap["task_version"] = json.RawMessage(`"tampered"`)
	bad, _ := canonical.Marshal(asMap)
	if ed25519.Verify(pub, bad, sig) {
		t.Fatal("a tampered manifest must not verify")
	}
}

func TestArchiveExtractsToTheSameFiles(t *testing.T) {
	ws := sampleWS()
	_, archive, err := Build(ws, DevKeyID, DevKey())
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	got := map[string]string{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		got[h.Name] = string(b)
	}
	for p, want := range ws.Files {
		if got[p] != want {
			t.Fatalf("archive file %s = %q, want %q", p, got[p], want)
		}
	}
}
