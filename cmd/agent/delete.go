package agent

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
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

// DeleteOptions holds the agent delete options.
type DeleteOptions struct {
	tmeet   *internal.Tmeet
	agentId string
	force   bool
}

// newDeleteCmd is the agent delete command.
func newDeleteCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &DeleteOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the agent (sub-account) under the active main account",
		Long:  "Delete the agent (sub-account) under the active main account and clear its local credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.agentId, "agent-id", "", "Agent ID")
	cmd.Flags().BoolVar(&opts.force, "force", false, "跳过二次确认")
	_ = cmd.MarkFlagRequired("agent-id")

	return cmd
}

func (o *DeleteOptions) Run(cmd *cobra.Command, args []string) error {
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

	// 二次确认
	if !o.force {
		output.PrintInfof(cmd, "即将删除以下 agent：")
		output.PrintInfof(cmd, "  AgentId:    %s", existing.AgentOpenId)
		if existing.CreateTime > 0 {
			output.PrintInfof(cmd, "  CreateTime: %s", utils.TimeStampToISO8601(existing.CreateTime))
		}
		fmt.Print("\n确认删除？(yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "yes" && input != "y" {
			output.PrintInfof(cmd, "已取消删除操作")
			return nil
		}
	}

	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": operatorIdTypeOpenId,
		"agent_id":         existing.AgentOpenId,
	}
	if _, err = restProxy.RequestProxy(ctx, http.MethodPost, o.tmeet, &thttp.Request{
		ApiURI: "/v1/cli/delete-sub-agent",
		Body:   body,
	}); err != nil {
		log.Errorf(ctx, "delete sub agent failed: %v", err)
		return err
	}

	if err = retry.Do(ctx, func(ctx context.Context) error {
		if clearErr := config.ClearAgentAccountConfig(); clearErr != nil {
			log.Errorf(ctx, "clear agent config failed: %v", clearErr)
			return exception.DeleteAgentFailedError.With("clear agent config failed: %v", clearErr)
		}
		return nil
	}, retry.DefaultOptions); err != nil {
		return err
	}

	output.PrintInfof(cmd, "Agent deleted successfully")
	output.PrintInfof(cmd, "  AgentId:    %s", existing.AgentOpenId)
	output.PrintInfof(cmd, "  CreateTime: %s", utils.TimeStampToISO8601(existing.CreateTime))
	return nil
}
