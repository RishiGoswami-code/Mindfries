// Package config reads every V2_* environment variable once, validates them,
// and hands the rest of the server a typed Config. Nothing else in v2/api
// calls os.Getenv, so the full list of settings is in this file (and in the
// table in api/README.md).
package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Config is the server's settings.
type Config struct {
	Port         string
	DatabaseURL  string // V2_DATABASE_URL, the v2_api role (required)
	ProductsRoot string
	PoolSize     int

	SigningKeyID string
	SigningKey   ed25519.PrivateKey // nil: the dev key

	AdminToken       string // V2_ADMIN_TOKEN: admin and ops routes
	PortalSecret     string // V2_PORTAL_SECRET: the portals' servers call /v2/portal/*
	PublicURL        string // V2_PUBLIC_URL: this API's own address, e.g. https://mindfries-cli.onrender.com
	PortalURL        string // V2_PORTAL_URL: the candidate portal, where the sign-in and panel pages live
	MinCLIVersion    string // V2_MIN_CLI_VERSION: older CLIs get cli_too_old (default 0.0.0)
	LatestCLIVersion string // V2_LATEST_CLI_VERSION
	CLIDownloadURL   string // V2_CLI_DOWNLOAD_URL: where the installers fetch builds from
	CLIDistDir       string // V2_CLI_DIST_DIR: or this server hosts the builds from a folder
	TrustProxy       bool   // V2_TRUST_PROXY=1: believe X-Forwarded-For/Proto (set on Render)

	EncryptionKey []byte // V2_ENCRYPTION_KEY: 32 bytes, base64; seals GitHub tokens

	GitHubClientID     string
	GitHubClientSecret string
	GitHubWebURL       string // V2_GITHUB_WEB_URL: https://github.com (a test double in tests)
	GitHubAPIURL       string // V2_GITHUB_API_URL: https://api.github.com

	GeminiBaseURL   string // V2_GEMINI_BASE_URL: the API root (a test double in tests)
	GeminiKey       string // V2_GEMINI_API_KEY
	GeminiModel     string // the text model for the assistant and evaluators
	GeminiLiveModel string // the Gemini Live model for the avatar

	StorageURL    string // V2_STORAGE_URL: the Supabase project URL
	StorageKey    string // V2_STORAGE_KEY: its service role key
	StorageBucket string

	SandboxURL string // V2_SANDBOX_URL / KEY: where candidate code is run (Daytona)
	SandboxKey string

	RetentionDays   int
	SessionTokenTTL time.Duration

	// Cost caps (model tokens). Zero means "no cap".
	MaxTokensPerSession int
	MaxTokensPerDay     int

	AllowedOrigins []string // the portals' origins, for the panel's browser calls
}

// Getenv is os.LookupEnv shaped, so tests can pass a map.
type Getenv func(key string) string

