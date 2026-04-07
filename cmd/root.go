package cmd

import (
	"fmt"
	"tmeet/cmd/auth"
	"tmeet/cmd/meeting"
	"tmeet/cmd/record"
	"tmeet/cmd/report"
	"tmeet/internal"
	"tmeet/internal/common"
	"tmeet/internal/config"
	"tmeet/internal/exception"
	"tmeet/internal/log"

	"github.com/spf13/cobra"
)

// Version 由构建时通过 ldflags 注入，例如：
// go build -ldflags "-X tmeet/cmd.Version=v1.0.0" .
var Version = "dev"

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() int {
	tmeet, err := internal.NewTmeet()
	if err != nil {
		log.Errorf(nil, fmt.Sprintf("failed to initialize Tmeet: %v", err))
		return 1
	}

	rootCmd := &cobra.Command{
		Use:               common.CLI,
		Short:             "tmeet CLI",
		Long:              `tmeet CLI — OAuth authorization, API calls`,
		Version:           Version,
		PersistentPreRunE: preCheck,
		SilenceUsage:      true,
	}
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
