package cmd

import (
	"fmt"
	"strings"
	"tmeet/cmd/auth"
	"tmeet/cmd/contact"
	"tmeet/cmd/control"
	"tmeet/cmd/meeting"
	"tmeet/cmd/record"
	"tmeet/cmd/report"
	"tmeet/cmd/tshoot"
	"tmeet/internal"
	"tmeet/internal/common"
	"tmeet/internal/config"
	"tmeet/internal/crash"
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
func Execute() (exitCode int) {
	tmeet, err := internal.NewTmeet()
	if err != nil {
		output.PrintErrorf(nil, fmt.Sprintf("failed to initialize Tmeet: %v", err))
		return 1
	}

	// Initialise the file logging system FIRST so its defer is registered
	// before the panic-safety defer below. Under LIFO defer ordering that
	// means log.Close() runs LAST, i.e. after RecoverAndReport has finished
	// using the logger (both directly and transitively via the REST proxy).
	// Writing to an already-closed logger would panic and be silently
	// swallowed by the reporter's inner recover(), so the ordering matters.
	//
	// Errors from log.Init are non-fatal; the CLI continues without file
	// logging in that case.
	if logErr := log.Init(config.GetConfigDir(), log.LevelInfo); logErr == nil {
		defer log.Close()
	}

	// Global panic safety net: if any subcommand (or cobra itself) panics,
	// capture the stack, report it to the server, print "tmeet crashed: xxx"
	// and propagate a dedicated exit code (crash.PanicExitCode) to main via
	// the named return value `exitCode`.
	//
	// NOTE: recover() MUST be called directly inside this deferred closure.
	// If we moved it into RecoverAndReport, Go's runtime would consider it
	// an indirect call and recover() would return nil, letting the panic
	// keep propagating past the safety net. So the closure owns recover(),
	// and RecoverAndReport just takes the recovered value as an argument.
	//
	// IMPORTANT: RecoverAndReport must NOT call os.Exit itself -- doing so
	// would skip the earlier-declared `defer log.Close()` and leak the log
	// file handle / lose buffered log lines. Instead we thread the exit
	// code back through the return value so all sibling defers still run
	// during the normal function unwind.
	defer func() {
		if code, crashed := crash.RecoverAndReport(tmeet.TCtx, tmeet, recover()); crashed {
			exitCode = code
		}
	}()

	rootCmd := &cobra.Command{
		Use:     common.CLI,
		Short:   "tmeet CLI",
		Long:    `tmeet CLI — OAuth authorization, API calls`,
		Version: Version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			cobra.OnFinalize(func() {
				commandOperationLog(cmd, args)
			})
			injectCmdPathIntoTmeet(tmeet, cmd)
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
	// All subcommands inherit the --compact flag; when true,
	// the response packet results print only a few necessary fields.
	rootCmd.PersistentFlags().Bool("compact", false,
		"compact output: the response packet results print only a few necessary fields")

	// Add subcommand: auth
	rootCmd.AddCommand(auth.NewBaseCmd(tmeet))
	// Add subcommand: meeting
	rootCmd.AddCommand(meeting.NewBaseCmd(tmeet))
	// Add subcommand: contact
	rootCmd.AddCommand(contact.NewBaseCmd(tmeet))
	// Add subcommand: report
	rootCmd.AddCommand(report.NewBaseCmd(tmeet))
	// Add subcommand: record
	rootCmd.AddCommand(record.NewBaseCmd(tmeet))
	// Add subcommand: control
	rootCmd.AddCommand(control.NewBaseCmd(tmeet))
	// Add subcommand: tshoot
	rootCmd.AddCommand(tshoot.NewBaseCmd(tmeet))
	err = rootCmd.Execute()
	if err != nil {
		log.Errorf(rootCmd.Context(), "execute failed: %v", err)
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

func injectCmdPathIntoTmeet(tmeet *internal.Tmeet, cmd *cobra.Command) {
	tmeet.CmdPath = cmd.CommandPath()
}
