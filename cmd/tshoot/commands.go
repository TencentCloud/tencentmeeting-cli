package tshoot

import (
	"encoding/json"
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	"tmeet/internal/output"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagInfo describes the minimal set of information for a single command flag.
type flagInfo struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Usage string `json:"usage"`
}

// commandInfo describes the minimal set of information for a single public command.
type commandInfo struct {
	Path  string     `json:"path"`
	Short string     `json:"short"`
	Flags []flagInfo `json:"flags"`
}

// commandsData is the `data` payload of `tshoot commands`.
// It wraps the command list in an object so that output.FormatPrint can be
// reused (FormatPrint requires `data` to be a JSON object rather than an array).
type commandsData struct {
	Version     string        `json:"version"`
	GlobalFlags []flagInfo    `json:"global_flags"`
	Commands    []commandInfo `json:"commands"`
}

// newCommandsCmd is the internal command `tmeet tshoot commands`.
// It dumps the path / short description / flags of every public leaf command
// exposed by the CLI. Intended for capability discovery by upstream agents and
// for documentation generation.
// The command reads pure local metadata: it does not hit the network and does
// not require login, hence Hidden + skipPreCheck.
func newCommandsCmd(tmeet *internal.Tmeet) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "commands",
		Short:  "[internal] dump all public commands with their short description and flags",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()
			items := make([]commandInfo, 0, 64)
			collectCommands(root, &items)

			payload := commandsData{
				Version:     root.Version,
				GlobalFlags: collectGlobalFlags(root),
				Commands:    items,
			}
			b, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			// Pure local metadata: no traceId / message.
			output.FormatPrint(cmd, "", "", string(b))
			return nil
		},
	}
	cmdutil.InjectSkipPreCheckAnnotation(cmd)
	return cmd
}

// collectGlobalFlags collects persistent flags declared on the root command
// (e.g. --format, --compact). These are inherited by every subcommand at runtime
// but are intentionally excluded from each command's `flags` field to avoid
// N-times duplication; they are surfaced once here as a top-level field so that
// upstream agents still know they exist.
// The auto-injected --help and any Hidden flag are skipped.
func collectGlobalFlags(root *cobra.Command) []flagInfo {
	globals := make([]flagInfo, 0)
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		globals = append(globals, flagInfo{
			Name:  f.Name,
			Type:  f.Value.Type(),
			Usage: f.Usage,
		})
	})
	return globals
}

// collectCommands walks the command tree depth-first and collects only "leaf commands".
// A leaf command is defined as: Runnable AND has no sub-commands.
// It skips the auto-injected help / completion branches and every Hidden command
// (which includes this command itself, so it does not expose itself).
func collectCommands(cmd *cobra.Command, out *[]commandInfo) {
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		if c.Runnable() && !c.HasSubCommands() {
			*out = append(*out, buildCommandInfo(c))
		}
		collectCommands(c, out)
	}
}

// buildCommandInfo builds the output info for a single command.
//   - `path` uses cobra.Command.CommandPath(), e.g. "tmeet auth login", matching
//     the wording used in docs / SKILL.
//   - `flags` uses LocalFlags only, which naturally filters out persistent flags
//     inherited from the root command (--format / --compact, etc.).
//   - For each flag, the auto-injected --help and any user-marked Hidden flag are skipped.
func buildCommandInfo(c *cobra.Command) commandInfo {
	info := commandInfo{
		Path:  c.CommandPath(),
		Short: c.Short,
		Flags: []flagInfo{}, // ensure JSON serializes as [] instead of null
	}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		info.Flags = append(info.Flags, flagInfo{
			Name:  f.Name,
			Type:  f.Value.Type(),
			Usage: f.Usage,
		})
	})
	return info
}
