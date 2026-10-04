//go:build windows

package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/update"
)

const serviceName = "AgaevaId"

func programData() string {
	if v := os.Getenv("ProgramData"); v != "" {
		return v
	}
	return `C:\ProgramData`
}

// InstalledPath is the single copy shared by everyone who logs on to this PC.
func InstalledPath() string {
	return filepath.Join(programData(), "AgaevaId", "agaeva-id.exe")
}

// Install copies the program once and registers a Windows service.
// The service does not print. It starts the same program inside the interactive
// user session, where Chrome, Edge, and Yandex Browser are visible.
func Install(currentExe string) error {
	dest := InstalledPath()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("нет прав записать программу в %s: %w", filepath.Dir(dest), err)
	}
	if !sameFile(currentExe, dest) {
		if err := copyFile(currentExe, dest); err != nil {
			return err
		}
	}
	removeLegacyAutostart()
	if err := update.EnsureDisabled(update.SettingsPath()); err != nil {
		return err
	}
	if err := registerService(dest); err != nil {
		return err
	}
	return exec.Command("sc", "start", serviceName).Run()
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func registerService(dest string) error {
	bin := fmt.Sprintf(`"%s"`, dest)
	_ = exec.Command("sc", "stop", serviceName).Run()
	query := exec.Command("sc", "query", serviceName)
	if err := query.Run(); err != nil {
		create := exec.Command("sc", "create", serviceName,
			"binPath=", bin,
			"start=", "auto",
			"DisplayName=", "Агаева ID",
		)
		if out, err := create.CombinedOutput(); err != nil {
			return fmt.Errorf("не удалось создать службу (нужны права администратора): %s", trim(out))
		}
	} else {
		config := exec.Command("sc", "config", serviceName, "binPath=", bin, "start=", "auto")
		if out, err := config.CombinedOutput(); err != nil {
			return fmt.Errorf("не удалось обновить службу: %s", trim(out))
		}
	}
	_ = exec.Command("sc", "description", serviceName, "Будит помощника вкладки Битрикс24 в сессии вошедшего пользователя").Run()
	failure := exec.Command("sc", "failure", serviceName, "reset=", "86400", "actions=", "restart/5000/restart/5000/restart/5000")
	_ = failure.Run()
	return nil
}

// Uninstall stops the service and removes the shared copy.
func Uninstall() error {
	_ = exec.Command("sc", "stop", serviceName).Run()
	if out, err := exec.Command("sc", "delete", serviceName).CombinedOutput(); err != nil {
		return fmt.Errorf("не удалось удалить службу: %s", trim(out))
	}
	removeLegacyAutostart()
	_ = os.Remove(InstalledPath())
	return nil
}

func removeLegacyAutostart() {
	_ = exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "AgaevaId", "/f").Run()
	startup := filepath.Join(programData(), "Microsoft", "Windows", "Start Menu", "Programs", "StartUp", "AgaevaId.cmd")
	_ = os.Remove(startup)
}

func trim(b []byte) string {
	s := string(b)
	if len(s) > 400 {
		return s[:400]
	}
	return s
}

func sameFile(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	return err1 == nil && err2 == nil && stringsEqualFold(aa, bb)
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// AlreadyRunning reports whether another copy holds the user-session mutex.
func AlreadyRunning() bool {
	name, _ := syscall.UTF16PtrFromString(`Local\AgaevaId`)
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("CreateMutexW")
	handle, _, callErr := proc.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return false
	}
	if callErr == syscall.Errno(183) { // ERROR_ALREADY_EXISTS
		return true
	}
	return false
}
