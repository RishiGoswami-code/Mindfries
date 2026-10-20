// Package panel is what the floating panel is made of on the server side: the
// tabs a candidate gets (from the role's modes and the task), the shape of
// what they make in those tabs (a diagram, notes, review comments, debugging
// hypotheses), and the task material the tabs show. The panel itself is a
// page in the candidate portal; this keeps the rules in one place for every
// kind of candidate (v2/PANEL.md section 3).
package panel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Tab is one entry on the panel's rail.
type Tab struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Badge int    `json:"badge,omitempty"`
}

var labels = map[string]string{
	"design": "System Design", "notes": "Notes", "resources": "Resources",
	"review": "Code Review", "debug": "Debugging", "test": "Testing", "activity": "Activity",
}

// modeTab maps a manifest mode or a task kind to the tab it needs.
var modeTab = map[string]string{
	"design": "design", "system_design": "design", "architecture": "design", "system design": "design",
	"review": "review", "comprehension": "review",
	"debugging": "debug", "debug": "debug", "reliability": "debug",
	"testing": "test", "test": "test",
}

var order = []string{"design", "notes", "resources", "review", "debug", "test", "activity"}

// TabsFor is the rail for a candidate: Notes, Resources and Activity always,
// the rest from the modes the role is assessed in and the task's kind. A new
// role or product is a manifest change, not a code change.
func TabsFor(modes []string, taskKind string) []Tab {
	want := map[string]bool{"notes": true, "resources": true, "activity": true}
	for _, m := range append(append([]string{}, modes...), taskKind) {
		if t, ok := modeTab[strings.ToLower(strings.TrimSpace(m))]; ok {
			want[t] = true
		}
	}
	var out []Tab
	for _, id := range order {
		if want[id] {
			out = append(out, Tab{ID: id, Label: labels[id]})
		}
	}
	return out
}

// FirstTab is where the panel opens: the task's own mode first, else the
// first working tab, else Notes.
func FirstTab(tabs []Tab, taskKind string) string {
	has := func(id string) bool {
		for _, t := range tabs {
			if t.ID == id {
				return true
			}
		}
		return false
	}
	if own, ok := modeTab[strings.ToLower(taskKind)]; ok && has(own) {
		return own
	}
	for _, id := range []string{"design", "debug", "review", "test"} {
		if has(id) {
			return id
		}
	}
	return "notes"
}

// Artifact kinds the candidate saves.
const (
	Diagram    = "diagram"
	Notes      = "notes"
	Review     = "review"
	Hypotheses = "hypotheses"
)

// Kinds lists them.
var Kinds = []string{Diagram, Notes, Review, Hypotheses}

// ValidKind reports whether k is an artifact kind.
func ValidKind(k string) bool {
	for _, x := range Kinds {
		if x == k {
			return true
		}
	}
	return false
}

// MaxArtifactBytes is the most one artifact may hold.
const MaxArtifactBytes = 256 << 10

var tones = map[string]bool{"blue": true, "purple": true, "green": true, "sky": true, "amber": true, "red": true, "violet": true}

// Validate checks that body is a well-formed artifact of kind and returns it
// in canonical form (the fields the panel knows, nothing else). The panel is
// a browser: whatever it sends is untrusted, and the reviewer reads this.
func Validate(kind string, body json.RawMessage) (json.RawMessage, error) {
	if !ValidKind(kind) {
		return nil, fmt.Errorf("unknown artifact %q", kind)
	}
	if len(body) == 0 || len(body) > MaxArtifactBytes {
		return nil, fmt.Errorf("the %s must be between 1 byte and %d KB", kind, MaxArtifactBytes>>10)
	}
	switch kind {
	case Diagram:
		return validateDiagram(body)
	case Notes:
		var n struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(body, &n); err != nil {
			return nil, errors.New("notes must be {\"text\": …}")
		}
		if !utf8.ValidString(n.Text) || len(n.Text) > 100<<10 {
			return nil, errors.New("notes text is too long or not valid text")
		}
		return json.Marshal(n)
	case Review:
		var r struct {
			Comments []struct {
				File string `json:"file"`
				Line int    `json:"line"`
				Text string `json:"text"`
			} `json:"comments"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			return nil, errors.New("review must be {\"comments\": [{file, line, text}]}")
		}
		if len(r.Comments) > 500 {
			return nil, errors.New("too many review comments")
		}
		for _, c := range r.Comments {
			if c.File == "" || len(c.File) > 300 || c.Line < 0 || len(c.Text) > 4000 {
				return nil, errors.New("a review comment needs a file, a line and short text")
			}
		}
		if r.Comments == nil {
			r.Comments = nil
		}
		return json.Marshal(r)
	case Hypotheses:
		var h struct {
			Items []struct {
				Text   string `json:"text"`
				Status string `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal(body, &h); err != nil {
			return nil, errors.New("hypotheses must be {\"items\": [{text, status}]}")
		}
		if len(h.Items) > 200 {
			return nil, errors.New("too many hypotheses")
		}
		for _, i := range h.Items {
			if len(i.Text) > 2000 || (i.Status != "open" && i.Status != "confirmed" && i.Status != "ruled_out") {
				return nil, errors.New("a hypothesis needs short text and a status of open, confirmed or ruled_out")
			}
		}
		return json.Marshal(h)
	}
	return nil, errors.New("unreachable")
}

type diagramNode struct {
	ID       string  `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
	Title    string  `json:"title"`
	Subtitle string  `json:"subtitle,omitempty"`
	Tone     string  `json:"tone"`
	Icon     string  `json:"icon"`
}

type diagramEdge struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Dashed bool   `json:"dashed,omitempty"`
}

type diagramGroup struct {
	ID    string   `json:"id"`
	Nodes []string `json:"nodes"`
}

var icons = map[string]bool{"client": true, "balancer": true, "lock": true, "user": true, "file": true, "database": true, "storage": true, "cache": true, "box": true}

func validateDiagram(body json.RawMessage) (json.RawMessage, error) {
	var d struct {
		Title  string         `json:"title"`
		Nodes  []diagramNode  `json:"nodes"`
		Edges  []diagramEdge  `json:"edges"`
		Groups []diagramGroup `json:"groups,omitempty"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, errors.New("the diagram must be {title, nodes, edges}")
	}
	if len(d.Title) > 200 || len(d.Nodes) > 200 || len(d.Edges) > 400 || len(d.Groups) > 50 {
		return nil, errors.New("the diagram is too large")
	}
	ids := map[string]bool{}
	for i, n := range d.Nodes {
		if n.ID == "" || len(n.ID) > 64 || ids[n.ID] {
			return nil, errors.New("every node needs its own id")
		}
		ids[n.ID] = true
		if len(n.Title) > 200 || len(n.Subtitle) > 200 {
			return nil, errors.New("a node's text is too long")
		}
		for _, v := range []float64{n.X, n.Y, n.W, n.H} {
			if v < -50000 || v > 50000 {
				return nil, errors.New("a node is out of range")
			}
		}
		if n.W <= 0 || n.H <= 0 {
			return nil, errors.New("a node needs a size")
		}
		if !tones[n.Tone] {
			d.Nodes[i].Tone = "blue"
		}
		if !icons[n.Icon] {
			d.Nodes[i].Icon = "box"
		}
	}
	for _, e := range d.Edges {
		if e.ID == "" || !ids[e.From] || !ids[e.To] {
			return nil, errors.New("an arrow must connect two nodes that exist")
		}
	}
	for _, g := range d.Groups {
		for _, id := range g.Nodes {
			if !ids[id] {
				return nil, errors.New("a group names a node that doesn't exist")
			}
		}
	}
	if d.Nodes == nil {
		d.Nodes = []diagramNode{}
	}
	if d.Edges == nil {
		d.Edges = []diagramEdge{}
	}
	return json.Marshal(d)
}

