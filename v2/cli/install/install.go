// Package install holds the one-line installers, embedded so the fake API
// can serve them next to its CLI builds.
package install

import _ "embed"

// DefaultDownload is the release base URL both installers use unless
// MINDFRIES_DOWNLOAD says otherwise. A server that hosts its own builds
// (the fake API) replaces it in the scripts it serves.
const DefaultDownload = "https://get.v2.mindfries.example/cli/latest"

// Shell is install.sh, for macOS, Linux and Git Bash.
//
//go:embed install.sh
var Shell string

// PowerShell is install.ps1, for Windows.
//
//go:embed install.ps1
var PowerShell string
