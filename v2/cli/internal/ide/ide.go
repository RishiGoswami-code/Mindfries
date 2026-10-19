// Package ide finds the editors installed on the machine and opens the
// workspace in the one the candidate picks.
//
// Supported: VS Code, JetBrains IDEs and Zed, all through their command-line
// launchers. Agent-first IDEs (Cursor, Windsurf, Antigravity) are detected
// only so they can be named as not offered (v2/PLAN.md, task 1.6).
package ide

import (
	"github.com/mindfries/v2/cli/internal/sys"
)

// Family groups editors that share an extension mechanism.
type Family string

const (
	VSCode    Family = "vscode"
	JetBrains Family = "jetbrains"
	Zed       Family = "zed"
	// Manual means the candidate opens the folder themselves.
	Manual Family = "manual"
)

// Editor is one installed editor.
type Editor struct {
	ID         string // e.g. "vscode", "pycharm"
	Name       string // for people
	Family     Family
	Path       string // the launcher found
	AgentFirst bool   // detected, but not offered
	// mac app bundles are opened with "open -na <app> --args <dir>".
	macApp bool
}

type known struct {
	id, name   string
	family     Family
	agentFirst bool
	commands   []string
	paths      map[string][]string // by GOOS
}

var catalog = []known{
	{id: "vscode", name: "VS Code", family: VSCode, commands: []string{"code"}, paths: map[string][]string{
		"windows": {`${LOCALAPPDATA}\Programs\Microsoft VS Code\bin\code.cmd`, `${ProgramFiles}\Microsoft VS Code\bin\code.cmd`},
		"darwin":  {"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code", "${HOME}/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"},
		"linux":   {"/usr/share/code/bin/code", "/snap/bin/code"},
	}},
	{id: "intellij", name: "IntelliJ IDEA", family: JetBrains, commands: []string{"idea", "idea64"}, paths: jbPaths("idea", "IntelliJ IDEA")},
	{id: "pycharm", name: "PyCharm", family: JetBrains, commands: []string{"pycharm", "charm"}, paths: jbPaths("pycharm", "PyCharm")},
	{id: "webstorm", name: "WebStorm", family: JetBrains, commands: []string{"webstorm"}, paths: jbPaths("webstorm", "WebStorm")},
	{id: "goland", name: "GoLand", family: JetBrains, commands: []string{"goland"}, paths: jbPaths("goland", "GoLand")},
	{id: "clion", name: "CLion", family: JetBrains, commands: []string{"clion"}, paths: jbPaths("clion", "CLion")},
	{id: "rider", name: "Rider", family: JetBrains, commands: []string{"rider"}, paths: jbPaths("rider", "Rider")},
	{id: "phpstorm", name: "PhpStorm", family: JetBrains, commands: []string{"phpstorm"}, paths: jbPaths("phpstorm", "PhpStorm")},
	{id: "rubymine", name: "RubyMine", family: JetBrains, commands: []string{"rubymine"}, paths: jbPaths("rubymine", "RubyMine")},
	{id: "rustrover", name: "RustRover", family: JetBrains, commands: []string{"rustrover"}, paths: jbPaths("rustrover", "RustRover")},
	{id: "zed", name: "Zed", family: Zed, commands: []string{"zed", "zeditor"}, paths: map[string][]string{
		"darwin": {"/Applications/Zed.app/Contents/MacOS/cli"},
		"linux":  {"${HOME}/.local/bin/zed"},
	}},
	{id: "cursor", name: "Cursor", agentFirst: true, commands: []string{"cursor"}, paths: map[string][]string{
		"windows": {`${LOCALAPPDATA}\Programs\cursor\Cursor.exe`},
		"darwin":  {"/Applications/Cursor.app"},
	}},
	{id: "windsurf", name: "Windsurf", agentFirst: true, commands: []string{"windsurf"}, paths: map[string][]string{
		"windows": {`${LOCALAPPDATA}\Programs\Windsurf\Windsurf.exe`},
		"darwin":  {"/Applications/Windsurf.app"},
	}},
	{id: "antigravity", name: "Antigravity", agentFirst: true, commands: []string{"antigravity"}, paths: map[string][]string{
		"windows": {`${LOCALAPPDATA}\Programs\Antigravity\Antigravity.exe`},
		"darwin":  {"/Applications/Antigravity.app"},
	}},
}

// jbPaths are where JetBrains launchers live outside PATH: Toolbox's
// generated scripts, and macOS app bundles.
func jbPaths(script, app string) map[string][]string {
	return map[string][]string{
		"windows": {`${LOCALAPPDATA}\JetBrains\Toolbox\scripts\` + script + `.cmd`},
		"darwin":  {"${HOME}/Library/Application Support/JetBrains/Toolbox/scripts/" + script, "/Applications/" + app + ".app"},
		"linux":   {"${HOME}/.local/share/JetBrains/Toolbox/scripts/" + script},
	}
}

// Detect returns the installed editors, supported ones first.
func Detect(s *sys.Sys) []Editor {
	var supported, agentFirst []Editor
	for _, k := range catalog {
		p := s.Find(k.commands, k.paths[s.GOOS])
		if p == "" {
			continue
		}
		e := Editor{ID: k.id, Name: k.name, Family: k.family, Path: p, AgentFirst: k.agentFirst}
		e.macApp = s.GOOS == "darwin" && len(p) > 4 && p[len(p)-4:] == ".app"
		if k.agentFirst {
			agentFirst = append(agentFirst, e)
		} else {
			supported = append(supported, e)
		}
	}
	return append(supported, agentFirst...)
}

// Supported filters out agent-first editors.
func Supported(eds []Editor) []Editor {
	var out []Editor
	for _, e := range eds {
		if !e.AgentFirst {
			out = append(out, e)
		}
	}
	return out
}
