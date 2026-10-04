//go:build !windows

package update

import "errors"

func applyScript(string) error {
	return errors.New("самообновление выполняется только в Windows")
}
