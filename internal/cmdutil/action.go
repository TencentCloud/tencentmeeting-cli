package cmdutil

import (
	"sort"
	"strings"

	"tmeet/internal/exception"

	"github.com/spf13/cobra"
)

// ActionHandler declares one branch of an --action-dispatched command.
//
// A command whose behavior is selected by an --action flag (or any similarly
// named flag) can be decomposed into N handlers, one per action value. The
// handler carries everything that is specific to that branch:
//
//   - which ApiCmd the invocation maps to (for compact-schema mounting);
//   - which of the command's union of flags must NOT appear for this action
//     (both action-level permanent rejections and form-level conditional
//     rejections, declared uniformly via FlagRules);
//   - branch-specific mutual-exclusion / requirement checks;
//   - the actual request-building and response-rendering logic.
//
// The framework (ActionRegistry.Dispatch) drives handlers so each handler
// stays free of dispatch / error-message boilerplate.
type ActionHandler struct {
	// ApiCmd is the ApiCmd this branch maps to.
	//
	// Use ApiCmd for branches with a single fixed ApiCmd. For branches whose
	// ApiCmd further depends on flag values (e.g. minutes flipping between
	// list-by-url and single-file variants based on --share-url), set
	// ResolveApiCmd instead. When both are set, ResolveApiCmd wins.
	ApiCmd        string
	ResolveApiCmd func(cmd *cobra.Command) string

	// FlagRules declares the flag-rejection contract of this action in a
	// single table that unifies both action-level and form-level rejections:
	//
	//   - Unconditional rule (When == nil): the Reject list applies to the
	//     whole --action and is always enforced. At most one unconditional
	//     rule is allowed per handler; multiple ones should be merged into
	//     one. The error message is "--<flag> is not supported by --action
	//     <action>" (no form suffix).
	//
	//   - Conditional rule (When != nil): the Reject list applies only when
	//     When(cmd) returns true. All conditional rules' When conditions
	//     MUST be mutually exclusive; at most one conditional rule may be
	//     matched per Dispatch. The error message is "--<flag> is not
	//     supported by --action <action> (form: <Form>)" where Form names
	//     the matched sub-form (e.g. "minutes(file)", "transcript(url)").
	//
	// Dispatch enforces both arities (>= 2 unconditional / >= 2 conditional
	// matches) as programmer errors and returns an internal error, so bugs
	// in the rule table surface at test time instead of silently masking
	// unrelated flags.
	//
	// FlagRules is optional; a nil / empty table means "no flag rejections"
	// (callers still get the required/exclusivity checks of Validate).
	FlagRules []FlagRule

	// Validate runs branch-specific requirement / mutual-exclusion checks
	// (e.g. "requires one of --record-file-id or --share-url"). Any error
	// returned here is surfaced verbatim to the user; typically the error
	// should be wrapped by exception.InvalidArgsError.With.
	//
	// Validate is optional; when nil, the framework skips validation.
	Validate func(cmd *cobra.Command) error

	// Run executes the branch: build the request, call the transport, render
	// the response. Run is required; a nil Run at Dispatch time is a
	// programmer error and will surface as an InvalidArgsError.
	Run func(cmd *cobra.Command) error
}

// FlagRule declares "under some business condition, reject these flags" for
// an --action branch. See ActionHandler.FlagRules for the full semantics.
//
// When is a pure function with no side effects; it typically inspects the
// handler's own options struct (or cmd.Flags()) to decide whether the rule
// applies. Set When to nil to express an unconditional rule that always
// applies to the whole --action (i.e. an action-level permanent rejection).
//
// Form is a human-readable sub-form label used only to build the "(form:
// <Form>)" suffix of the error message when a conditional rule is violated.
// Keep Form in sync with the sub-form name documented in --help and the
// skill reference so that users can jump from the error back to the form
// matrix (e.g. "minutes(file)", "transcript(details)").
//
// Reject lists the flag names forbidden under this rule. Unknown flag names
// are silently ignored by the underlying rejection helper, so callers may
// keep the list stable even when the command's flag set evolves.
type FlagRule struct {
	When   func(cmd *cobra.Command) bool
	Form   string
	Reject []string
}

// ActionRegistry maps action values (e.g. "minutes") to their handlers.
//
// Registries are typically built once per cobra command construction and
// consumed by a single Dispatch call at RunE time. Keys should match the raw
// string values users pass through the --action flag.
type ActionRegistry map[string]*ActionHandler

