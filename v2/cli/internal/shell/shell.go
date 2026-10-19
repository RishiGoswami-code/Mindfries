// Package shell runs a command line the way the candidate's terminal would:
// cmd.exe on Windows, sh elsewhere.
package shell

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"time"
)

// Result of running a command.
type Result struct {
	ExitCode int
	Duration time.Duration
	TimedOut bool
}

// Run runs command in dir, writing its output to out. A command that can't
// start returns an error; one that runs and fails returns its exit code.
func Run(ctx context.Context, dir, command string, out io.Writer, timeout time.Duration) (Result, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", command)
	}
	c.Dir = dir
	c.Stdout = out
	c.Stderr = out
	c.WaitDelay = 2 * time.Second
	start := time.Now()
	err := c.Run()
	res := Result{Duration: time.Since(start)}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		res.ExitCode = ee.ExitCode()
		if ctx.Err() == context.DeadlineExceeded {
			res.TimedOut = true
		}
	case ctx.Err() == context.DeadlineExceeded:
		res.ExitCode = -1
		res.TimedOut = true
	default:
		return res, err
	}
	return res, nil
}
