package state

import (
	"os"
	"runtime"
	"time"
)

// renameReplace renames over an existing file. On Windows a reader holding
// the target open makes the rename fail briefly, so it retries.
func renameReplace(from, to string) error {
	var err error
	for i := 0; i < 20; i++ {
		if err = os.Rename(from, to); err == nil || runtime.GOOS != "windows" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		os.Remove(from)
	}
	return err
}
