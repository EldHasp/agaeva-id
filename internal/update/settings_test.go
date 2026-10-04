package update

import (
	"path/filepath"
	"testing"
)

func TestUpdateSwitchDefaultsOff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update.json")
	if Enabled(path) {
		t.Fatal("missing switch should stay off")
	}
	if err := EnsureDisabled(path); err != nil {
		t.Fatal(err)
	}
	if Enabled(path) {
		t.Fatal("first install turned updates on")
	}
	if err := SetEnabled(path, true); err != nil {
		t.Fatal(err)
	}
	if !Enabled(path) {
		t.Fatal("expected updates on")
	}
	if err := EnsureDisabled(path); err != nil {
		t.Fatal(err)
	}
	if !Enabled(path) {
		t.Fatal("repeat install must keep an explicit on switch")
	}
}
