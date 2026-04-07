// Package filelock provides a file-based cross-process exclusive lock for atomic write scenarios (SEC-012).
package filelock

import "time"

// defaultTimeout is the maximum wait time for acquiring an exclusive lock.
const defaultTimeout = 5 * time.Second
