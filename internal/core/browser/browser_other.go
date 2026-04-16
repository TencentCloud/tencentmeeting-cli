//go:build !darwin && !windows

package browser

import (
	"errors"
	"os"
	"os/exec"
)

// execCommand is an indirect reference to exec.Command, allowing it to be replaced with a mock in unit tests.
var execCommand = exec.Command

// Open opens the specified URL using the system default browser.
// On Linux, a graphical environment (DISPLAY or WAYLAND_DISPLAY) must be available
// and xdg-open must exist; otherwise an error is returned.
func Open(url string) error {
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return errors.New("no graphical environment detected")
	}
	_, err := exec.LookPath("xdg-open")
	if err != nil {
		return errors.New("xdg-open not found, cannot open browser automatically")
	}
	return execCommand("xdg-open", url).Start()
}
