// Package filecheck performs security checks before reading sensitive local files
// (Unix: Lstat rejects symlinks and validates file UID).
// On Windows, the common scenario is registry storage, and path validation is a no-op.
package filecheck
