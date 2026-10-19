// Package buildinfo holds values set at link time by the release build:
//
//	go build -ldflags "-X github.com/mindfries/v2/cli/internal/buildinfo.Version=1.2.3 ..."
//
// Everything the candidate would otherwise have to configure is baked in
// here, so `mindfries start` needs no flags or environment variables.
package buildinfo

import "strings"

var (
	// Version is the CLI's semantic version.
	Version = "0.1.0-dev"
	// Commit is the Git commit the binary was built from.
	Commit = "unknown"
	// DefaultAPI is the V2 API base URL used when neither --api nor
	// MINDFRIES_API is given. Release builds set the real one; a build for
	// the hosted fake API sets https://mindfries-cli.onrender.com/v2/cli.
	// The V2 domain is still an open decision.
	DefaultAPI = "https://api.v2.mindfries.example/v2/cli"
	// TrustDevKey, set to "1" at link time, makes a dev build trust the
	// fake API's signing key from DefaultAPI's server, not only from
	// localhost. Builds for the hosted fake API set it; a release build
	// (a version without "-dev") ignores it.
	TrustDevKey = ""
)

// IsDev reports whether this is a development build.
func IsDev() bool { return strings.Contains(Version, "-dev") }
