package app

import (
	"net/http"
	"strings"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

const (
	// maxSdkNameLen limits the sdk-name field to 20 characters in width
	// (ASCII counts as 1, non-ASCII such as Chinese counts as 2).
	maxSdkNameLen = 20

	// maxHomepageLen limits the homepage field to 200 characters.
	maxHomepageLen = 200
)

// SetOptions holds the options for setting app info.
type SetOptions struct {
	tmeet       *internal.Tmeet
	Homepage    string // App homepage URL
	LayoutStyle string // In-meeting open layout: narrow | wide
	SdkName     string // App SDK name
}

// newSetCmd sets app info.
func newSetCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SetOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set your current cli-app info",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdAppInfoSet)),
		),
	}

	// 填充参数
	cmd.Flags().StringVar(&opts.Homepage, "homepage", "", "app homepage URL (must use http or https, and the address must have a trusted SSL/TLS certificate; up to 200 characters)")
	cmd.Flags().Var(&cmdutil.EnumValue{Value: &opts.LayoutStyle, Allowed: []string{"sidebar", "wide_sidebar", "popout"}},
		"layout-style", "in-meeting open layout: sidebar (narrow sidebar default) | wide_sidebar (wide sidebar) | popout (standalone popout window)")
	cmd.Flags().StringVar(&opts.SdkName, "sdk-name", "", "app sdk name (up to 20 in width; ASCII counts as 1, non-ASCII such as Chinese counts as 2)")

	// mark required flags
	cmd.MarkFlagsOneRequired("homepage", "layout-style", "sdk-name")

	return cmd
}

func (o *SetOptions) Run(cmd *cobra.Command, args []string) error {
	params := map[string]interface{}{
		"operator_id":      o.tmeet.UserConfig.OpenId,
		"operator_id_type": 2, // openId
	}

	// Optional fields: only set when user provides them.
	if cmd.Flags().Changed("homepage") {
		if o.Homepage != "" {
			// only set https/http://xxx and check url valid
			if err := utils.ValidateURL(o.Homepage); err != nil {
				return err
			}
			if err := utils.CharacterLimit("homepage", o.Homepage, maxHomepageLen); err != nil {
				return err
			}
		}
		params["home_page"] = o.Homepage
	}
	if cmd.Flags().Changed("layout-style") {
		params["layout_style"] = o.LayoutStyle
	}
	if cmd.Flags().Changed("sdk-name") {
		if o.SdkName == "" {
			return exception.InvalidArgsError.With("sdk-name cannot be empty")
		}
		if strings.TrimSpace(o.SdkName) == "" {
			return exception.InvalidArgsError.With("sdk-name cannot contain only invisible characters")
		}
		if err := utils.CharacterWidthLimit("sdk-name", o.SdkName, maxSdkNameLen); err != nil {
			return err
		}
		params["sdk_name"] = o.SdkName
	}

	req := &thttp.Request{
		ApiURI: "/v1/cli/app-info",
		Body:   params,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodPut, o.tmeet, req)
	if err != nil {
		return err
	}

	output.FormatPrint(cmd, rsp.TraceId, rsp.Message, rsp.Data)
	return nil
}
