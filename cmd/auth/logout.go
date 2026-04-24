package auth

import (
	"context"
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"
	"tmeet/internal/utils/retry"

	"github.com/spf13/cobra"
)

// LogoutOptions holds the logout options.
type LogoutOptions struct {
	tmeet *internal.Tmeet
}

// newLogoutCmd is the logout command.
func newLogoutCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &LogoutOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "logout from tmeet",
		Long:  "Logout and clear local authentication credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	return cmd
}

func (o *LogoutOptions) Run(cmd *cobra.Command, args []string) error {
	err := retry.Do(cmd.Context(), func(ctx context.Context) error {
		return config.ClearUserConfig()
	}, retry.DefaultOptions)
	if err != nil {
		log.Errorf(cmd.Context(), "Logout failed: %v", err)
		return exception.LogoutFailedError
	}
	output.PrintInfof(cmd, "Logout success, welcome to use it again.")
	return nil
}
