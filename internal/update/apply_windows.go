//go:build windows

package update

import (
	"os"
	"os/exec"
)

func applyScript(script string) error {
	cmd := exec.Command("cmd", "/c", "start", "", "/min", script)
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
