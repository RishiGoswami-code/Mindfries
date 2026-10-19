package watcher

import (
	"context"
	"time"
)

type extraLoop struct {
	every time.Duration
	run   func(context.Context)
}

// extraLoops are periodic jobs beyond the core ones.
func (w *Watcher) extraLoops() []extraLoop { return nil }
