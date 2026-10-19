// Package trust verifies the Ed25519 signature on a task manifest. The CLI
// carries the public keys; a manifest it can't verify is rejected.
package trust

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/canonical"
)

// ManifestKeys is set at link time by the release build, as
// "key-id:base64-public-key,...". It is empty until the production signing
// key exists.
var ManifestKeys = ""

// DevKeyID is the fake API's key. The CLI accepts it only from an API on
// this machine (localhost), so a dev key can never vouch for a real task.
const DevKeyID = "dev-fakeapi"

// AllowDevKeyAnywhere trusts the dev key from any API, for a throwaway
// deployment of the fake API. The CLI sets it only for dev builds: baked in
// with buildinfo.TrustDevKey, or MINDFRIES_TRUST_DEV_KEY=1. Release builds never do.
var AllowDevKeyAnywhere = false

// DevPrivateKey is the fake API's signing key. It is derived from a fixed
// seed on purpose: it is public, and only trusted for localhost.
func DevPrivateKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("mindfries-v2-fakeapi-dev-signing-key"))
	return ed25519.NewKeyFromSeed(seed[:])
}

// SignManifest signs m in place with key.
func SignManifest(m *api.BundleManifest, keyID string, key ed25519.PrivateKey) error {
	m.Signature = nil
	msg, err := canonical.MarshalWithout(m, "signature")
	if err != nil {
		return err
	}
	m.Signature = &api.Signature{KeyID: keyID, Value: base64.StdEncoding.EncodeToString(ed25519.Sign(key, msg))}
	return nil
}

// VerifyManifest checks the signature on a manifest exactly as received.
// It works on the raw JSON, not a decoded struct, so a field this version
// of the CLI doesn't know about is still covered. apiURL decides whether
// the dev key is acceptable.
func VerifyManifest(raw []byte, apiURL string) error {
	var env struct {
		Signature *api.Signature `json:"signature"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("the task manifest is unreadable: %w", err)
	}
	if env.Signature == nil {
		return errors.New("the task manifest isn't signed")
	}
	pub, err := keyFor(env.Signature.KeyID, apiURL)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(env.Signature.Value)
	if err != nil {
		return errors.New("the task manifest's signature is malformed")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	delete(m, "signature")
	msg, err := canonical.Marshal(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, msg, sig) {
		return errors.New("the task manifest's signature doesn't match: it may have been altered")
	}
	return nil
}

func keyFor(id, apiURL string) (ed25519.PublicKey, error) {
	for _, entry := range strings.Split(ManifestKeys, ",") {
		kid, b64, ok := strings.Cut(strings.TrimSpace(entry), ":")
		if !ok || kid != id {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(b64)
		if err != nil || len(b) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("built-in key %s is malformed", kid)
		}
		return ed25519.PublicKey(b), nil
	}
	if id == DevKeyID {
		if !isLocal(apiURL) && !AllowDevKeyAnywhere {
			return nil, errors.New("the task manifest is signed with the development key, which is only trusted from a local test server")
		}
		return DevPrivateKey().Public().(ed25519.PublicKey), nil
	}
	return nil, fmt.Errorf("the task manifest is signed with an unknown key (%s); update mindfries by running the setup command again", id)
}

func isLocal(apiURL string) bool {
	u, err := url.Parse(apiURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
