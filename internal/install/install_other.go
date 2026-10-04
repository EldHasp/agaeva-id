//go:build !windows

package install

import "errors"

// Install copies the program for all users and registers logon autostart.
func Install(string) error {
	return errors.New("установка выполняется на Windows")
}

// Uninstall removes the shared copy and the logon autostart entry.
func Uninstall() error {
	return errors.New("удаление выполняется на Windows")
}

// InstalledPath is where a shared computer keeps the single copy.
func InstalledPath() string { return "" }