// Load reads and validates the configuration. required lists what must be set
// for this process (the server needs the database; the keygen tool does not).
func Load(get Getenv, requireDB bool) (Config, error) {
	var errs []error
	str := func(key, def string) string {
		if v := strings.TrimSpace(get(key)); v != "" {
			return v
		}
		return def
	}
	num := func(key string, def int) int {
		v := strings.TrimSpace(get(key))
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			errs = append(errs, fmt.Errorf("%s must be a non-negative integer", key))
			return def
		}
		return n
	}

	c := Config{
		Port:                str("PORT", "8080"),
		DatabaseURL:         str("V2_DATABASE_URL", ""),
		ProductsRoot:        str("V2_PRODUCTS_ROOT", "../products"),
		PoolSize:            num("V2_POOL_SIZE", 3),
		SigningKeyID:        str("V2_SIGNING_KEY_ID", ""),
		AdminToken:          str("V2_ADMIN_TOKEN", ""),
		PortalSecret:        str("V2_PORTAL_SECRET", ""),
		PublicURL:           strings.TrimRight(str("V2_PUBLIC_URL", str("RENDER_EXTERNAL_URL", "")), "/"),
		PortalURL:           strings.TrimRight(str("V2_PORTAL_URL", ""), "/"),
		MinCLIVersion:       str("V2_MIN_CLI_VERSION", "0.0.0"),
		LatestCLIVersion:    str("V2_LATEST_CLI_VERSION", "0.1.0"),
		CLIDownloadURL:      strings.TrimRight(str("V2_CLI_DOWNLOAD_URL", ""), "/"),
		CLIDistDir:          str("V2_DIST_DIR", ""),
		TrustProxy:          str("V2_TRUST_PROXY", "") == "1",
		GitHubClientID:      str("V2_GITHUB_CLIENT_ID", ""),
		GitHubClientSecret:  str("V2_GITHUB_CLIENT_SECRET", ""),
		GitHubWebURL:        str("V2_GITHUB_WEB_URL", ""),
		GitHubAPIURL:        str("V2_GITHUB_API_URL", ""),
		GeminiKey:           str("V2_GEMINI_API_KEY", str("GEMINI_API_KEY", "")),
		GeminiBaseURL:       strings.TrimRight(str("V2_GEMINI_BASE_URL", ""), "/"),
		GeminiModel:         str("V2_GEMINI_MODEL", str("GEMINI_MODEL", "gemini-3.8-flash")),
		GeminiLiveModel:     str("V2_GEMINI_LIVE_MODEL", "gemini-3.8-live"),
		StorageURL:          strings.TrimRight(str("V2_STORAGE_URL", ""), "/"),
		StorageKey:          str("V2_STORAGE_KEY", ""),
		StorageBucket:       str("V2_STORAGE_BUCKET", "v2-evidence"),
		SandboxURL:          strings.TrimRight(str("V2_SANDBOX_URL", ""), "/"),
		SandboxKey:          str("V2_SANDBOX_KEY", ""),
		RetentionDays:       num("V2_RETENTION_DAYS", 90),
		SessionTokenTTL:     time.Duration(num("V2_SESSION_TOKEN_HOURS", 24*7)) * time.Hour,
		MaxTokensPerSession: num("V2_MAX_TOKENS_PER_SESSION", 400000),
		MaxTokensPerDay:     num("V2_MAX_TOKENS_PER_DAY", 20000000),
	}
	if requireDB && c.DatabaseURL == "" {
		errs = append(errs, errors.New("V2_DATABASE_URL is required (Postgres URL for the v2_api role)"))
	}
	if seed := str("V2_SIGNING_PRIVATE_KEY", ""); seed != "" {
		b, err := base64.StdEncoding.DecodeString(seed)
		if err != nil || len(b) != ed25519.SeedSize {
			errs = append(errs, fmt.Errorf("V2_SIGNING_PRIVATE_KEY must be base64 of a %d-byte seed", ed25519.SeedSize))
		} else {
			c.SigningKey = ed25519.NewKeyFromSeed(b)
		}
	}
	if k := str("V2_ENCRYPTION_KEY", ""); k != "" {
		b, err := base64.StdEncoding.DecodeString(k)
		if err != nil || len(b) != 32 {
			errs = append(errs, errors.New("V2_ENCRYPTION_KEY must be base64 of 32 bytes"))
		} else {
			c.EncryptionKey = b
		}
	}
	for _, o := range strings.Split(str("V2_ALLOWED_ORIGINS", ""), ",") {
		if o = strings.TrimRight(strings.TrimSpace(o), "/"); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}
	if c.PoolSize < 1 {
		c.PoolSize = 1
	}
	return c, errors.Join(errs...)
}

// Has says which optional features are configured, so a route can answer
// "not configured" plainly instead of failing later.
// GitHub needs the OAuth app and the key that seals tokens in the meantime.
func (c Config) HasGitHub() bool {
	return c.GitHubClientID != "" && c.GitHubClientSecret != "" && len(c.EncryptionKey) == 32
}
func (c Config) HasGemini() bool  { return c.GeminiKey != "" }
func (c Config) HasStorage() bool { return c.StorageURL != "" && c.StorageKey != "" }
func (c Config) HasSandbox() bool { return c.SandboxURL != "" && c.SandboxKey != "" }
