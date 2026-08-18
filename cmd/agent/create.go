package agent

import (
	"context"
	"encoding/json"
	"net/http"
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

// operatorIdTypeOpenId is the operator_id_type value for OpenId-typed operator_id.
const operatorIdTypeOpenId = 2

// createSubAgentData is the response body of /v1/cli/create-sub-agent and /v1/cli/gen-sub-agent-token.
type createSubAgentData struct {
	AgentId         string `json:"agent_id,omitempty"`
	AgentAk         string `json:"agent_ak,omitempty"`
	AgentRk         string `json:"agent_rk,omitempty"`
	AgentAkExpTime  int64  `json:"agent_ak_exp_time,omitempty"`
	AgentRkExpTime  int64  `json:"agent_rk_exp_time,omitempty"`
	AgentCreateTime int64  `json:"agent_create_time,omitempty"`
}

// CreateOptions holds the agent create options.
type CreateOptions struct {
	tmeet *internal.Tmeet
}

// newCreateCmd is the agent create command.
func newCreateCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &CreateOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an agent (sub-account) under the active main account",
		Long:  "Create an agent (sub-account) under the active main account and persist its credentials locally",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	return cmd
}

func (o *CreateOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	existing, err := config.GetAgentAccountConfig()
	if err != nil {
		log.Errorf(ctx, "read local agent config failed: %v", err)
		return exception.GetUserConfigUnknownError.With("read local agent config failed: %v", err)
	}
	if existing != nil && existing.AgentOpenId != "" {
		output.PrintInfof(cmd, "agent already exists:")
		output.PrintInfof(cmd, "  AgentId:    %s", existing.AgentOpenId)
		if existing.CreateTime > 0 {
			output.PrintInfof(cmd, "  CreateTime: %s", utils.TimeStampToISO8601(existing.CreateTime))
		}
		return exception.AgentAlreadyExistsError
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": operatorIdTypeOpenId,
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodPost, o.tmeet, &thttp.Request{
		ApiURI: "/v1/cli/create-sub-agent",
		Body:   body,
	})
	if err != nil {
		log.Errorf(ctx, "create sub agent failed: %v", err)
		return err
	}

	data := &createSubAgentData{}
	if err = json.Unmarshal([]byte(rsp.Data), data); err != nil {
		log.Errorf(ctx, "decode create sub agent response failed: %v", err)
		return exception.CreateAgentFailedError.With("decode create sub agent response failed: %v", err)
	}
	if data.AgentId == "" {
		return exception.CreateAgentFailedError.With("create sub agent failed: empty agent_id in response")
	}

	agentCfg := &config.AgentAccountConfig{
		AgentOpenId:         data.AgentId,
		MasterOpenId:        o.tmeet.UserConfig.OpenId,
		SdkId:               o.tmeet.UserConfig.SdkId,
		AccessToken:         data.AgentAk,
		RefreshToken:        data.AgentRk,
		Expires:             data.AgentAkExpTime,
		RefreshTokenExpires: data.AgentRkExpTime,
		CreateTime:          data.AgentCreateTime,
	}

	if err = retry.Do(ctx, func(ctx context.Context) error {
		if saveErr := config.SaveAgentAccountConfig(agentCfg); saveErr != nil {
			log.Errorf(ctx, "save agent config failed: %v", saveErr)
			return exception.CreateAgentFailedError.With("save agent config failed: %v", saveErr)
		}
		return nil
	}, retry.DefaultOptions); err != nil {
		return err
	}

	output.PrintInfof(cmd, "Agent created successfully")
	output.PrintInfof(cmd, "  AgentId:    %s", agentCfg.AgentOpenId)
	output.PrintInfof(cmd, "  CreateTime: %s", utils.TimeStampToISO8601(agentCfg.CreateTime))
	return nil
}
