//go:build darwin

package browser

import "os/exec"

// execCommand is an indirect reference to exec.Command, allowing it to be replaced with a mock in unit tests.
var execCommand = exec.Command

// Open opens the specified URL using the system default browser.
func Open(url string) error {
	return execCommand("open", url).Start()
}
