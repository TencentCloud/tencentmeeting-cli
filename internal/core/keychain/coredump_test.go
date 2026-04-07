package keychain

import "testing"

func TestDisableCoreDumpNoPanic(t *testing.T) {
	if err := disableCoreDump(); err != nil {
		t.Fatalf("disableCoreDump() returned error: %v", err)
	}
}
