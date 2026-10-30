package update

import (
	"encoding/json"
	"tmeet/internal"
	"tmeet/internal/notice"
	"tmeet/internal/output"

	"github.com/spf13/cobra"
)

// GetOptions holds options for `tmeet update get`.
type GetOptions struct {
	tmeet *internal.Tmeet
}

// newGetCmd shows the user's locally saved update preference.
//
// Scope is intentionally narrow — a single value — so Agents have a
// cheap way to answer "did the user already opt out of update prompts?"
// without any extra wrangling of a bigger response payload. The authoritative
// server-side view (and the cached version-info bundle) is reached via
// `tmeet update check`.
func newGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &GetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "show the local update preference",
		RunE:  opts.Run,
	}
	return cmd
}

// Run executes `tmeet update get`.
func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	pref := notice.LoadPreference(o.tmeet.UserConfig.OpenId)

	// Narrow, deterministic shape. `preference` is empty-string when the
	// user has never explicitly set a value; callers rely on that to
	// render a "default" state rather than guessing.
	resp := map[string]interface{}{
		"preference": pref.Value,
		"updated_at": pref.UpdatedAt,
	}
	data, _ := json.Marshal(resp)
	// WithoutUpdateNotice because the data payload IS the update
	// preference — adding a wrapped `_notice.update` on top would
	// duplicate it.
	output.FormatPrint(cmd, "", "success", string(data), output.WithoutUpdateNotice())
	return nil
}
