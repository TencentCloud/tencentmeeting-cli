package auth

import (
	"context"
	"time"

	"tmeet/internal/config"
	"tmeet/internal/core/filelock"
	"tmeet/internal/exception"
	"tmeet/internal/log"
)

// AgentTokenData is the refreshed agent token pair returned by AgentTokenFetcher.
//
// Field tags align with the server-side pb (RefreshSubAgentTokenRsp).
type AgentTokenData struct {
	AgentId        string `json:"agent_id,omitempty"`
	AgentAk        string `json:"agent_ak,omitempty"`
	AgentRk        string `json:"agent_rk,omitempty"`
	AgentAkExpTime int64  `json:"agent_ak_exp_time,omitempty"`
	AgentRkExpTime int64  `json:"agent_rk_exp_time,omitempty"`
}

// AgentTokenFetcher fetches a new agent token pair using the given agent refresh_token.
//
// The actual HTTP transport (including master-account authentication header injection)
// is provided by the caller (typically rest-proxy) to avoid an auth -> rest-proxy
// import cycle.
type AgentTokenFetcher func(ctx context.Context, agentRk string) (*AgentTokenData, error)

// RefreshAgentToken refreshes the active agent's access_token if needed.
//
// Strategy mirrors RefreshToken for the master account, with two differences:
//   - When both ak and rk have expired, return AgentTokenExpiredError instead of
//     clearing local credentials. The user can recover via 'tmeet agent token'
//     (which calls GenSubAgentToken with master-account identity).
//   - Uses a dedicated agent_token.lock so that master-account refresh and
//     agent refresh do not block each other.
func (w *TmeetAuth) RefreshAgentToken(ctx context.Context, fetcher AgentTokenFetcher) error {
	if fetcher == nil {
		return exception.InvalidArgsError.With("agent token fetcher is nil")
	}

	agentCfg, err := config.GetAgentAccountConfig()
	if err != nil {
		return exception.GetUserConfigUnknownError.With("read local agent config failed: %v", err)
	}
	if agentCfg == nil || agentCfg.AgentOpenId == "" {
		return exception.AgentNotFoundError
	}

	now := time.Now().Unix()
	if agentCfg.Expires > now {
		return nil
	}
	if agentCfg.RefreshTokenExpires <= now {
		return exception.AgentTokenExpiredError
	}

	lockPath := config.GetAgentTokenLockPath()
	return filelock.WithLock(lockPath, func() error {
		// Lock contention above; re-read the agent config here (double-check).
		config.ResetCache()
		latest, _ := config.GetAgentAccountConfig()
		if latest != nil && latest.Expires > now {
			return nil
		}
		if latest == nil || latest.AgentOpenId == "" {
			return exception.AgentNotFoundError
		}
		if latest.RefreshTokenExpires <= now {
			return exception.AgentTokenExpiredError
		}

		tokenData, err := fetcher(ctx, latest.RefreshToken)
		if err != nil {
			log.Errorf(ctx, "refresh agent token failed: %v", err)
			return exception.RefreshAgentTokenFailedError.With("refresh agent token failed: %v", err)
		}
		if tokenData == nil || tokenData.AgentAk == "" || tokenData.AgentRk == "" {
			return exception.RefreshAgentTokenFailedError.With(
				"refresh agent token failed: incomplete token in response")
		}
		if tokenData.AgentId != "" && tokenData.AgentId != latest.AgentOpenId {
			return exception.RefreshAgentTokenFailedError.With(
				"refresh agent token failed: agent_id mismatch")
		}

		// Preserve identity / ownership / create_time, only rotate the token pair.
		latest.AccessToken = tokenData.AgentAk
		latest.RefreshToken = tokenData.AgentRk
		latest.Expires = tokenData.AgentAkExpTime
		latest.RefreshTokenExpires = tokenData.AgentRkExpTime

		return config.SaveAgentAccountConfig(latest)
	})
}
