package auth

import (
	"fmt"
	"time"
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"

	"github.com/spf13/cobra"
)

// StatusOptions holds the status query options.
type StatusOptions struct {
	tmeet *internal.Tmeet
}

// newStatusCmd is the status query command.
func newStatusCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &StatusOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		Long:  "Display the current login status and credential information",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}
	cmd.Annotations = map[string]string{"skipPreCheck": "true"}

	return cmd
}

func (o *StatusOptions) Run(cmd *cobra.Command, args []string) error {
	userConfig, err := config.GetUserConfig()
	if err != nil {
		log.Errorf(cmd.Context(), "failed to read user config: %v", err)
		return exception.GetUserConfigUnknownError.With("failed to read user config: %v", err)
	}
	if userConfig == nil {
		output.PrintInfof(cmd, "Not logged in. Please use 'tmeet auth login' to authenticate.")
		return nil
	}

	now := time.Now().Unix()

	// Show login status.
	output.PrintInfof(cmd, "Logged in")
	output.PrintInfof(cmd, "  OpenId:  %s", userConfig.OpenId)

	// AccessToken expiry status.
	if userConfig.Expires > 0 {
		expiresTime := time.Unix(userConfig.Expires, 0)
		if now >= userConfig.Expires {
			output.PrintInfof(cmd, "  AccessToken:  expired (at %s)", expiresTime.Format(time.DateTime))
		} else {
			remaining := time.Duration(userConfig.Expires-now) * time.Second
			output.PrintInfof(cmd, "  AccessToken:  valid (expires at %s, remaining %s)",
				expiresTime.Format(time.DateTime), formatDuration(remaining))
		}
	}

	// RefreshToken expiry status.
	if userConfig.RefreshTokenExpires > 0 {
		refreshExpiresTime := time.Unix(userConfig.RefreshTokenExpires, 0)
		if now >= userConfig.RefreshTokenExpires {
			output.PrintInfof(cmd, "  RefreshToken: expired (at %s)", refreshExpiresTime.Format(time.DateTime))
		} else {
			remaining := time.Duration(userConfig.RefreshTokenExpires-now) * time.Second
			output.PrintInfof(cmd, "  RefreshToken: valid (expires at %s, remaining %s)",
				refreshExpiresTime.Format(time.DateTime), formatDuration(remaining))
		}
	}

	return nil
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
