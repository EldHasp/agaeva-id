//go:build !windows

package install

func AlreadyRunning() bool { return false }
