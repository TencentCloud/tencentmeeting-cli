// consume.go — `tmeet event consume --event-id <EventKey>`.
//
// Top-level command + parameter validation only; the actual event loop and
// IO plumbing lives in consume_runner.go.  Splitting the two keeps cobra
// wiring readable and lets tests drive runConsumeLoop directly without
// going through the cobra layer.
//
// Scope of this batch (2.4):
//   - Required --event-id <key>; rejected if not in the registry.
//   - --max-events / --timeout / --quiet / --output-dir.
//   - Auto-fork _bus when none is running.
//   - Hello → HelloAck → consume loop → graceful Bye on shutdown.
//
// Multi-key / --domain are out of scope for this batch; the wire
// protocol (Hello.EventKeys []string) is a slice so the bus can accept
// a longer list without a protocol churn if that scope changes.
//
// Out of scope (later batches):
//   - --param key=value         (needs schema-driven param validation)
//   - --jq <expr>               (needs gojq + per-event evaluator)
//   - automatic reconnect       (a single Hello error is fatal by design)

package event

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tmeet/internal"
	"tmeet/internal/config"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/event/jqfilter"
	"tmeet/internal/event/spawner"
	"tmeet/internal/event/transport"
	"tmeet/internal/exception"
	"tmeet/internal/output"
)

// ConsumeOptions bundles the parsed flags + immutable runtime context so the
// runner can be a single-receiver method (easier to mock in tests).
type ConsumeOptions struct {
	tmeet      *internal.Tmeet   // injected; nil only in tests that drive runLoop directly
	EventKey   string            // bound from --event-id; single-key only
	ParamsRaw  []string          // raw "k=v" slice from --param; validated into Params before runLoop
	Params     map[string]string // validated; nil/empty when no --param given
	JQ         string            // raw --jq expression (kept for stderr/debug); empty = pass-through
	JQRoot     string            // KeyDef.JQRootPath snapshot; "." or ".payload"
	jqFilter   *jqfilter.Filter  // compiled --jq; nil iff JQ==""
	MaxEvents  int
	Timeout    time.Duration
	Quiet      bool
	OutputDir  string
	BusVersion string // injected from internal.Tmeet.CLIVersion for Hello.Version

	// AgentOpenID is the agent (sub-account) open_id forwarded in Hello for
	// EventKeys whose SubscribeRole == agent.  Resolved in Run from
	// config.GetAgentAccountConfig(); empty for master/none events.
	AgentOpenID string
}

// consumeOpts is a backward-compatible alias retained so the in-package
// tests (consume_test.go) can keep their existing `&consumeOpts{...}`
// literals — switching them all to `&ConsumeOptions{...}` would touch ~25
// lines of tests for no functional gain.  Production code uses the
// exported name (ConsumeOptions).
type consumeOpts = ConsumeOptions

