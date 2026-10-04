//go:build !windows

package install

import (
	"errors"
	"log"
)

// RunningAsService is always false outside Windows.
func RunningAsService() bool { return false }

// RunService is the Windows wake-up service.
func RunService(*log.Logger) error {
	return errors.New("служба запускается только в Windows")
}
