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

// ListOptions holds the agent list options.
type ListOptions struct {
	tmeet *internal.Tmeet
}

// newListCmd is the agent list command.
func newListCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ListOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List agents (sub-accounts) under the active main account",
		Long:  "List agents (sub-accounts) under the active main account by reading local configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	return cmd
}

func (o *ListOptions) Run(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	existing, err := config.GetAgentAccountConfig()
	if err != nil {
		log.Errorf(ctx, "read local agent config failed: %v", err)
		return exception.GetUserConfigUnknownError.With("read local agent config failed: %v", err)
	}
	if existing == nil || existing.AgentOpenId == "" {
		output.PrintInfof(cmd, "当前主账号下没有子账号（agent），请通过 `tmeet agent create` 创建")
		return nil
	}

	output.PrintInfof(cmd, "当前主账号下的 agent 列表（共 1 个）：")
	output.PrintInfof(cmd, "")
	output.PrintInfof(cmd, "  [1] AgentId: %s", existing.AgentOpenId)
	if existing.CreateTime > 0 {
		output.PrintInfof(cmd, "      CreateTime:          %s", utils.TimeStampToISO8601(existing.CreateTime))
	}
	if existing.Expires > 0 {
		output.PrintInfof(cmd, "      AccessTokenExpires:  %s", utils.TimeStampToISO8601(existing.Expires))
	}
	if existing.RefreshTokenExpires > 0 {
		output.PrintInfof(cmd, "      RefreshTokenExpires: %s", utils.TimeStampToISO8601(existing.RefreshTokenExpires))
	}

	return nil
}
