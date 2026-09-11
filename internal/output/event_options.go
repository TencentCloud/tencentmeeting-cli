// event_options.go — Option framework for the EventPrint family.
//
// Mirrors the optionsMsg / Option pattern used by FormatPrint (see options.go),
// but carries fields that make sense for the *raw JSON* output channel
// rather than the {trace_id, message, data} envelope.
//
// Why a separate type instead of reusing optionsMsg?
//   - optionsMsg's `data string` field is intentionally typed as a serialised
//     JSON string so envelope-mode Options (WithCompact, WithConvert,
//     WithContactSearchLogic) can run regex / map operations on it.  EventPrint
//     hands in `interface{}` and serialises *after* Options run, so the two
//     models can't share state.
//   - Keeping the types distinct also keeps the two output contracts grep-able
//     from outside the package: anything touching eventOptionsMsg is bare-JSON
//     (the `tmeet event` family); anything touching optionsMsg is envelope.
//
// No options are shipped today.  The framework is in place so future event
// commands can request behaviours like "force pretty output regardless of
// --format" or "skip the trailing newline" without breaking the public
// signature of EventPrint.  Add concrete EventOption values to this file
// when a real use case appears.

package output

import "github.com/spf13/cobra"

// eventOptionsMsg is the mutable state EventOption values may tweak.
//
// Fields are unexported because callers configure them through With* helpers
// (none yet — see file header) rather than struct literals; the EventPrint
// runtime reads them after applying every option in declaration order.
type eventOptionsMsg struct {
	cmd *cobra.Command
}

// EventOption is the functional-options type for EventPrint.
//
// Following the same shape as Option (see options.go) so the two systems
// look familiar side-by-side, even though they don't share state.
type EventOption func(msg *eventOptionsMsg)

// applyEventOptions runs every EventOption against msg in order.  Centralised
// so EventPrint stays readable and so future changes (e.g. a default option
// list) have one place to land.
func applyEventOptions(msg *eventOptionsMsg, opts ...EventOption) {
	for _, o := range opts {
		o(msg)
	}
}
