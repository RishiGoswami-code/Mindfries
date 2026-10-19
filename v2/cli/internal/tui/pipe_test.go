package tui

import "io"

func ioPipe() (*io.PipeReader, *io.PipeWriter) { return io.Pipe() }
