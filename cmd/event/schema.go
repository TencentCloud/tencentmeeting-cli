package event

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"tmeet/internal"
	eventruntime "tmeet/internal/event"
	"tmeet/internal/exception"
	"tmeet/internal/output"
)

// schemaView is the on-the-wire shape of `event schema <key>`.
//
// Defined as a separate struct (rather than reusing eventruntime.KeyDef
// directly) because:
//
//   - we need to decode ResolvedOutputSchema from RawMessage into a real object
//     so that --format json-pretty produces nicely indented JSON instead of
//     a quoted string blob;
//   - we want field ordering and keys to exactly match the public contract,
//     not the internal struct's idiosyncratic naming.
type schemaView struct {
	Key                  string                           `json:"key"`
	Domain               string                           `json:"domain"`
	JQRootPath           string                           `json:"jq_root_path"`
	ParamsSchema         map[string]eventruntime.ParamDef `json:"params_schema"`
	ResolvedOutputSchema interface{}                      `json:"resolved_output_schema"`
}

// SchemaOptions holds the options for `tmeet event schema <EventKey>`.
//
// EventKey is bound from the positional argument in the RunE wrapper
// rather than from a flag, mirroring `event consume`.  Arity is enforced
// by a custom Args validator (see newSchemaCmd) so the "missing argument"
// message can point users at `tmeet event list`.
type SchemaOptions struct {
	tmeet    *internal.Tmeet
	EventKey string // EventKey to inspect; bound from args[0]
}

// newSchemaCmd implements `tmeet event schema <EventKey>`.
//
// Output contract:
//   - stdout: a single JSON object with key/domain/jq_root_path/params_schema/
//     resolved_output_schema; no envelope.
//   - exit codes: 0 on success; 1 on unknown key (with hint).
//
// As with `event list`, this command is a pure local-registry read and is
// annotated skipPreCheck.
func newSchemaCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &SchemaOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "schema <EventKey>",
		Short: "Show the contract (params / output schema / jq root) for an EventKey",
		Long: `Show the full contract for an EventKey: parameter schema (--param keys),
the JSON Schema of the event payload, and the jq root path used to write
--jq expressions.  Output is a single JSON object on stdout (use --format
json-pretty for an indented view).`,
		Annotations: map[string]string{"skipPreCheck": "true"},
		// Custom Args validator (instead of cobra.ExactArgs(1)) so that the
		// "missing EventKey" error mirrors the "unknown EventKey" branch's
		// tone and points the user at `tmeet event list` — the same
		// discovery hint they'd see if they typed an unknown key.  The
		// default cobra message ("accepts 1 arg(s), received 0") gives no
		// such guidance.
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return exception.InvalidArgsError.With(
					"event schema requires exactly one EventKey argument; " +
						"run 'tmeet event list' to discover registered keys, " +
						"or 'tmeet event schema --help' for usage")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.EventKey = args[0]
			return opts.Run(cmd, args)
		},
	}
	return cmd
}

// Run executes `event schema`.
func (o *SchemaOptions) Run(cmd *cobra.Command, args []string) error {
	def, ok := eventruntime.Lookup(o.EventKey)
	if !ok {
		return exception.InvalidArgsError.With(
			"unknown EventKey %q; run 'tmeet event list' to see available keys", o.EventKey)
	}

	// Decode the embedded JSON schema from RawMessage so it's rendered as a
	// real object (not a string) under --format json-pretty.
	var schemaObj interface{}
	if len(def.ResolvedOutputSchema) > 0 {
		if err := json.Unmarshal(def.ResolvedOutputSchema, &schemaObj); err != nil {
			// Defensive: schema is registered at compile time so this should
			// never trip in production; surface as an exit-1 error rather
			// than silently dropping the field.
			return exception.InvalidArgsError.With(
				"internal: malformed schema for %q: %v", o.EventKey, err)
		}
	}

	view := schemaView{
		Key:                  def.Key,
		Domain:               def.Domain,
		JQRootPath:           def.JQRootPath,
		ParamsSchema:         def.ParamsSchema,
		ResolvedOutputSchema: schemaObj,
	}
	if view.ParamsSchema == nil {
		// Same rationale as list: emit "{}" instead of "null" for jq friendliness.
		view.ParamsSchema = map[string]eventruntime.ParamDef{}
	}
	return output.EventPrint(cmd, view)
}
