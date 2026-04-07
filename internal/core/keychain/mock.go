// mock.go provides an in-memory KeychainAccess mock implementation for testing only.
//
// MockKeychain does not involve any system calls, file I/O, or encryption operations.
// All data is stored in plaintext in an in-memory map for easy unit test verification of business logic.
//
// Usage:
//
//	mock := keychain.NewMockKeychain()
//	config.SetKeychain(mock)
//	// ... run tests ...
package keychain

import "fmt"

// MockKeychain is an in-memory KeychainAccess implementation for testing only.
// All data is stored in an in-memory map and is lost when the process exits.
type MockKeychain struct {
	store map[string]string
}

// NewMockKeychain creates a new MockKeychain instance.
func NewMockKeychain() *MockKeychain {
	return &MockKeychain{
		store: make(map[string]string),
	}
}

func mockKey(service, account string) string {
	return service + "/" + account
}

// Get reads the specified entry from memory; returns ErrNotFound if not found.
func (m *MockKeychain) Get(service, account string) (string, error) {
	key := mockKey(service, account)
	data, ok := m.store[key]
	if !ok {
		return "", ErrNotFound
	}
	return data, nil
}

// Set writes data to the in-memory map; overwrites if already exists.
func (m *MockKeychain) Set(service, account, data string) error {
	if account == "" {
		return fmt.Errorf("account cannot be empty")
	}
	m.store[mockKey(service, account)] = data
	return nil
}

// Remove deletes the specified entry from memory; returns ErrNotFound if not found.
func (m *MockKeychain) Remove(service, account string) error {
	key := mockKey(service, account)
	if _, ok := m.store[key]; !ok {
		return ErrNotFound
	}
	delete(m.store, key)
	return nil
}
