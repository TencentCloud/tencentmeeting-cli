package middleware

import (
	"tmeet/internal/cmdutil"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

// WithApiCmd resolves the ApiCmd for this invocation (possibly choosing one
// out of several candidates based on flags) and writes it to both:
//   - the command's annotation, readable via cmdutil.GetApiCmdAnnotation(cmd);
//   - cmd.Context(), readable via restProxy.GetApiCmdFromContext(ctx).
//
// When resolver is nil or resolves to an empty string, nothing is written
// and the chain proceeds transparently.
func WithApiCmd(resolver cmdutil.ApiCmdResolver) CmdMiddleware {
	return func(next RunEFunc) RunEFunc {
		return func(cmd *cobra.Command, args []string) error {
			if resolver != nil {
				if name := resolver.Resolve(cmd); name != "" {
					cmdutil.InjectApiCmdAnnotation(cmd, name)
					cmd.SetContext(restProxy.InjectApiCmdContext(cmd.Context(), name))
				}
			}
			return next(cmd, args)
		}
	}
}
