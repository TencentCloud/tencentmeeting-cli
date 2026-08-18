// exit_test_hook.go — test-only replacement for the os.Exit-based code
// path used by exitWithCodeAfterJSON.
//
// Production code calls os.Exit(code) which terminates the whole process
// — fine for the real `tmeet event status` / `event stop` invocation,
// fatal in `go test` (it would tear down the test binary mid-run).  We
// expose two helpers tests use to swap in a panic-based stub and recover
// the requested code:
//
//	t.Cleanup(SetExitFuncForTest(t))   // panics on exit, restores on cleanup
//	code, panicked := captureExitCode(func() { ... })
//
// The build tag `_test` is implicit because the file is named with the
// `_test_hook` suffix and only test files import its symbols; non-test
// builds simply don't reference SetExitFuncForTest, so the indirection
// in exit.go has no behavioural effect outside tests.

package event

// exitPanic is the sentinel a test-installed exitFunc panics with.  Tests
// use captureExitCode (defined inline in *_test.go) to recover and pull
// out the int.  Exported across files in the same package so tests can
// reference it directly.
type exitPanic struct{ Code int }

// SetExitFuncForTest installs a panicking exitFunc and returns a cleanup
// function that restores the previous value.  Pass the cleanup directly
// to t.Cleanup so test runs always restore the production behaviour
// regardless of pass/fail.
//
// Exported (capitalised) so siblings tests in package event can call it
// without exposing exitFunc itself.
func SetExitFuncForTest() func() {
	prev := exitFunc
	exitFunc = func(code int) { panic(exitPanic{Code: code}) }
	return func() { exitFunc = prev }
}
