package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/core"
	"tmeet/internal/core/filelock"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
)

const authorizationTimeout = 5 * time.Minute
const loopPollTime = 5 * time.Second
const authorizationLocation = "%s://%s/marketplace/tencentmeeting-cli-auth.html?code=%s"

type TmeetAuth struct {
	tmeet *internal.Tmeet
}

func NewTmeetAuth(tmeet *internal.Tmeet) *TmeetAuth {
	return &TmeetAuth{
		tmeet: tmeet,
	}
}

func (w *TmeetAuth) GeneralOauth2Address(ctx context.Context) (string, string, error) {
	authCode, err := w.getDeviceAuthCode(ctx)
	if err != nil {
		return "", "", err
	}

	return fmt.Sprintf(authorizationLocation, thttp.DefaultHttpsProtocol, core.GetAuthEndpoint(), authCode), authCode, nil
}

func (w *TmeetAuth) PollingOauth2Result(ctx context.Context, authCode string) (*AuthTokenData, error) {
	timeout := time.After(authorizationTimeout)
	loopTime := time.NewTicker(loopPollTime)

	for {
		select {
		case <-timeout:
			return nil, exception.AuthorizationTimeoutError
		case <-loopTime.C:
			authTokenData, err := w.getAuthToken(ctx, authCode)
			if err != nil {
				return nil, err
			}
			if authTokenData == nil || authTokenData.OpenID == "" {
				// wait next loop
				continue
			}
			return authTokenData, nil
		case <-ctx.Done():
			return nil, exception.AuthorizationTimeoutError
		}
	}
}

// RefreshToken refreshes the token.
func (w *TmeetAuth) RefreshToken(ctx context.Context) error {
	now := time.Now().Unix()
	expires := w.tmeet.UserConfig.Expires
	refreshTokenExpires := w.tmeet.UserConfig.RefreshTokenExpires
	if expires > now {
		return nil
	}
	if refreshTokenExpires <= now {
		// refresh token expired, delete local credentials
		if err := config.ClearUserConfig(); err != nil {
			return err
		}
		return exception.UserIdentityExpiredError
	}

	// Refreshing the token requires a lock.
	lockPath := config.GetTokenLockPath()
	return filelock.WithLock(lockPath, func() error {
		// Lock contention above; re-read the file config here.
		config.ResetCache()
		userConfig, _ := config.GetUserConfig()
		if userConfig != nil && userConfig.Expires > now {
			// Another process refreshed successfully, return directly.
			w.tmeet.UserConfig = userConfig
			return nil
		}

		// Refresh the token.
		tokenData, err := w.refreshAuthToken(ctx)
		if err != nil {
			// Delete local config.
			_ = config.ClearUserConfig()
			return exception.RefreshTokenFailedError
		}

		w.tmeet.UserConfig = &config.UserConfig{}
		w.tmeet.UserConfig.SdkId = tokenData.SdkId
		w.tmeet.UserConfig.OpenId = tokenData.OpenID
		w.tmeet.UserConfig.AccessToken = tokenData.AccessToken
		w.tmeet.UserConfig.RefreshToken = tokenData.RefreshToken
		w.tmeet.UserConfig.Expires, _ = strconv.ParseInt(tokenData.AccessTokenExpireTime, 10, 64)
		w.tmeet.UserConfig.RefreshTokenExpires, _ = strconv.ParseInt(tokenData.RefreshTokenExpireTime, 10, 64)

		return config.SaveUserConfig(w.tmeet.UserConfig)
	})
}
