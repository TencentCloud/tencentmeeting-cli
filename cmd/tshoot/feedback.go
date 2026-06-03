package tshoot

import (
	"net/http"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// Character length limits for feedback fields.
const (
	maxIntentLen       = 200
	maxActionsTriedLen = 500
	maxResultLen       = 500
)

// FeedbackOptions is the options for feedback.
type FeedbackOptions struct {
	tmeet        *internal.Tmeet
	Category     string // Feedback category
	Intent       string // Original intent of the agent
	ActionsTried string // Actions the agent has tried
	Result       string // Result or blocker of the tried actions
	ToolName     string // Tool/command name used
	ErrorCode    string // Error code returned by the tool
}

// newFeedbackCmd reports troubleshooting feedback to the server.
func newFeedbackCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &FeedbackOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "feedback",
		Short: "report troubleshooting feedback to the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.Category, "category", "", `feedback category (required), options:
		tool_not_found: want to do something but cannot find a matching tool;
		tool_error: called a tool but it returned an error;
		tool_inadequate: tool exists but its capability/parameters are insufficient;
		unexpected_result: call succeeded but the result did not meet expectations;
		suggestion: general suggestion or improvement idea`,
	)
	cmd.Flags().StringVar(&opts.Intent, "intent", "", "original intent of the agent (required), max 200 characters")
	cmd.Flags().StringVar(&opts.ActionsTried, "actions-tried", "", "actions the agent has tried, max 500 characters")
	cmd.Flags().StringVar(&opts.Result, "result", "", "result or blocker of the tried actions, max 500 characters")
	cmd.Flags().StringVar(&opts.ToolName, "tool-name", "", "tool/command name used")
	cmd.Flags().StringVar(&opts.ErrorCode, "error-code", "", "error code returned by the tool")

	// mark required flags
	_ = cmd.MarkFlagRequired("category")
	_ = cmd.MarkFlagRequired("intent")

	return cmd
}

// Run executes the feedback command.
func (o *FeedbackOptions) Run(cmd *cobra.Command, args []string) error {
	// validate field length limits
	if err := utils.CharacterLimit("--intent", o.Intent, maxIntentLen); err != nil {
		return err
	}
	if err := utils.CharacterLimit("--actions-tried", o.ActionsTried, maxActionsTriedLen); err != nil {
		return err
	}
	if err := utils.CharacterLimit("--result", o.Result, maxResultLen); err != nil {
		return err
	}

	// feedback to the server
	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": "2", // OpenId
		"category":         o.Category,
		"intent":           o.Intent,
		"actions_tried":    o.ActionsTried,
		"result":           o.Result,
		"tool_name":        o.ToolName,
		"error_code":       o.ErrorCode,
		"from_source":      "CLI", // fixed
	}

	req := &thttp.Request{
		ApiURI: "/v1/api/feedback",
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPost, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
