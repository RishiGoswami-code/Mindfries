//go:build !windows && !linux && !darwin

package term

func isTerminal(fd uintptr) bool            { return false }
func makeRaw(fd uintptr) (func(), error)    { return nil, ErrUnsupported }
func enableANSI(fd uintptr) (func(), error) { return nil, ErrUnsupported }
func width(fd uintptr) (int, error)         { return 0, ErrUnsupported }
