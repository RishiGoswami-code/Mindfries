//go:build !windows && !darwin && !linux

package secret

func platformKeychain() Store { return nil }
