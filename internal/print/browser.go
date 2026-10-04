package print

import (
	"os"
	"path/filepath"
	"runtime"
)

// FindBrowser returns the first installed Chrome, Edge, or Yandex Browser.
func FindBrowser() (path string, name string, err error) {
	for _, candidate := range candidates() {
		if fileExists(candidate.path) {
			return candidate.path, candidate.name, nil
		}
	}
	return "", "", errString("не найден Chrome, Edge или Яндекс.Браузер")
}

type browserCandidate struct {
	path string
	name string
}

func candidates() []browserCandidate {
	if runtime.GOOS != "windows" {
		return []browserCandidate{
			{path: "/usr/bin/google-chrome", name: "Chrome"},
			{path: "/usr/bin/google-chrome-stable", name: "Chrome"},
			{path: "/usr/bin/chromium", name: "Chrome"},
			{path: "/usr/bin/chromium-browser", name: "Chrome"},
			{path: "/usr/bin/microsoft-edge", name: "Edge"},
		}
	}
	local := os.Getenv("LOCALAPPDATA")
	prog := os.Getenv("ProgramFiles")
	prog86 := os.Getenv("ProgramFiles(x86)")
	var list []browserCandidate
	add := func(name, path string) {
		if path != "" {
			list = append(list, browserCandidate{path: path, name: name})
		}
	}
	add("Chrome", filepath.Join(prog, "Google", "Chrome", "Application", "chrome.exe"))
	add("Chrome", filepath.Join(prog86, "Google", "Chrome", "Application", "chrome.exe"))
	add("Chrome", filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"))
	add("Edge", filepath.Join(prog, "Microsoft", "Edge", "Application", "msedge.exe"))
	add("Edge", filepath.Join(prog86, "Microsoft", "Edge", "Application", "msedge.exe"))
	add("Яндекс.Браузер", filepath.Join(local, "Yandex", "YandexBrowser", "Application", "browser.exe"))
	add("Яндекс.Браузер", filepath.Join(prog, "Yandex", "YandexBrowser", "Application", "browser.exe"))
	add("Яндекс.Браузер", filepath.Join(prog86, "Yandex", "YandexBrowser", "Application", "browser.exe"))
	return list
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
