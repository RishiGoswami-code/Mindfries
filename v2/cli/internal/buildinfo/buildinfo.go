// Package buildinfo holds values set at link time by the release build:
//
//	go build -ldflags "-X github.com/mindfries/v2/cli/internal/buildinfo.Version=1.2.3 ..."
package buildinfo

var (
	// Version is the CLI's semantic version.
	Version = "0.0.0-dev"
	// Commit is the Git commit the binary was built from.
	Commit = "unknown"
	// DefaultAPI is the V2 API base URL used when neither --api nor
	// MINDFRIES_API is given. The V2 domain is still an open decision.
	DefaultAPI = "https://api.v2.mindfries.example/v2/cli"
)
