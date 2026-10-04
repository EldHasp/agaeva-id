//go:build windows

package install

import (
	"log"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

// RunningAsService reports whether the Service Control Manager started this process.
func RunningAsService() bool {
	ok, err := svc.IsWindowsService()
	return err == nil && ok
}

// RunService keeps the machine-wide service alive and starts the helper
// in the interactive console session. Printing stays in that session.
func RunService(logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}
	return svc.Run(serviceName, &wakeHandler{log: logger})
}

type wakeHandler struct {
	log *log.Logger
}

func (h *wakeHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: accepted}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	h.wake()
	for {
		select {
		case <-ticker.C:
			h.wake()
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				stopWorkers()
				return false, 0
			}
		}
	}
}

func (h *wakeHandler) wake() {
	session := windows.WTSGetActiveConsoleSessionId()
	if session == 0xFFFFFFFF || session == 0 {
		return
	}
	exe := InstalledPath()
	inSession, others := findWorkers(exe, session)
	for _, pid := range others {
		terminate(pid)
	}
	if inSession {
		return
	}
	if err := startInSession(exe, session); err != nil {
		h.log.Printf("не удалось разбудить помощника в сессии %d: %s", session, err.Error())
		return
	}
	h.log.Printf("помощник запущен в сессии %d", session)
}

func findWorkers(exe string, keepSession uint32) (inSession bool, others []uint32) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, nil
	}
	defer windows.CloseHandle(snap)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snap, &entry); err != nil {
		return false, nil
	}
	want := filepath.Base(exe)
	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if stringsEqualFold(name, want) {
			var sid uint32
			if err := windows.ProcessIdToSessionId(entry.ProcessID, &sid); err == nil && sid != 0 {
				if sid == keepSession {
					inSession = true
				} else {
					others = append(others, entry.ProcessID)
				}
			}
		}
		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}
	return inSession, others
}

func terminate(pid uint32) {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.TerminateProcess(h, 0)
}

func stopWorkers() {
	_, others := findWorkers(InstalledPath(), 0)
	for _, pid := range others {
		terminate(pid)
	}
}

func startInSession(exe string, session uint32) error {
	var user windows.Token
	if err := windows.WTSQueryUserToken(session, &user); err != nil {
		return err
	}
	defer user.Close()

	var primary windows.Token
	if err := windows.DuplicateTokenEx(user, windows.TOKEN_ALL_ACCESS, nil, windows.SecurityImpersonation, windows.TokenPrimary, &primary); err != nil {
		return err
	}
	defer primary.Close()

	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, primary, false); err != nil {
		return err
	}
	defer windows.DestroyEnvironmentBlock(env)

	app, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	desktop, err := windows.UTF16PtrFromString(`winsta0\default`)
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	si := windows.StartupInfo{
		Cb:      uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Desktop: desktop,
	}
	var pi windows.ProcessInformation
	err = windows.CreateProcessAsUser(primary, app, nil, nil, nil, false, windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NO_WINDOW, env, dir, &si, &pi)
	if err != nil {
		return err
	}
	_ = windows.CloseHandle(pi.Process)
	_ = windows.CloseHandle(pi.Thread)
	return nil
}
