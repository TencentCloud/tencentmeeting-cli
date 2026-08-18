package agent

import (
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// GetOptions holds the agent get options.
type GetOptions struct {
	tmeet   *internal.Tmeet
	agentId string
}

// newGetCmd is the agent get command.
func newGetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &GetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get details of a specific agent (sub-account)",
		Long:  "Get details of a specific agent (sub-account) by reading local configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.agentId, "agent-id", "", "Agent ID")
	_ = cmd.MarkFlagRequired("agent-id")

	return cmd
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	existing, err := config.GetAgentAccountConfig()
	if err != nil {
		log.Errorf(ctx, "read local agent config failed: %v", err)
		return exception.GetUserConfigUnknownError.With("read local agent config failed: %v", err)
	}
	if existing == nil || existing.AgentOpenId == "" {
		return exception.AgentNotFoundError
	}

	if o.agentId != existing.AgentOpenId {
		return exception.AgentNotFoundError
	}

	output.PrintInfof(cmd, "Agent 详情：")
	output.PrintInfof(cmd, "")
	output.PrintInfof(cmd, "  AgentId: %s", existing.AgentOpenId)
	if existing.CreateTime > 0 {
		output.PrintInfof(cmd, "  CreateTime:          %s", utils.TimeStampToISO8601(existing.CreateTime))
	}
	if existing.Expires > 0 {
		output.PrintInfof(cmd, "  AccessTokenExpires:  %s", utils.TimeStampToISO8601(existing.Expires))
	}
	if existing.RefreshTokenExpires > 0 {
		output.PrintInfof(cmd, "  RefreshTokenExpires: %s", utils.TimeStampToISO8601(existing.RefreshTokenExpires))
	}

	return nil
}
