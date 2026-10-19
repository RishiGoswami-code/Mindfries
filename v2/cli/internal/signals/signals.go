// Package signals looks for outside coding agents (v2/PLAN.md task 1.8).
// Candidates may write code with `mindfries agent`, not with their own
// agents. Every finding is a review signal for a person, never a verdict.
//
// What is read, and nothing else:
//  1. the names of running processes, and only matches are kept;
//  2. the editors' installed-extension lists (`code --list-extensions`,
//     JetBrains plugin folder names, Zed's settings file), and only AI
//     extensions are kept;
//  3. file and folder names in the workspace, and commit messages on the
//     assessment branch;
//  4. and 5. come from the watcher's own record of file changes.
//
// Known limit: an agent installed with npm (Claude Code, Gemini CLI, Codex)
// may show up only as "node". Names are not proof either way, which is why
// the avatar's follow-up questions check understanding.
package signals

import (
	"context"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mindfries/v2/cli/internal/sys"
)

// agentProcesses maps a process base name (lower case, no .exe) to the
// agent it belongs to.
var agentProcesses = map[string]string{
	"claude":       "claude-code",
	"claude-code":  "claude-code",
	"gemini":       "gemini-cli",
	"codex":        "codex",
	"aider":        "aider",
	"copilot":      "copilot-cli",
	"cursor-agent": "cursor",
	"opencode":     "opencode",
	"goose":        "goose",
	"amp":          "amp",
	"qwen":         "qwen-code",
	"crush":        "crush",
	"cline":        "cline",
	"auggie":       "augment",
	"kiro":         "kiro",
	"kiro-cli":     "kiro",
	"devin":        "devin",
}

// agentAppPrefixes catch agent-first editors, whose helper processes carry
// suffixes ("Cursor Helper (Renderer)").
var agentAppPrefixes = map[string]string{
	"cursor":      "cursor",
	"windsurf":    "windsurf",
	"antigravity": "antigravity",
	"kiro":        "kiro",
}

// ProcessMatch is one agent found running.
type ProcessMatch struct {
	Agent       string `json:"agent"`
	ProcessName string `json:"process_name"`
}

