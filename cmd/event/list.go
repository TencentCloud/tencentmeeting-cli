package event

import (
	"strings"

	"github.com/spf13/cobra"

	"tmeet/internal"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/exception"
	"tmeet/internal/output"
)

// ListOptions holds the options for `tmeet event list`.
type ListOptions struct {
	tmeet  *internal.Tmeet
	Domain string // only show EventKeys belonging to the given domain (e.g. meeting, record); empty = all
}

// newListCmd implements `tmeet event list [--domain <name>]`.
//
// Output contract:
//   - stdout: a JSON array of {key, domain, description}, sorted by (domain, key)
//   - stderr: one informational line pointing at `event schema <key>`
//   - exit codes: 0 always, except invalid --domain => 1 (with KnownDomains hint)
//
// Annotated as skipPreCheck so it works for users that have not yet logged in:
// the registry is built into the binary, no remote calls are made.
func newListCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List built-in EventKeys",
		Long: `List all EventKeys this tmeet build can subscribe to.

Output is a JSON array (one entry per EventKey) on stdout; an informational
hint is written to stderr.  Use --domain to narrow the list, and --format
json-pretty for an indented view.`,
		Annotations: map[string]string{"skipPreCheck": "true"},
		Args:        cobra.NoArgs,
		RunE:        opts.Run,
	}
	cmd.Flags().StringVar(&opts.Domain, "domain", "",
		"only show EventKeys belonging to the given domain (e.g. meeting, record)")
	return cmd
}

// Run executes `event list`.
func (o *ListOptions) Run(cmd *cobra.Command, args []string) error {
	if o.Domain != "" {
		known := eventruntime.KnownDomains()
		if !containsFold(known, o.Domain) {
			return exception.InvalidArgsError.With(
				"unknown domain %q (known: %s)", o.Domain, strings.Join(known, ", "))
		}
	}
	items := eventruntime.ListKeys(o.Domain)
	if items == nil {
		// Ensure stdout is "[]" instead of "null" when the registry is empty
		// or the filter excludes every key — friendlier for jq.
		items = []eventruntime.EventListItem{}
	}
	if err := output.EventPrint(cmd, items); err != nil {
		return err
	}
	output.EventStderr(cmd, "Use 'tmeet event schema <key>' for details.")
	return nil
}

// containsFold reports whether haystack contains needle, case-insensitive.
func containsFold(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.EqualFold(s, needle) {
			return true
		}
	}
	return false
}
