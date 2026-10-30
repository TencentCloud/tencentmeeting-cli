package update

import (
	"encoding/json"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/notice"
	"tmeet/internal/output"

	"github.com/spf13/cobra"
)

// CheckOptions holds the flags parsed by `tmeet update check`.
type CheckOptions struct {
	tmeet *internal.Tmeet
}

// newCheckCmd prints the current update notice by querying the server.
//
// Notice is attached to the output as the `data` payload itself (via a
// normal FormatPrint call), AND the automatic `_notice.update` injection
// is suppressed via WithoutUpdateNotice so the Agent does not see the
// same information twice.
func newCheckCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &CheckOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "check",
		Short: "check for an available tmeet CLI update",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdUpdateCheck)),
		),
	}
	return cmd
}

// Run executes `tmeet update check`.
func (o *CheckOptions) Run(cmd *cobra.Command, args []string) error {
	openId := o.tmeet.UserConfig.OpenId

	// No per-call timeout wrapper — the underlying REST client already
	// enforces its own timeout policy, and nesting a shorter one here
	// would just mask legitimately-slow but successful fetches. Check
	// is an explicit query, so we always talk to the server.
	payload, err := notice.FetchFromAPI(cmd.Context(), o.tmeet)
	if err != nil {
		return err
	}

	pref := notice.LoadPreference(openId)
	info := notice.AssembleForCheck(o.tmeet.CLIVersion, payload, pref)

	// info is almost always non-nil: AssembleForCheck encodes the "no
	// upgrade available" verdict via info.Available rather than by
	// returning nil, so the caller sees a stable shape (current + target
	// + available + preference) in every normal outcome, including
	// "already on the latest" and "blacklisted". A nil info only happens
	// in the degenerate case of a missing cached payload (shouldn't
	// occur here — we just fetched it a few lines above).
	data, _ := json.Marshal(info)
	output.FormatPrint(cmd, "", "success", string(data), output.WithoutUpdateNotice())
	return nil
}
