package update

import (
	"encoding/json"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/notice"
	"tmeet/internal/output"

	"github.com/spf13/cobra"
)

// SetOptions holds options for `tmeet update set`.
type SetOptions struct {
	tmeet      *internal.Tmeet
	Preference string // --preference: remind | never_remind
}

// newSetCmd persists the user's update preference locally AND reports it
// to the server.
//
// Semantics:
//   - `--preference remind`       asks the server to drop the user from the
//     "no remind" telemetry allowlist, and the
//     local file flips to remind.
//   - `--preference never_remind` adds the user to that allowlist via the
//     standard telemetry key, and the local
//     file flips to never_remind.
//
// The local write always succeeds first; the network report is best-effort.
// Rationale: the local file is the authority the CLI uses on next launch;
// a transient upload failure would otherwise leave the user with a surprise
// prompt the next time they run a command. Logging the error is enough.
func newSetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set the update preference (remind | never_remind)",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdUpdateSet)),
		),
	}
	cmd.Flags().StringVar(&opts.Preference, "preference", "",
		"update preference, one of: remind | never_remind (required)")
	_ = cmd.MarkFlagRequired("preference")
	return cmd
}

// Run executes `tmeet update set`.
func (o *SetOptions) Run(cmd *cobra.Command, args []string) error {
	switch o.Preference {
	case notice.PreferenceRemind, notice.PreferenceNeverRemind:
		// ok
	default:
		return exception.InvalidArgsError.With(
			"--preference must be one of: %s | %s (got %q)",
			notice.PreferenceRemind, notice.PreferenceNeverRemind, o.Preference)
	}

	openId := o.tmeet.UserConfig.OpenId
	if err := notice.SavePreference(openId, o.Preference); err != nil {
		return exception.UpdatePreferenceSaveError.With("failed to save update preference locally: %v", err)
	}

	// Best-effort upload. We DELIBERATELY do not propagate this error to
	// the user — the local write already succeeded, and the Agent would
	// otherwise retry the whole `tmeet update set` cycle (writing the
	// same preference again) on every transient network blip.
	reportErr := notice.ReportPreference(cmd.Context(), o.tmeet, o.Preference)
	if reportErr != nil {
		log.Errorf(cmd.Context(), "update preference report failed: %v", reportErr)
	}

	resp := map[string]interface{}{
		"preference": o.Preference,
	}
	data, _ := json.Marshal(resp)
	output.FormatPrint(cmd, "", "success", string(data), output.WithoutUpdateNotice())
	return nil
}
