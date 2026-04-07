//go:build windows

package filecheck

// ValidateBeforeRead on Windows uses registry storage and does not involve .enc path reads; no Lstat/uid check needed (SEC-010).
func ValidateBeforeRead(_ string) error {
	return nil
}
