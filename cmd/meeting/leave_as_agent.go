package meeting

import (
	"net/http"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// LeaveAsAgentOptions holds the options for the `meeting leave-as-agent` command.
//
// The command lets the agent (sub-account) leave the specified meeting under
// the agent identity. Whether the agent is actually in the meeting is
// determined by the server; the command simply forwards the request.
type LeaveAsAgentOptions struct {
	tmeet     *internal.Tmeet
	MeetingID string // Meeting ID to leave
	AgentId   string // Agent ID (must match locally stored agent config)
}

// newLeaveAsAgentCmd builds the `meeting leave-as-agent` command.
func newLeaveAsAgentCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &LeaveAsAgentOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "leave-as-agent",
		Short: "Let the agent leave the specified meeting",
		Long:  "Let the agent (sub-account) leave the specified meeting under the agent identity.",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingLeaveAsAgent)),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().StringVar(&opts.AgentId, "agent-id", "", "agent ID (required)")
	_ = cmd.MarkFlagRequired("meeting-id")
	_ = cmd.MarkFlagRequired("agent-id")

	return cmd
}

// Run executes the leave-as-agent command: agent leaves the given meeting.
//
// Output strategy: the response payload is reduced to its only useful field
// (meeting_id) so the caller can confirm which meeting was acted on.
func (o *LeaveAsAgentOptions) Run(cmd *cobra.Command, args []string) error {
	if o.MeetingID == "" {
		return exception.InvalidArgsError.With("--meeting-id is required")
	}

	// Validate --agent-id against the locally stored agent configuration.
	agentCfg, err := config.GetAgentAccountConfig()
	if err != nil {
		return err
	}
	if agentCfg == nil || agentCfg.AgentOpenId != o.AgentId {
		return exception.AgentNotFoundError
	}

	// operator_id / operator_id_type still carry the master-account OpenId; the
	// agent identity is conveyed by RequestProxyWithAgent via the
	// Tmeet-Agent-ID / Tmeet-Agent-AK headers.
	//
	// instanceid / robot_user_type are kept aligned with the join call (see
	// join_as_agent.go) for symmetry. Agent identity itself is matched via the agent
	// headers, not via these two fields.
	body := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": agentOperatorIdTypeOpenId,
		"instanceid":       agentJoinInstanceId,
		"robot_user_type":  agentJoinRobotUserType,
		"leave_type":       0,
		"reason_code":      1,
	}
	req := &thttp.Request{
		ApiURI: "/v1/meetings/{meeting_id}/leave",
		Body:   body,
		PathParams: thttp.PathParams{
			"meeting_id": o.MeetingID,
		},
	}

	rsp, err := restProxy.RequestProxyWithAgent(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	// Reduce the response payload to its only useful field (meeting_id) so
	// the caller can confirm which meeting was acted on.
	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, buildAgentMeetingIdOnlyData(rsp.Data))
	return nil
}
