package assistant

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The assistant's limits (v2/PLAN.md task 1.16). It never:
//   - reads or writes outside the task folder (no "..", no absolute paths,
//     no symlinks that lead out, nothing under .git);
//   - touches credentials (key files, .env files, cloud and package-manager
//     credentials, keychains);
//   - pushes to Git or changes where Git pushes;
//   - installs anything without the candidate seeing a warning and saying
//     yes (every command needs a yes anyway).
//
// A shell command can't be fully sandboxed by inspecting its text; the
// command checks refuse the obvious routes out, and the real safeguard is
// that every command is shown and runs only on the candidate's yes.

// credentialNames are files the assistant may not read or write, by base
// name (lower case).
var credentialNames = regexp.MustCompile(`^(\.env(\..*)?|.*\.(pem|key|p12|pfx|jks|keystore|kdbx)|id_(rsa|dsa|ecdsa|ed25519)(\.pub)?|\.netrc|_netrc|\.npmrc|\.pypirc|\.git-credentials|credentials(\.json)?|\.htpasswd|secrets?\.(json|ya?ml|toml))$`)

// credentialDirs are folders the assistant may not enter.
var credentialDirs = map[string]bool{".ssh": true, ".aws": true, ".gnupg": true, ".docker": true, ".kube": true, ".azure": true, ".config/gcloud": true}

func isCredentialPath(p string) bool {
	p = strings.ToLower(filepath.ToSlash(p))
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if credentialDirs[s] || (i+1 < len(segs) && credentialDirs[s+"/"+segs[i+1]]) {
			return true
		}
	}
	return credentialNames.MatchString(segs[len(segs)-1])
}

// inside reports whether full, once symlinks are followed, is still under
// root. A path that doesn't exist yet is judged by its nearest existing
// parent.
func inside(root, full string) (bool, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	p := full
	var rest []string
	for {
		real, err := filepath.EvalSymlinks(p)
		if err == nil {
			p = filepath.Join(append([]string{real}, rest...)...)
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return false, nil
		}
		rest = append([]string{filepath.Base(p)}, rest...)
		p = parent
	}
	r, err := filepath.Rel(realRoot, p)
	if err != nil {
		return false, nil
	}
	return r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)), nil
}

// blockedCommands are refused outright, with the reason shown.
var blockedCommands = []struct {
	re  *regexp.Regexp
	why string
}{
	{regexp.MustCompile(`(?i)\bgit\b[^|;&\n]*\bpush\b`), "pushing to Git isn't allowed; 'mindfries submit' hands your work in"},
	{regexp.MustCompile(`(?i)\bgit\b[^|;&\n]*\bremote\s+(add|set-url|rename)\b`), "changing Git remotes isn't allowed"},
	{regexp.MustCompile(`(?i)\bgit\b[^|;&\n]*\bconfig\b[^|;&\n]*--(global|system)\b`), "changing your Git settings outside the task isn't allowed"},
	{regexp.MustCompile(`(?i)\bgit\b[^|;&\n]*\bcredential`), "Git credentials are off limits"},
	{regexp.MustCompile(`(?i)(~|\$home|%userprofile%|\$env:userprofile)[/\\]`), "commands may only touch the task folder"},
	{regexp.MustCompile(`(^|[\s"'=(/\\])\.\.([/\\]|\s|$|["')])`), "commands may only touch the task folder"},
	{regexp.MustCompile(`(?i)(^|[\s"'=])(/(etc|usr|var|home|users|root|opt|private)\b|[a-z]:[\\/])`), "commands may only touch the task folder"},
	{regexp.MustCompile(`(?i)(\.ssh|\.aws|\.gnupg|\.kube|\.netrc|\.git-credentials|\.npmrc|\.pypirc|id_rsa|id_ed25519|\.env\b)`), "credentials are off limits"},
	{regexp.MustCompile(`(?i)\b(security\s+(find|dump|export)-|secret-tool\b|cmdkey\b|vaultcmd\b|keyring\b|printenv\b)`), "credentials are off limits"},
	{regexp.MustCompile(`(?i)((^|[\s;&|(])mindfries(\.exe)?(\s|$|[;&|)])|mindfries_[a-z]+=|\$env:mindfries_|\$mindfries_|%mindfries_)`), "the assistant can't run or inspect mindfries itself"},
}

// installCommands download and install software.
var installCommands = regexp.MustCompile(`(?i)\b(npm\s+(i|install|add|ci)\b|yarn\s+(add|install)\b|pnpm\s+(add|i|install)\b|bun\s+(add|install)\b|pip3?\s+install\b|python3?\s+-m\s+pip\s+install\b|uv\s+(add|pip\s+install|sync)\b|poetry\s+(add|install)\b|pipenv\s+install\b|conda\s+install\b|go\s+(get|install)\b|cargo\s+(add|install)\b|gem\s+install\b|bundle\s+install\b|composer\s+(require|install)\b|dotnet\s+(add|tool\s+install)\b|brew\s+install\b|apt(-get)?\s+install\b|dnf\s+install\b|yum\s+install\b|choco\s+install\b|winget\s+install\b|scoop\s+install\b|curl\b[^|]*\|\s*(sh|bash)\b)`)
