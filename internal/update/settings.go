package update

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SettingsPath is the switch next to the installed program.
// Missing file means updates stay off, which is the development default.
func SettingsPath() string {
	if pd := os.Getenv("ProgramData"); pd != "" {
		return filepath.Join(pd, "AgaevaId", "update.json")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "AgaevaId", "update.json")
}

// Enabled reports whether this computer should replace itself from GitHub releases.
func Enabled(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var doc struct {
		Enabled bool `json:"enabled"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return false
	}
	return doc.Enabled
}

// SetEnabled writes the switch. The first install leaves it off.
func SetEnabled(path string, on bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := json.MarshalIndent(map[string]bool{"enabled": on}, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}

// EnsureDisabled creates the switch as off when the computer has no setting yet.
func EnsureDisabled(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return SetEnabled(path, false)
}