// newConsumeCmd builds `tmeet event consume --event-id <EventKey> [flags]`.
//
// Annotated WITHOUT skipPreCheck: consume needs a real authenticated user
// to compute OpenIDHash for the Hello frame.  Without that the bus would
// reject us with WrongOwner, and the resulting error message would be far
// less helpful than preCheck's "user config is empty, please use 'tmeet
// auth login'".
func newConsumeCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ConsumeOptions{tmeet: tmeet}

	cmd := &cobra.Command{
		Use:   "consume --event-id <EventKey>",
		Short: "Subscribe to and stream events for one EventKey",
		Long: `Subscribe to a Tencent Meeting event stream and emit one NDJSON line per
event to stdout.  stderr carries control-plane diagnostics (handshake
status, source state, drop notifications).

The EventKey is passed via the required --event-id flag; positional
arguments are rejected.  Only a single EventKey is accepted.

Two operating modes:
  batch:        --max-events N or --timeout D set; exits 0 on first match.
  long-running: neither set; exits on SIGINT/SIGTERM or 'tmeet event stop'.

stderr emits a stable ready marker once the consumer is attached and ready
to receive events:

    [event] ready event_key=<key>

Agents can grep for that line to know when to trigger upstream actions.
The marker is NOT suppressed by --quiet — it's part of the public contract.`,
		Args: func(cmd *cobra.Command, args []string) error {
			// Positional args are always invalid: the EventKey moved to the
			// required --event-id flag.  We keep the friendly-error contract
			// (mentions 'tmeet event list' and '--help') the previous
			// positional-argument validator established, so tests and users
			// see a stable diagnostic tone across the migration.
			if len(args) != 0 {
				return exception.InvalidArgsError.With(
					"event consume no longer accepts positional arguments; " +
						"pass the EventKey via --event-id <key> instead " +
						"(e.g. 'tmeet event consume --event-id meeting.started'). " +
						"Run 'tmeet event list' to discover registered keys, " +
						"or 'tmeet event consume --help' for usage")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// --event-id is required; enforce here rather than via
			// MarkFlagRequired so we can emit the same friendly-error tone
			// as the positional-args branch above (mentions 'tmeet event
			// list' and '--help').
			//
			// Also normalise leading/trailing whitespace: without this a
			// value like "meeting.started " would pass the emptiness check
			// only to fail later at registry Lookup with the confusing
			// message `unknown EventKey "meeting.started "`.  Writing the
			// trimmed value back to opts makes the downstream Lookup /
			// Hello frame see the canonical form.
			opts.EventKey = strings.TrimSpace(opts.EventKey)
			if opts.EventKey == "" {
				return exception.InvalidArgsError.With(
					"event consume requires --event-id <EventKey>; " +
						"run 'tmeet event list' to discover registered keys, " +
						"or 'tmeet event consume --help' for usage")
			}
			opts.BusVersion = tmeet.CLIVersion
			return opts.Run(cmd, args)
		},
	}

	cmd.Flags().StringVar(&opts.EventKey, "event-id", "",
		"EventKey to subscribe to (required); run 'tmeet event list' to discover registered keys")
	cmd.Flags().IntVar(&opts.MaxEvents, "max-events", 0,
		"exit after N events have been delivered to stdout (0 = unlimited)")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 0,
		"exit after this long since the ready marker (0 = unlimited)")
	cmd.Flags().BoolVar(&opts.Quiet, "quiet", false,
		"suppress informational stderr lines (ready marker and exit line are still emitted)")
	cmd.Flags().StringVar(&opts.OutputDir, "output-dir", "",
		"additionally write each event to <output-dir>/<trace_id>.json (relative path only)")
	cmd.Flags().StringArrayVar(&opts.ParamsRaw, "param", nil,
		"narrow the subscription with key=value pairs (repeatable); run 'tmeet event schema <key>' for valid keys")
	cmd.Flags().StringVar(&opts.JQ, "jq", "",
		"gojq expression evaluated per event; null/no-result drops the event, otherwise its output replaces the default NDJSON line")

	return cmd
}

