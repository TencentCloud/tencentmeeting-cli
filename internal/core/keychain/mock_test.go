package keychain

import "testing"

func TestMockKeychainRoundtrip(t *testing.T) {
	mock := NewMockKeychain()

	if err := mock.Set("svc", "acc", "hello"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	got, err := mock.Get("svc", "acc")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("Get() = %q, want %q", got, "hello")
	}

	if err := mock.Remove("svc", "acc"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	_, err = mock.Get("svc", "acc")
	if err != ErrNotFound {
		t.Fatalf("Get() after Remove() error = %v, want ErrNotFound", err)
	}
}

func TestMockKeychainNotFound(t *testing.T) {
	mock := NewMockKeychain()

	_, err := mock.Get("svc", "nonexistent")
	if err != ErrNotFound {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}

	err = mock.Remove("svc", "nonexistent")
	if err != ErrNotFound {
		t.Fatalf("Remove() error = %v, want ErrNotFound", err)
	}
}

func TestMockKeychainEmptyAccount(t *testing.T) {
	mock := NewMockKeychain()

	err := mock.Set("svc", "", "data")
	if err == nil {
		t.Fatal("Set() should fail with empty account")
	}
}
