package cmdutil

import (
	"tmeet/internal/exception"

	"github.com/spf13/pflag"
)

// SingleStringValue is a pflag.Value that wraps a *string but rejects being
// set more than once from the command line. It is intended for flags whose
// semantics require a single value (e.g. --action for --action-dispatched
// commands): pflag's default StringVar silently lets the last occurrence win
// when a user passes --foo a --foo b, which for single-value flags is a
// footgun rather than a feature.
//
// The zero value is not usable; construct instances via SingleStringVar.
type SingleStringValue struct {
	name    string  // flag name, only used to build the error message
	target  *string // underlying storage, shared with the owning options struct
	setOnce bool    // flips to true after the first successful Set
}

// String returns the current value of the underlying string. It is called by
// pflag when rendering defaults / help, so it must be safe to invoke before
// Set has ever run.
func (s *SingleStringValue) String() string {
	if s.target == nil {
		return ""
	}
	return *s.target
}

// Set records the value on first call and returns an InvalidArgsError on any
// subsequent call, so that passing --<name> twice fails command parsing
// instead of silently letting the later value win.
func (s *SingleStringValue) Set(value string) error {
	if s.setOnce {
		return exception.InvalidArgsError.With(
			"--%s can only be specified once", s.name)
	}
	*s.target = value
	s.setOnce = true
	return nil
}

// Type is the type name shown by cobra in --help output.
func (s *SingleStringValue) Type() string {
	return "string"
}

// SingleStringVar registers a string flag on fs that can only be specified
// once on the command line. It mirrors pflag.FlagSet.StringVar's signature
// (target / name / defaultVal / usage) so call sites can migrate by simply
// swapping the function name.
//
// The default value is written to *target eagerly (matching StringVar's
// behavior) and does not count as a "set", so users can still override it
// exactly once from the CLI.
func SingleStringVar(fs *pflag.FlagSet, target *string, name, defaultVal, usage string) {
	*target = defaultVal
	fs.Var(&SingleStringValue{name: name, target: target}, name, usage)
}

// ensure SingleStringValue implements pflag.Value at compile time
var _ pflag.Value = &SingleStringValue{}
