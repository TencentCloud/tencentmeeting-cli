package auth

import (
	"context"
	"strconv"
	"time"
	"tmeet/internal"
	"tmeet/internal/auth"
	"tmeet/internal/config"
	"tmeet/internal/core/browser"
	"tmeet/internal/core/filelock"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/utils/retry"

	"github.com/spf13/cobra"
)

// LoginOptions holds the login options.
type LoginOptions struct {
	tmeet     *internal.Tmeet
	NoBrowser bool // login without browser, true to disable auto-open browser
}

// newLoginCmd is the login command.
func newLoginCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &LoginOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "login",
		Short: "login to tmeet",
		Long:  "Login to tmeet by setting up configuration and credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			lockPath := config.GetTokenLockPath()
			return filelock.WithLock(lockPath, func() error {
				return opts.Run(cmd, args)
			})
		},
	}
	cmd.Annotations = map[string]string{"skipPreCheck": "true"}

	cmd.Flags().BoolVar(&opts.NoBrowser, "no-browser", false, "disable auto-opening the browser, only print the authorization URL")

	return cmd
}

func (o *LoginOptions) Run(cmd *cobra.Command, args []string) error {
	// Lock contention above; re-read the file config here.
	config.ResetCache()
	userConfig, _ := config.GetUserConfig()
	if userConfig != nil &&
		userConfig.RefreshTokenExpires > time.Now().Unix() {
		// Already logged in && refreshToken not expired, return directly.
		return exception.UserHasBeenInitializedError
	}
	// Get authorization URL.
	tmeetAuth := auth.NewTmeetAuth(o.tmeet)
	authorizationLocation, authCode, err := tmeetAuth.GeneralOauth2Address(cmd.Context())
	if err != nil {
		return err
	}
	log.Infof(cmd, "")
	log.Infof(cmd, "Please open the following URL in your browser to authorize")
	log.Infof(cmd, "")
	log.Infof(cmd, "authorize url: %s", authorizationLocation)
	log.Infof(cmd, "")
	log.Infof(cmd, "waiting for authorization...")
	log.Infof(cmd, "")

	if !o.NoBrowser {
		// Try to open the default browser automatically
		_ = browser.Open(authorizationLocation)
	}

	// Poll for authorization result.
	authTokenData, err := tmeetAuth.PollingOauth2Result(cmd.Context(), authCode)
	if err != nil {
		return err
	}

	userConfig = &config.UserConfig{
		SdkId:        authTokenData.SdkId,
		OpenId:       authTokenData.OpenID,
		AccessToken:  authTokenData.AccessToken,
		RefreshToken: authTokenData.RefreshToken,
	}
	userConfig.Expires, _ = strconv.ParseInt(authTokenData.AccessTokenExpireTime, 10, 64)
	userConfig.RefreshTokenExpires, _ = strconv.ParseInt(authTokenData.RefreshTokenExpireTime, 10, 64)
	err = retry.Do(cmd.Context(), func(ctx context.Context) error {
		err = config.SaveUserConfig(userConfig)
		if err != nil {
			return exception.AuthorizationFailedError
		}
		return nil
	}, retry.DefaultOptions)

	if err != nil {
		return err
	}

	log.Infof(cmd, "Login successful. Start managing your meetings using tmeet.")
	return nil
}
