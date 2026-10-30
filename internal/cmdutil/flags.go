package cmdutil

import (
	"tmeet/internal/exception"

	"github.com/spf13/cobra"
)

// rejectFlags enforces a single FlagRule's Reject list against cmd.
//
// It is the shared implementation behind ActionRegistry.Dispatch's handling
// of ActionHandler.FlagRules; handlers do not call it directly, they
// declare FlagRules and let Dispatch do the enforcement.
//
// A flag is considered "explicitly set" when cobra reports Flag.Changed =
// true, so callers can safely list flags that carry a non-zero default
// value (e.g. --page-size defaults to 30) without triggering false
// positives.
//
// Unknown flag names in flagNames are silently ignored, so FlagRule.Reject
// lists stay stable even when the command's flag set evolves.
//
// The error message format depends on form:
//   - form == "" : "--<flag> is not supported by --action <action>"
//     (used by unconditional rules that apply to the whole --action).
//   - form != "" : "--<flag> is not supported by --action <action> (form: <form>)"
//     (used by conditional / sub-form rules).
func rejectFlags(cmd *cobra.Command, action, form string, flagNames []string) error {
	for _, name := range flagNames {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			continue
		}
		if !f.Changed {
			continue
		}
		if form == "" {
			return exception.InvalidArgsError.With(
				"--%s is not supported by --action %s", name, action)
		}
		return exception.InvalidArgsError.With(
			"--%s is not supported by --action %s (form: %s)", name, action, form)
	}
	return nil
}
