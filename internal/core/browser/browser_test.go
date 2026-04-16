package browser

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// fakeExecCommand replaces exec.Command with a helper subprocess that calls the current test binary,
// allowing command arguments to be verified without actually opening a browser.
func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// TestHelperProcess is the subprocess entry point launched by fakeExecCommand.
// It only executes when the environment variable GO_WANT_HELPER_PROCESS=1 is set, simulating a successful command exit.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

// TestOpen_ValidURL verifies that Open does not return an error for a valid URL.
// Since the actual command differs per platform, this test is only meaningful when the corresponding system command exists.
func TestOpen_ValidURL(t *testing.T) {
	url := "https://example.com"

	switch runtime.GOOS {
	case "darwin":
		// macOS: open command is always available
		if _, err := exec.LookPath("open"); err != nil {
			t.Skip("open command not found, skipping")
		}
	case "windows":
		// Windows: rundll32 is always available
	default:
		// Linux: xdg-open may not exist (e.g. in CI environments)
		if _, err := exec.LookPath("xdg-open"); err != nil {
			t.Skip("xdg-open not found, skipping")
		}
	}

	// Replace the real command with fakeExecCommand to avoid actually opening a browser
	origCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = origCommand }()

	if err := Open(url); err != nil {
		t.Errorf("Open(%q) returned unexpected error: %v", url, err)
	}
}

// TestOpen_EmptyURL verifies the behavior of Open with an empty string URL (the command just needs to start successfully).
func TestOpen_EmptyURL(t *testing.T) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("open"); err != nil {
			t.Skip("open command not found, skipping")
		}
	case "windows":
		// rundll32 is always available
	default:
		if _, err := exec.LookPath("xdg-open"); err != nil {
			t.Skip("xdg-open not found, skipping")
		}
	}

	origCommand := execCommand
	execCommand = fakeExecCommand
	defer func() { execCommand = origCommand }()

	// An empty URL should not cause a panic; the test passes as long as the command can start
	_ = Open("")
}
