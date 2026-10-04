package update

import "testing"

func TestNewerRelease(t *testing.T) {
	if !newer("agaeva-id-v0.2.0", "0.1.0") {
		t.Fatal("expected newer")
	}
	if newer("v0.1.0", "0.1.0") {
		t.Fatal("same version")
	}
	if newer("v0.1.0", "0.1.1") {
		t.Fatal("older")
	}
}
