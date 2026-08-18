package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"
	"tmeet/internal/utils/retry"

	"github.com/spf13/cobra"
)

// TokenOptions holds the agent token options.
type TokenOptions struct {
	tmeet *internal.Tmeet
}

// newTokenCmd is the agent token command.
func newTokenCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &TokenOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Issue a fresh token pair for the agent under the active main account",
		Long:  "Issue a fresh access_token and refresh_token pair for the agent under the active main account, and persist them locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	return cmd
}

func (o *TokenOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	existing, err := config.GetAgentAccountConfig()
	if err != nil {
		log.Errorf(ctx, "read local agent config failed: %v", err)
		return exception.GetUserConfigUnknownError.With("read local agent config failed: %v", err)
	}
	if existing == nil || existing.AgentOpenId == "" {
		return exception.AgentNotFoundError
	}

	// 检查 refresh_token 是否仍在有效期内，未过期则无需刷新
	if existing.RefreshTokenExpires > 0 {
		remaining := time.Until(time.Unix(existing.RefreshTokenExpires, 0))
		if remaining > 0 {
			output.PrintInfof(cmd, "Agent 的 refresh_token 未过期，无需刷新")
			output.PrintInfof(cmd, "  AgentId:             %s", existing.AgentOpenId)
			output.PrintInfof(cmd, "  RefreshTokenExpires: %s", utils.TimeStampToISO8601(existing.RefreshTokenExpires))
			output.PrintInfof(cmd, "  剩余有效期:          %s", formatDuration(remaining))
			return nil
		}
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": operatorIdTypeOpenId,
		"agent_id":         existing.AgentOpenId,
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodPost, o.tmeet, &thttp.Request{
		ApiURI: "/v1/cli/gen-sub-agent-token",
		Body:   body,
	})
	if err != nil {
		log.Errorf(ctx, "gen sub agent token failed: %v", err)
		return err
	}

	data := &createSubAgentData{}
	if err = json.Unmarshal([]byte(rsp.Data), data); err != nil {
		log.Errorf(ctx, "decode gen sub agent token response failed: %v", err)
		return exception.RefreshAgentTokenFailedError.With("decode gen sub agent token response failed: %v", err)
	}
	if data.AgentId == "" || data.AgentAk == "" || data.AgentRk == "" {
		return exception.RefreshAgentTokenFailedError.With("gen sub agent token failed: incomplete token in response")
	}
	if data.AgentId != existing.AgentOpenId {
		return exception.RefreshAgentTokenFailedError.With("gen sub agent token failed: agent_id mismatch")
	}

	agentCfg := &config.AgentAccountConfig{
		AgentOpenId:         existing.AgentOpenId,
		MasterOpenId:        existing.MasterOpenId,
		SdkId:               existing.SdkId,
		AccessToken:         data.AgentAk,
		RefreshToken:        data.AgentRk,
		Expires:             data.AgentAkExpTime,
		RefreshTokenExpires: data.AgentRkExpTime,
		CreateTime:          existing.CreateTime,
	}

	if err = retry.Do(ctx, func(ctx context.Context) error {
		if saveErr := config.SaveAgentAccountConfig(agentCfg); saveErr != nil {
			log.Errorf(ctx, "save agent config failed: %v", saveErr)
			return exception.RefreshAgentTokenFailedError.With("save agent config failed: %v", saveErr)
		}
		return nil
	}, retry.DefaultOptions); err != nil {
		return err
	}

	output.PrintInfof(cmd, "Agent token refreshed successfully")
	output.PrintInfof(cmd, "  AgentId:             %s", agentCfg.AgentOpenId)
	output.PrintInfof(cmd, "  RefreshTokenExpires: %s", utils.TimeStampToISO8601(agentCfg.RefreshTokenExpires))
	return nil
}

// formatDuration 将 time.Duration 格式化为人类可读的字符串。
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}
