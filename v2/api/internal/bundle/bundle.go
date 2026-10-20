// Package bundle turns an engine-delivered candidate workspace into the exact
// signed manifest + gzipped-tar archive the CLI downloads and verifies
// (v2/contracts/cli-api.yaml: GET /session/bundle and /archive).
//
// The wire structs here are the server's view of the protocol — they mirror the
// CLI's api.BundleManifest by json tag. The one piece that MUST be bit-identical
// to the CLI is the signing input: Ed25519 over the RFC 8785 canonical JSON of
// the manifest without its signature field. That canonicalization is the shared
// v2/contracts/canonical package, so the CLI's trust.VerifyManifest accepts what
// this produces.
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/mindfries/v2/api/internal/delivery"
	"github.com/mindfries/v2/contracts/canonical"
)

// Manifest is the signed task manifest (GET /session/bundle).
type Manifest struct {
	TaskVersion      string     `json:"task_version"`
	ArchiveSHA256    string     `json:"archive_sha256"`
	ArchiveBytes     int64      `json:"archive_bytes"`
	Files            []File     `json:"files"`
	Git              Git        `json:"git"`
	RequiredRuntimes []Runtime  `json:"required_runtimes,omitempty"`
	SetupCommands    []string   `json:"setup_commands,omitempty"`
	CheckCommands    []string   `json:"check_commands,omitempty"`
	Signature        *Signature `json:"signature,omitempty"`
}

// File is one file in the bundle, with the hash the CLI validates after extract.
type File struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int64  `json:"bytes"`
	Executable bool   `json:"executable,omitempty"`
	Locked     bool   `json:"locked,omitempty"`
}

// Git is the local identity + branch the CLI sets up.
type Git struct {
	Branch      string `json:"branch"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

// Runtime is a required runtime (feeds the CLI's preflight).
type Runtime struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	InstallURL string `json:"install_url,omitempty"`
}

// Signature is the Ed25519 signature envelope.
type Signature struct {
	KeyID string `json:"key_id"`
	Value string `json:"value"`
}

// DevKeyID and DevKey are the fake API's dev signing key — the SAME fixed seed
// the CLI's trust package derives, so a dev build (or MINDFRIES_TRUST_DEV_KEY=1)
// trusts bundles this server signs. Production uses a real key (Slice D).
const DevKeyID = "dev-fakeapi"

// DevKey derives the dev Ed25519 private key from the shared fixed seed.
func DevKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("mindfries-v2-fakeapi-dev-signing-key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

// tarEpoch is a fixed mod-time so the archive is byte-deterministic.
var tarEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// Build assembles the signed manifest and the gzipped-tar archive for ws.
func Build(ws delivery.Workspace, keyID string, key ed25519.PrivateKey) (Manifest, []byte, error) {
	paths := make([]string, 0, len(ws.Files))
	for p := range ws.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths) // deterministic order

	locked := map[string]bool{}
	for _, p := range ws.Locked {
		locked[p] = true
	}

	// ponytail: executable bits aren't tracked on the workspace yet (P1–P3 have
	// none). Everything ships 0644; add an Executable set to delivery.Workspace
	// when a product needs a runnable script.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := make([]File, 0, len(paths))
	for _, p := range paths {
		content := []byte(ws.Files[p])
		if err := tw.WriteHeader(&tar.Header{
			Name: p, Mode: 0o644, Size: int64(len(content)), ModTime: tarEpoch, Typeflag: tar.TypeReg,
		}); err != nil {
			return Manifest{}, nil, err
		}
		if _, err := tw.Write(content); err != nil {
			return Manifest{}, nil, err
		}
		sum := sha256.Sum256(content)
		files = append(files, File{
			Path: p, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(content)), Locked: locked[p],
		})
	}
	if err := tw.Close(); err != nil {
		return Manifest{}, nil, err
	}
	if err := gz.Close(); err != nil {
		return Manifest{}, nil, err
	}
	archive := buf.Bytes()
	asum := sha256.Sum256(archive)

	m := Manifest{
		TaskVersion:      ws.TaskVersion,
		ArchiveSHA256:    hex.EncodeToString(asum[:]),
		ArchiveBytes:     int64(len(archive)),
		Files:            files,
		Git:              Git{Branch: ws.Branch, AuthorName: "Candidate", AuthorEmail: "candidate@assessment.mindfries.invalid"},
		RequiredRuntimes: runtimes(ws.Runtimes),
		CheckCommands:    ws.Check,
	}
	if err := sign(&m, keyID, key); err != nil {
		return Manifest{}, nil, err
	}
	return m, archive, nil
}

func sign(m *Manifest, keyID string, key ed25519.PrivateKey) error {
	m.Signature = nil
	msg, err := canonical.MarshalWithout(m, "signature")
	if err != nil {
		return fmt.Errorf("bundle: canonicalize: %w", err)
	}
	m.Signature = &Signature{KeyID: keyID, Value: base64.StdEncoding.EncodeToString(ed25519.Sign(key, msg))}
	return nil
}

func runtimes(names []string) []Runtime {
	var out []Runtime
	for _, n := range names {
		switch n {
		case "go":
			out = append(out, Runtime{Name: "go", Version: ">=1.22", InstallURL: "https://go.dev/dl/"})
		case "python":
			out = append(out, Runtime{Name: "python", Version: ">=3.10", InstallURL: "https://www.python.org/downloads/"})
		default:
			out = append(out, Runtime{Name: n})
		}
	}
	return out
}
