package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

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
	cmdutil.InjectSkipPreCheckAnnotation(cmd)

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
	userName := ""
	// Show user info(if refresh token is not expired)
	if userConfig.RefreshTokenExpires > 0 && now < userConfig.RefreshTokenExpires {
		if userInfo, err := o.getUserInfo(cmd.Context()); err != nil {
			log.Warnf(cmd.Context(), "get user info failed: %v", err)
		} else if userInfo.UserName != "" {
			userName = userInfo.UserName
		}
		// Show token expiry status.
		lastestUserConfig, err1 := config.GetUserConfig()
		if err1 != nil {
			log.Errorf(cmd.Context(), "failed to read user config: %v", err1)
			return exception.GetUserConfigUnknownError.With("failed to read user config: %v", err1)
		}
		if lastestUserConfig == nil {
			output.PrintInfof(cmd, "  Session has expired. Please use 'tmeet auth login' to re-authenticate.")
			return nil
		}
		userConfig = lastestUserConfig
	}

	// Show login status.
	output.PrintInfof(cmd, "Logged in")
	output.PrintInfof(cmd, "  OpenId:  %s", userConfig.OpenId)
	if userName != "" {
		output.PrintInfof(cmd, "  UserName:  %s", userName)
	}
	now = time.Now().Unix()

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

// UserInfoData is the user info data.
type UserInfoData struct {
	UserName string `json:"user_name,omitempty"`
}

// getUserInfo get user info.
func (o *StatusOptions) getUserInfo(ctx context.Context) (*UserInfoData, error) {
	req := &thttp.Request{
		ApiURI: "/v1/cli/get-user-info",
		Body: map[string]interface{}{
			"operator_id":      o.tmeet.UserConfig.OpenId,
			"operator_id_type": 2, // openId
		},
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodPost, o.tmeet, req)
	if err != nil {
		log.Errorf(ctx, "request api /v1/cli/get-user-info failed, err: %v", err)
		return nil, err
	}
	data := &UserInfoData{}
	err = json.Unmarshal([]byte(rsp.Data), data)
	if err != nil {
		log.Errorf(ctx, "unmarshal api /v1/cli/get-user-info failed, err: %v", err)
		return nil, err
	}
	return data, nil
}