// Run is the top-level body for `event consume`.
//
//  1. Validate flags (EventKey existence, --max-events / --timeout / --output-dir).
//  2. Compute the consumer's OpenIDHash for Hello.
//  3. EnsureBus — fork _bus if none running.
//  4. Hello → HelloAck → run the event loop.
//
// All "fatal" errors return non-nil to cobra (which root.go translates to
// exit code 1).  Graceful exits return nil after emitting the exit line.
func (o *ConsumeOptions) Run(cmd *cobra.Command, args []string) error {
	// 1a. EventKey must exist in the local registry.
	def, ok := eventruntime.Lookup(o.EventKey)
	if !ok {
		return exception.InvalidArgsError.With("unknown EventKey %q; run 'tmeet event list' to discover registered keys",
			o.EventKey)
	}
	o.JQRoot = def.JQRootPath

	// 1a-0. SubscribeRole gate.  EventKeys flagged as agent-only require a
	// configured agent (子账号); we validate its existence BEFORE forking
	// the bus / handshaking so the user gets the obvious error early
	// instead of a silent "no events ever arrive".  The resolved
	// agent_open_id is forwarded in Hello and threaded down to the
	// upstream SubscribeReq.  The bus itself always connects with the
	// MASTER account — agent identity is per-subscription, not per-conn.
	switch def.SubscribeRole {
	case eventruntime.SubscribeRoleAgent:
		agent, aerr := config.GetAgentAccountConfig()
		if aerr != nil {
			return aerr
		}
		if agent == nil || agent.AgentOpenId == "" {
			return exception.InvalidArgsError.With(
				"event %q can only be subscribed by an agent (sub-account), but none is configured; create one first",
				o.EventKey)
		}
		o.AgentOpenID = agent.AgentOpenId
	case eventruntime.SubscribeRoleMaster, eventruntime.SubscribeRoleNone, "":
		// master / unrestricted: no agent required; AgentOpenID stays empty.
	}

	// 1a-bis. --param: parse + validate against the EventKey's ParamsSchema.
	// Done after the EventKey existence check so a typo in the key produces
	// the obvious error first (rather than a confusing "unknown --param" for
	// every flag the user passed).
	params, err := eventruntime.ParseAndValidateParams(o.EventKey, o.ParamsRaw)
	if err != nil {
		return err
	}
	o.Params = params

	// 1a-ter. --jq: compile up-front so a syntax error fails BEFORE we fork
	// the bus / handshake / emit the ready marker.  An Agent that captures
	// stderr expecting `[event] ready ...` would otherwise sit waiting for
	// a marker that never arrives.
	if o.JQ != "" {
		flt, jerr := jqfilter.Compile(o.JQ)
		if jerr != nil {
			return jerr
		}
		o.jqFilter = flt
	}

	// 1b. Numeric bounds.
	if o.MaxEvents < 0 {
		return exception.InvalidArgsError.With("--max-events must be >= 0, got %d", o.MaxEvents)
	}
	if o.Timeout < 0 {
		return exception.InvalidArgsError.With("--timeout must be >= 0, got %s", o.Timeout)
	}

	// 1c. --output-dir: relative path only (hard rule of the consume contract).
	// Reject absolute paths and any '..' segment to avoid escaping the working dir.
	if o.OutputDir != "" {
		if filepath.IsAbs(o.OutputDir) {
			return exception.InvalidArgsError.With("--output-dir must be relative, got absolute path %q", o.OutputDir)
		}
		clean := filepath.Clean(o.OutputDir)
		// After Clean, leading "../" is preserved; reject any segment == ".."
		// to prevent escapes via e.g. "./a/../../b".
		for _, seg := range strings.Split(clean, string(os.PathSeparator)) {
			if seg == ".." {
				return exception.InvalidArgsError.With("--output-dir must not contain '..' segments, got %q", o.OutputDir)
			}
		}
		// Pre-create the directory so a runtime write failure is the
		// exception rather than the default.  0o755 mirrors the default
		// umask behaviour for user-created dirs.
		if err := os.MkdirAll(clean, 0o755); err != nil {
			return exception.InvalidArgsError.With("create --output-dir %q: %v", clean, err)
		}
		o.OutputDir = clean
	}

	// 2. OpenIDHash.  preCheck already ensured a user is logged in, but the
	// hash itself must be non-empty (else WrongOwner from any bus would
	// trivially pass).
	ownerHash := currentOpenIDHash()
	if ownerHash == "" {
		return exception.GetUserConfigEmptyError.With(
			"event consume requires an authenticated user; run 'tmeet auth login' first")
	}

	// 3. Make sure a bus is running.
	tr := transport.New()
	o.stderrf(cmd, "[event] starting consume key=%s", o.EventKey)
	forked, err := spawner.EnsureBus(tr)
	if err != nil {
		return exception.EventBusError.With("event consume: ensure bus: %v", err)
	}
	if forked {
		o.stderrf(cmd, "[event] bus not running, forked daemon")
	}

	// 4. Connect → Hello → HelloAck → loop.
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	return o.runLoop(ctx, cmd, tr, ownerHash)
}

// runConsume is a thin shim retained so consume_test.go can keep driving
// the validation + fork path via runConsume(cmd, &consumeOpts{...}).
// Production code reaches Run via newConsumeCmd's RunE.
func runConsume(cmd *cobra.Command, opts *ConsumeOptions) error {
	return opts.Run(cmd, nil)
}

// stderrf writes to stderr unless --quiet AND the line is informational.
//
// "Informational" is the default; the ready marker, the exit line and any
// WARN/error must call output.EventStderr directly so --quiet can't hide them.
//
// The split is hard-coded (vs. having a `level int` arg) because the set of
// always-on lines is tiny and explicit listing makes the contract grep-able
// from outside this file.
func (o *ConsumeOptions) stderrf(cmd *cobra.Command, format string, args ...interface{}) {
	if o != nil && o.Quiet {
		return
	}
	output.EventStderr(cmd, format, args...)
}
