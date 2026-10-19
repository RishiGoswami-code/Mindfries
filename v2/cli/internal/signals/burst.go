package signals

import (
	"sort"
	"time"
)

// Arrival is code arriving in one file between two scans, not from
// `mindfries agent`.
type Arrival struct {
	Path  string
	Lines int
	At    time.Time
}

// Burst is signal 4: a lot of code arriving within seconds. People write in
// small edits spread over time; agents write whole files at once.
type Burst struct {
	Paths      []string `json:"paths"`
	LinesAdded int      `json:"lines_added"`
	WindowMS   int64    `json:"window_ms"`
}

// BurstDetector watches arrivals over a sliding window.
type BurstDetector struct {
	// Window is how far back arrivals are added up.
	Window time.Duration
	// Lines in the window that make a burst.
	Lines int
	// Files each with at least FileLines, within FileWindow, make a burst.
	Files      int
	FileLines  int
	FileWindow time.Duration

	recent []Arrival
}

// NewBurstDetector has the default thresholds: 120 lines in 10 seconds, or
// 3 files of 20+ lines each within 5 seconds.
func NewBurstDetector() *BurstDetector {
	return &BurstDetector{Window: 10 * time.Second, Lines: 120, Files: 3, FileLines: 20, FileWindow: 5 * time.Second}
}

// Observe adds an arrival and returns a burst if one just happened.
func (d *BurstDetector) Observe(a Arrival) *Burst {
	if a.Lines <= 0 {
		return nil
	}
	d.recent = append(d.recent, a)
	cut := a.At.Add(-d.Window)
	keep := d.recent[:0]
	for _, r := range d.recent {
		if !r.At.Before(cut) {
			keep = append(keep, r)
		}
	}
	d.recent = keep

	total := 0
	for _, r := range d.recent {
		total += r.Lines
	}
	bigFiles := map[string]bool{}
	fcut := a.At.Add(-d.FileWindow)
	for _, r := range d.recent {
		if !r.At.Before(fcut) && r.Lines >= d.FileLines {
			bigFiles[r.Path] = true
		}
	}
	if total < d.Lines && len(bigFiles) < d.Files {
		return nil
	}
	b := &Burst{LinesAdded: total, WindowMS: a.At.Sub(d.recent[0].At).Milliseconds()}
	seen := map[string]bool{}
	for _, r := range d.recent {
		if !seen[r.Path] {
			seen[r.Path] = true
			b.Paths = append(b.Paths, r.Path)
		}
	}
	sort.Strings(b.Paths)
	d.recent = nil
	return b
}
