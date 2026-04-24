package cmd

import (
	"fmt"
	"strings"
	"tmeet/cmd/auth"
	"tmeet/cmd/meeting"
	"tmeet/cmd/record"
	"tmeet/cmd/report"
	"tmeet/cmd/tshoot"
	"tmeet/internal"
	"tmeet/internal/common"
	"tmeet/internal/config"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/output"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"
)

// Version 由构建时通过 ldflags 注入，例如：
// go build -ldflags "-X tmeet/cmd.Version=v1.0.0" .
var Version = "dev"

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() int {
	tmeet, err := internal.NewTmeet()
	if err != nil {
		output.PrintErrorf(nil, fmt.Sprintf("failed to initialize Tmeet: %v", err))
		return 1
	}

	// Initialise the file logging system. Logs are written to ~/.tmeet/logs/.
	// Errors here are non-fatal; the CLI continues without file logging.
	if logErr := log.Init(config.GetConfigDir(), log.LevelInfo); logErr == nil {
		defer log.Close()
	}

	rootCmd := &cobra.Command{
		Use:     common.CLI,
		Short:   "tmeet CLI",
		Long:    `tmeet CLI — OAuth authorization, API calls`,
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cobra.OnFinalize(func() {
				commandOperationLog(cmd, args)
			})
			return preCheck(cmd, args)
		},
		SilenceUsage: true,
	}
	rootCmd.SetContext(tmeet.TCtx)
	tmeet.CLIVersion = Version
	// Supports -V uppercase short flag for version query.
	rootCmd.Flags().BoolP("version", "V", false, "version for tmeet")
	// All subcommands inherit the --format flag, supporting json (default, compact) | json-pretty (indented).
	rootCmd.PersistentFlags().String("format", "json", "output format: json(compact)|json-pretty(indented)")

	// Add subcommand: auth
	rootCmd.AddCommand(auth.NewBaseCmd(tmeet))
	// Add subcommand: meeting
	rootCmd.AddCommand(meeting.NewBaseCmd(tmeet))
	// Add subcommand: report
	rootCmd.AddCommand(report.NewBaseCmd(tmeet))
	// Add subcommand: record
	rootCmd.AddCommand(record.NewBaseCmd(tmeet))
	// Add subcommand: tshoot
	rootCmd.AddCommand(tshoot.NewBaseCmd(tmeet))
	err = rootCmd.Execute()
	if err != nil {
		return 1
	}
	return 0
}

func preCheck(cmd *cobra.Command, args []string) error {
	if cmd.Annotations["skipPreCheck"] == "true" {
		return nil
	}
	if preCheckFlag := cmd.Annotations["skipPreCheckFlag"]; preCheckFlag != "" {
		flagValue, _ := cmd.Flags().GetBool(preCheckFlag) // ignore this err, it can downgrade
		if !flagValue {
			// flag is false, skip preCheck
			return nil
		}
	}

	// Validate local user information.
	usr, err := config.GetUserConfig()
	if err != nil {
		return err
	}
	if usr == nil {
		return exception.GetUserConfigEmptyError
	}
	return nil
}

func commandOperationLog(cmd *cobra.Command, args []string) {
	cmdStr := cmd.CommandPath() + " " + strings.Join(args, " ")
	cmd.Flags().Visit(func(f *pflag.Flag) {
		cmdStr += " --" + f.Name + " " + f.Value.String()
	})
	log.Infof(cmd.Context(), "command: %s", cmdStr)
}
