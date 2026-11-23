// Package flagalias installs parse-time aliases for cobra/pflag flags.
//
// An alias is another accepted spelling for one canonical flag. Unlike
// registering a second pflag, an alias never surfaces in --help, keeps the
// value/default/Changed state attached to the single canonical pflag, and is
// rewritten silently before pflag applies the value.
//
// It is designed for agent-friendly CLIs where callers occasionally guess the
// wrong flag spelling (e.g. --keyword instead of --query). Rather than failing
// and asking the agent to retry, Bind transparently accepts the alias.
//
// When both an alias and its canonical flag appear in the same invocation
// Bind reports the alias as an unknown flag, so the alias is indistinguishable
// from a typo and callers cannot rely on it as a stable second interface.
package flagalias

import (
	"os"
	"strings"
	"tmeet/internal/exception"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Spec binds a set of alias long-flag names to one canonical long-flag name.
// Names must not include the leading "--".
type Spec struct {
	Canonical string
	Aliases   []string
}

// argvFn resolves the raw argument tokens to inspect for conflict detection.
// It is overridable in tests so we do not have to mutate os.Args there.
var argvFn = func() []string { return os.Args }

// Bind installs the given alias specs on cmd.
//
// Behaviour:
//   - Alias occurrences are rewritten to their canonical name at parse time
//     via pflag.FlagSet.SetNormalizeFunc, so the canonical pflag is the only
//     object that receives the value. Aliases do not appear in --help.
//   - A PreRunE hook (chained after any existing one) inspects the raw
//     process argv: if an alias and its canonical flag are both explicitly
//     supplied in the same invocation, or two different aliases of the
//     same canonical flag are supplied together, the alias is reported as
//     an unknown flag.
//   - Bind is the sole owner of SetNormalizeFunc on cmd; callers must not
//     install another normalizer on the same FlagSet.
//
// Bind panics on obvious programmer errors (unknown canonical flag, empty
// alias, alias colliding with an existing flag) so misconfiguration surfaces
// at startup rather than at request time.
func Bind(cmd *cobra.Command, specs []Spec) {
	if len(specs) == 0 {
		return
	}

	aliasToCanonical := make(map[string]string)
	for _, spec := range specs {
		if spec.Canonical == "" {
			panic("flagalias: empty canonical name")
		}
		if cmd.Flags().Lookup(spec.Canonical) == nil {
			panic("flagalias: unknown canonical flag --" + spec.Canonical)
		}
		for _, alias := range spec.Aliases {
			if alias == "" {
				panic("flagalias: empty alias for --" + spec.Canonical)
			}
			if alias == spec.Canonical {
				panic("flagalias: alias equals canonical --" + spec.Canonical)
			}
			if cmd.Flags().Lookup(alias) != nil {
				panic("flagalias: alias --" + alias + " collides with an existing flag")
			}
			if existing, ok := aliasToCanonical[alias]; ok {
				panic("flagalias: alias --" + alias + " bound twice (to --" +
					existing + " and --" + spec.Canonical + ")")
			}
			aliasToCanonical[alias] = spec.Canonical
		}
	}

	installNormalizer(cmd, aliasToCanonical)
	installConflictCheck(cmd, aliasToCanonical)
}

// installNormalizer rewrites alias names to their canonical names at parse
// time. Any normalizer already installed on cmd.Flags() is composed first
// so that its result feeds into alias resolution.
func installNormalizer(cmd *cobra.Command, aliasToCanonical map[string]string) {
	prev := cmd.Flags().GetNormalizeFunc()
	cmd.Flags().SetNormalizeFunc(func(fs *pflag.FlagSet, name string) pflag.NormalizedName {
		if prev != nil {
			name = string(prev(fs, name))
		}
		if canonical, ok := aliasToCanonical[name]; ok {
			return pflag.NormalizedName(canonical)
		}
		return pflag.NormalizedName(name)
	})
}

// installConflictCheck chains a PreRunE hook that walks the raw argv and
// rejects invocations where an alias and its canonical flag, or two
// aliases of the same canonical flag, are both explicitly supplied.
func installConflictCheck(cmd *cobra.Command, aliasToCanonical map[string]string) {
	prev := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if err := detectConflict(argvFn(), aliasToCanonical); err != nil {
			return err
		}
		if prev != nil {
			return prev(c, args)
		}
		return nil
	}
}

// detectConflict scans tokens for explicit occurrences of the names in
// aliasToCanonical (both aliases and canonicals) and returns an
// "unknown flag" error when more than one spelling of the same canonical
// flag is explicitly supplied.
//
// The scan understands the common pflag long-flag spellings:
//
//	--name value
//	--name=value
//	--name              (boolean / NoOptDefVal, no following value consumed)
//
// Shorthand "-x" forms are ignored because aliases are long-only. Anything
// after a bare "--" terminator is treated as positional and skipped so a
// positional value that happens to look like "--query" cannot trigger a
// false positive.
func detectConflict(tokens []string, aliasToCanonical map[string]string) error {
	// canonical -> ordered list of distinct spellings observed on the CLI.
	seen := make(map[string][]string)

	canonicalSet := make(map[string]struct{})
	for _, canonical := range aliasToCanonical {
		canonicalSet[canonical] = struct{}{}
	}

	for _, tok := range tokens {
		if tok == "--" {
			// Everything after a bare "--" is positional; stop scanning.
			break
		}
		if !strings.HasPrefix(tok, "--") {
			continue
		}
		name := strings.TrimPrefix(tok, "--")
		if i := strings.IndexByte(name, '='); i >= 0 {
			name = name[:i]
		}
		if name == "" {
			continue
		}

		var canonical string
		if c, ok := aliasToCanonical[name]; ok {
			canonical = c
		} else if _, ok := canonicalSet[name]; ok {
			canonical = name
		} else {
			continue
		}

		spellings := seen[canonical]
		duplicate := false
		for _, s := range spellings {
			if s == name {
				duplicate = true
				break
			}
		}
		if !duplicate {
			seen[canonical] = append(spellings, name)
		}
	}

	for canonical, spellings := range seen {
		if len(spellings) < 2 {
			continue
		}
		offender := canonical
		for _, s := range spellings {
			if s != canonical {
				offender = s
				break
			}
		}
		return exception.InvalidArgsError.With("unknown flag: --%s", offender)
	}
	return nil
}