// Dispatch is the unified entry point for --action-dispatched commands.
//
// The wiring is: parse the action value from the given flag, look up the
// matching handler, then run FlagRules check -> Validate -> Run in that
// order. Errors from any stage are returned verbatim so cobra can print
// them and exit non-zero.
//
// FlagRules execution splits into two passes:
//
//  1. All unconditional rules (When == nil) have their Reject lists
//     enforced. At most one unconditional rule is allowed; declaring two
//     is a programmer error and surfaces as an internal error.
//
//  2. All conditional rules (When != nil) are evaluated. At most one may
//     match (their conditions must be mutually exclusive); matching two or
//     more is a programmer error and surfaces as an internal error. If
//     exactly one matches, its Reject list is enforced.
//
// action is the raw value parsed from the --<actionFlag> flag; actionFlag is
// only used to build the "invalid --<flag>" error message when the value is
// unknown.
func (r ActionRegistry) Dispatch(cmd *cobra.Command, action, actionFlag string) error {
	h, ok := r[action]
	if !ok {
		return exception.InvalidArgsError.With(
			"invalid --%s %q, expected one of: %s",
			actionFlag, action, strings.Join(r.sortedKeys(), ", "))
	}
	if h == nil || h.Run == nil {
		return exception.InvalidArgsError.With(
			"internal: --%s %q has no Run handler registered", actionFlag, action)
	}
	if err := applyFlagRules(cmd, action, h.FlagRules); err != nil {
		return err
	}
	if h.Validate != nil {
		if err := h.Validate(cmd); err != nil {
			return err
		}
	}
	return h.Run(cmd)
}

// applyFlagRules enforces a handler's FlagRules against cmd. See
// ActionHandler.FlagRules for the semantics; this helper centralizes the
// two-pass execution and the arity checks that catch rule-table bugs.
func applyFlagRules(cmd *cobra.Command, action string, rules []FlagRule) error {
	if len(rules) == 0 {
		return nil
	}

	// Pass 1: unconditional rules. At most one allowed.
	var unconditional *FlagRule
	for i := range rules {
		if rules[i].When != nil {
			continue
		}
		if unconditional != nil {
			return exception.InvalidArgsError.With(
				"internal: --action %s has multiple unconditional FlagRules; merge them into one",
				action)
		}
		unconditional = &rules[i]
	}
	if unconditional != nil {
		if err := rejectFlags(cmd, action, unconditional.Form, unconditional.Reject); err != nil {
			return err
		}
	}

	// Pass 2: conditional rules. At most one may match (mutual exclusion).
	var matched *FlagRule
	for i := range rules {
		if rules[i].When == nil {
			continue
		}
		if !rules[i].When(cmd) {
			continue
		}
		if matched != nil {
			return exception.InvalidArgsError.With(
				"internal: --action %s has multiple matching conditional FlagRules "+
					"(forms: %q and %q); FlagRule.When conditions must be mutually exclusive",
				action, matched.Form, rules[i].Form)
		}
		matched = &rules[i]
	}
	if matched != nil {
		if err := rejectFlags(cmd, action, matched.Form, matched.Reject); err != nil {
			return err
		}
	}
	return nil
}

// BuildApiCmdResolver returns an ApiCmdResolver that, at middleware time,
// re-reads the current value of --<actionFlag> and delegates to the matching
// handler's ApiCmd / ResolveApiCmd. Feed the returned resolver directly into
// middleware.WithApiCmd so compact-schema and annotation flow stay wired.
//
// When --<actionFlag> is empty or unknown, the resolver returns "" so the
// downstream middleware chain proceeds transparently, matching the behavior
// documented on cmdutil.ApiCmdResolver.
func (r ActionRegistry) BuildApiCmdResolver(actionFlag string) ApiCmdResolver {
	return ResolverFunc(func(cmd *cobra.Command) string {
		action, err := cmd.Flags().GetString(actionFlag)
		if err != nil || action == "" {
			return ""
		}
		h, ok := r[action]
		if !ok || h == nil {
			return ""
		}
		if h.ResolveApiCmd != nil {
			return h.ResolveApiCmd(cmd)
		}
		return h.ApiCmd
	})
}

// sortedKeys returns the action names in stable lexical order, so the "expected
// one of" hint stays deterministic across runs (map iteration is randomized).
func (r ActionRegistry) sortedKeys() []string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