// Resource is something the Resources tab lists.
type Resource struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Kind   string `json:"kind"` // doc | link | file
	Detail string `json:"detail"`
	Href   string `json:"href,omitempty"`
}

var urlRe = regexp.MustCompile(`https?://[^\s<>)\]"']+`)

// ResourcesFor lists what a candidate may read: the task brief (locked), the
// product's own docs, and any links the brief itself gives.
func ResourcesFor(brief, productDir string) []Resource {
	out := []Resource{{ID: "task", Title: "TASK.md", Kind: "file", Detail: "The task brief (locked)"}}
	if productDir != "" {
		var docs []string
		for _, pat := range []string{"README.md", "docs/*.md"} {
			m, _ := filepath.Glob(filepath.Join(productDir, pat))
			docs = append(docs, m...)
		}
		sort.Strings(docs)
		for i, p := range docs {
			if i >= 10 {
				break
			}
			rel, _ := filepath.Rel(productDir, p)
			rel = filepath.ToSlash(rel)
			out = append(out, Resource{ID: "doc-" + rel, Title: rel, Kind: "doc", Detail: "In the workspace"})
		}
	}
	seen := map[string]bool{}
	for _, u := range urlRe.FindAllString(brief, -1) {
		u = strings.TrimRight(u, ".,;:")
		if seen[u] || len(out) > 30 {
			continue
		}
		seen[u] = true
		host := u
		if i := strings.Index(u, "://"); i >= 0 {
			host = u[i+3:]
			if j := strings.IndexByte(host, '/'); j >= 0 {
				host = host[:j]
			}
		}
		out = append(out, Resource{ID: fmt.Sprintf("link-%d", len(seen)), Title: host, Kind: "link", Detail: host, Href: u})
	}
	return out
}

// ReviewLine is one line of a file shown in the review tab.
type ReviewLine struct {
	Kind string `json:"kind"` // " ", "+" or "-"
	Text string `json:"text"`
	N    int    `json:"n"`
}

// ReviewFile is a file in the review tab.
type ReviewFile struct {
	Path  string       `json:"path"`
	Lines []ReviewLine `json:"lines"`
}

// ReviewFromOverlay shows the files a review task ships (its overlay) as the
// code to read and comment on. Capped, so a big task can't make a huge page.
func ReviewFromOverlay(overlayDir string) []ReviewFile {
	var out []ReviewFile
	_ = filepath.WalkDir(overlayDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || len(out) >= 6 {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil || len(raw) > 200<<10 || !utf8.Valid(raw) {
			return nil
		}
		rel, _ := filepath.Rel(overlayDir, p)
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		if len(lines) > 400 {
			lines = lines[:400]
		}
		f := ReviewFile{Path: filepath.ToSlash(rel)}
		for i, l := range lines {
			f.Lines = append(f.Lines, ReviewLine{Kind: " ", Text: l, N: i + 1})
		}
		out = append(out, f)
		return nil
	})
	return out
}

// DebugLog reads the logs a debugging task ships (logs.txt in the task
// directory), if it has any.
func DebugLog(taskDir string) []string {
	raw, err := os.ReadFile(filepath.Join(taskDir, "logs.txt"))
	if err != nil || len(raw) > 200<<10 {
		return []string{}
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) > 500 {
		lines = lines[:500]
	}
	return lines
}