// MatchProcesses keeps only the process names that belong to known coding
// agents, one per agent.
func MatchProcesses(names []string) []ProcessMatch {
	seen := map[string]bool{}
	var out []ProcessMatch
	for _, n := range names {
		base := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(n, ".exe"), ".EXE"))
		agent, ok := agentProcesses[base]
		if !ok {
			for p, a := range agentAppPrefixes {
				if base == p || strings.HasPrefix(base, p+" ") || strings.HasPrefix(base, p+"-") {
					agent, ok = a, true
					break
				}
			}
		}
		if ok && !seen[agent] {
			seen[agent] = true
			out = append(out, ProcessMatch{Agent: agent, ProcessName: n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

// Processes lists the running coding agents.
func Processes(ctx context.Context, s *sys.Sys) ([]ProcessMatch, error) {
	names, err := s.Processes(ctx)
	if err != nil {
		return nil, err
	}
	return MatchProcesses(names), nil
}

// aiExtensions are VS Code extension ids (lower case) for AI coding tools.
var aiExtensions = map[string]bool{
	"github.copilot":                         true,
	"github.copilot-chat":                    true,
	"saoudrizwan.claude-dev":                 true, // Cline
	"continue.continue":                      true,
	"rooveterinaryinc.roo-cline":             true,
	"anthropic.claude-code":                  true,
	"google.geminicodeassist":                true,
	"google.gemini-cli-vscode-ide-companion": true,
	"codeium.codeium":                        true,
	"supermaven.supermaven":                  true,
	"tabnine.tabnine-vscode":                 true,
	"amazonwebservices.amazon-q-vscode":      true,
	"sourcegraph.cody-ai":                    true,
	"kilocode.kilo-code":                     true,
	"openai.chatgpt":                         true,
	"augment.vscode-augment":                 true,
	"blackboxapp.blackbox":                   true,
}

// aiPluginWords match JetBrains plugin folder names.
var aiPluginWords = []string{"ml-llm", "junie", "copilot", "continue", "codeium", "tabnine", "claude", "gemini", "amazon-q", "aws-toolkit", "cody", "supermaven", "augment"}

// Extension is the AI extensions found in one editor.
type Extension struct {
	IDE        string   `json:"ide"`
	Extensions []string `json:"extensions"`
}

// Extensions lists AI extensions in the installed editors. code is the VS
// Code launcher, or "" if there's none.
func Extensions(ctx context.Context, s *sys.Sys, code string) []Extension {
	var out []Extension
	if code != "" {
		if list, err := s.Output(ctx, "", code, "--list-extensions"); err == nil {
			var found []string
			for _, l := range strings.Split(list, "\n") {
				id := strings.ToLower(strings.TrimSpace(l))
				if aiExtensions[id] {
					found = append(found, id)
				}
			}
			if len(found) > 0 {
				out = append(out, Extension{IDE: "vscode", Extensions: found})
			}
		}
	}
	if found := jetbrainsPlugins(s); len(found) > 0 {
		out = append(out, Extension{IDE: "jetbrains", Extensions: found})
	}
	if found := zedAgent(s); len(found) > 0 {
		out = append(out, Extension{IDE: "zed", Extensions: found})
	}
	return out
}

func jetbrainsRoots(s *sys.Sys) []string {
	home := s.Home()
	switch s.GOOS {
	case "windows":
		if a := s.Getenv("APPDATA"); a != "" {
			return []string{filepath.Join(a, "JetBrains")}
		}
	case "darwin":
		return []string{filepath.Join(home, "Library", "Application Support", "JetBrains")}
	default:
		return []string{filepath.Join(home, ".local", "share", "JetBrains"), filepath.Join(home, ".config", "JetBrains")}
	}
	return nil
}

func jetbrainsPlugins(s *sys.Sys) []string {
	seen := map[string]bool{}
	var found []string
	for _, root := range jetbrainsRoots(s) {
		products, err := s.ReadDir(root)
		if err != nil {
			continue
		}
		for _, p := range products {
			for _, dir := range []string{filepath.Join(root, p, "plugins"), filepath.Join(root, p)} {
				names, err := s.ReadDir(dir)
				if err != nil {
					continue
				}
				for _, n := range names {
					ln := strings.ToLower(n)
					for _, w := range aiPluginWords {
						if strings.Contains(ln, w) && !seen[ln] {
							seen[ln] = true
							found = append(found, n)
						}
					}
				}
			}
		}
	}
	sort.Strings(found)
	return found
}

var zedAgentKeys = regexp.MustCompile(`"(default_model|language_models|agent_servers|context_servers)"`)

func zedAgent(s *sys.Sys) []string {
	var paths []string
	if s.GOOS == "windows" {
		if a := s.Getenv("APPDATA"); a != "" {
			paths = append(paths, filepath.Join(a, "Zed", "settings.json"))
		}
	} else {
		paths = append(paths, filepath.Join(s.Home(), ".config", "zed", "settings.json"))
	}
	var found []string
	for _, p := range paths {
		b, err := s.ReadFile(p)
		if err != nil {
			continue
		}
		text := string(b)
		if zedAgentKeys.MatchString(text) {
			found = append(found, "agent-configured")
		}
		if strings.Contains(text, `"copilot"`) {
			found = append(found, "copilot")
		}
	}
	return found
}

// leftoverNames are agent instruction files and folders, by base name.
var leftoverNames = map[string]bool{
	"claude.md": true, ".claude": true, "agents.md": true, "gemini.md": true, ".gemini": true,
	".cursor": true, ".cursorrules": true, ".windsurf": true, ".windsurfrules": true,
	".clinerules": true, ".roo": true, ".continue": true, ".junie": true, ".kiro": true,
	".augment": true, ".codex": true, "copilot-instructions.md": true, ".aider.conf.yml": true,
}

// IsLeftover reports whether a workspace path is an agent's file.
func IsLeftover(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		l := strings.ToLower(seg)
		if leftoverNames[l] || strings.HasPrefix(l, ".aider") {
			return true
		}
	}
	return strings.HasPrefix(strings.ToLower(p), ".github/instructions/")
}

// LeftoverRoots reduces paths to the agent file or folder that matched, so
// a .claude folder is one finding, not one per file inside it.
func LeftoverRoots(paths []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range paths {
		segs := strings.Split(p, "/")
		root := ""
		for i, seg := range segs {
			l := strings.ToLower(seg)
			if leftoverNames[l] || strings.HasPrefix(l, ".aider") {
				root = path.Join(segs[:i+1]...)
				break
			}
		}
		if root == "" && strings.HasPrefix(strings.ToLower(p), ".github/instructions/") {
			root = ".github/instructions"
		}
		if root != "" && !seen[root] {
			seen[root] = true
			out = append(out, root)
		}
	}
	sort.Strings(out)
	return out
}

var trailerRe = regexp.MustCompile(`(?i)^\s*(co-authored-by:.*\b(claude|anthropic|gpt|openai|copilot|gemini|cursor|aider|codex|devin|windsurf|codeium|cody|amp|jules)\b.*|generated (with|by) .*|🤖.*)$`)

// AgentTrailer returns the first line of a commit message that marks it
// as written by a coding agent, or "".
func AgentTrailer(msg string) string {
	for _, l := range strings.Split(msg, "\n") {
		if trailerRe.MatchString(l) {
			return strings.TrimSpace(l)
		}
	}
	return ""
}
