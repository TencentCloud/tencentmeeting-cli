package record

import (
	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"

	"github.com/spf13/cobra"
)

// Content sub-action values.
const (
	contentActionMinutes    = "minutes"
	contentActionTodos      = "todos"
	contentActionTranscript = "transcript"
	contentActionParagraphs = "paragraphs"
	contentActionTimeline   = "timeline"
)

// contentLongDoc is the --help long description of `tmeet record content`.
// Kept as a package-level constant next to the command shell so that all
// dispatch-time facts (action names, flag layout, implicit sub-forms) live
// in one place and stay in sync with newContentCmd's flag registration.
//
// The doc is organized around "forms": a form = one concrete (--action,
// flag-combination) that the CLI dispatches to a single backend call. Some
// actions have exactly one form (todos / paragraphs / timeline), while
// --action minutes has two and --action transcript has three. Users should
// pick a form from the matrix below and provide exactly that form's flags.
const contentLongDoc = "Read record-related content. Pick one form from the matrix below\n" +
	"and provide exactly its required / optional flags.\n" +
	"\n" +
	"Form matrix (one row = one concrete call):\n" +
	"\n" +
	"  Form                   Required flags                          Optional flags                              Purpose\n" +
	"  ---------------------  --------------------------------------  ------------------------------------------  --------------------------------------------\n" +
	"  minutes(file)          --action minutes  --record-file-id      --lang, --pwd                               Smart minutes of a single record file\n" +
	"  minutes(url)           --action minutes  --share-url           --lang, --pwd, --page-token, --page-size    Minutes list by share url (paginated)\n" +
	"  todos                  --action todos    --record-file-id      --lang, --pwd                               Structured todos of a record file\n" +
	"  transcript(details)    --action transcript  --record-file-id   --meeting-id, --pid, --limit                Full transcript of a record file\n" +
	"  transcript(search)     --action transcript  --record-file-id   --meeting-id                                In-file keyword search\n" +
	"                         --text\n" +
	"  transcript(url)        --action transcript  --share-url        --page-token, --page-size                   Transcript list by share url (paginated)\n" +
	"  paragraphs             --action paragraphs  --record-file-id   --meeting-id                                Paragraph list (speaker / time)\n" +
	"  timeline               --action timeline    --record-file-id   --lang                                      AI smart timeline\n" +
	"\n" +
	"Form selection rules (how a form is picked at runtime):\n" +
	"  --action minutes:\n" +
	"    --share-url present -> minutes(url); otherwise -> minutes(file).\n" +
	"    --record-file-id and --share-url are mutually exclusive.\n" +
	"  --action transcript:\n" +
	"    --share-url present -> transcript(url)   (incompatible with --text);\n" +
	"    else --text present -> transcript(search);\n" +
	"    else                -> transcript(details).\n" +
	"\n" +
	"Any flag not listed in the chosen form's row is rejected with:\n" +
	"  \"--<flag> is not supported by --action <action>\""

// ContentOptions holds the union of options for `tmeet record content`. Which
// flags are used depends on the value of --action; per-action semantics are
// enforced by each action handler (see content_<action>.go files).
type ContentOptions struct {
	tmeet *internal.Tmeet

	Action string // required: minutes | todos | transcript | paragraphs | timeline

	// Common identifiers.
	RecordFileID string // used by all actions except when --share-url is provided
	ShareURL     string // used by minutes / transcript list-by-url branches
	MeetingID    string // used by transcript / paragraphs

	// Content-specific.
	Lang  string // used by minutes / todos / timeline
	Pwd   string // used by minutes / todos
	Pid   string // used by transcript get
	Limit string // used by transcript get
	Text  string // used by transcript search

	// Pagination (only for list-by-url branches).
	PageToken string
	PageSize  int
}

// newContentCmd builds the `tmeet record content` command. This is a pure
// command shell: it wires flags, assembles the ActionRegistry from
// per-action build functions (declared in content_<action>.go), then hands
// dispatch off to cmdutil.ActionRegistry.Dispatch. All action-specific logic
// (Validate / Run / ApiCmd resolution) stays in the corresponding
// content_<action>.go file.
func newContentCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &ContentOptions{tmeet: tmeet}

	// Each action lives in its own file; adding a new action is a matter of
	// creating content_<newaction>.go and registering it here.
	registry := cmdutil.ActionRegistry{
		contentActionMinutes:    opts.buildMinutesHandler(),
		contentActionTodos:      opts.buildTodosHandler(),
		contentActionTranscript: opts.buildTranscriptHandler(),
		contentActionParagraphs: opts.buildParagraphsHandler(),
		contentActionTimeline:   opts.buildTimelineHandler(),
	}

	cmd := &cobra.Command{
		Use:   "content",
		Short: "read record content (--action minutes|todos|transcript|paragraphs|timeline; see --help for form matrix)",
		Long:  contentLongDoc,
		RunE: middleWare.Chain(
			func(cmd *cobra.Command, args []string) error {
				return registry.Dispatch(cmd, opts.Action, "action")
			},
			middleWare.WithApiCmd(registry.BuildApiCmdResolver("action")),
			middleWare.WithCompact(tmeet),
		),
	}

	registerContentFlags(cmd, opts)
	return cmd
}

// registerContentFlags declares the union of flags consumed by any content
// form. Each flag's usage string names the exact forms it belongs to and
// its mutual-exclusion partners, so `--help` is self-contained and matches
// the form matrix in contentLongDoc. Per-form rules on which flags are
// required / forbidden are enforced by each handler's Validate + FlagRules.
func registerContentFlags(cmd *cobra.Command, opts *ContentOptions) {
	// --action is required and drives which forms are reachable.
	cmdutil.SingleStringVar(cmd.Flags(), &opts.Action, "action", "",
		"content type: minutes | todos | transcript | paragraphs | timeline (required)")
	_ = cmd.MarkFlagRequired("action")

	// Union of flags used by the forms. Each description names the forms
	// (see `--help` matrix) in which the flag is valid.
	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "",
		"record file id. Required for forms: minutes(file), todos, transcript(details|search), paragraphs, timeline. Mutually exclusive with --share-url")
	cmd.Flags().StringVar(&opts.ShareURL, "share-url", "",
		"recording access share url. Required for forms: minutes(url), transcript(url). Mutually exclusive with --record-file-id and --text")
	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "",
		"meeting id. Optional for forms: transcript(details|search), paragraphs")
	cmd.Flags().StringVar(&opts.Lang, "lang", "",
		"language: default (or empty) = original, zh, en, ja. Optional for forms: minutes(file|url), todos, timeline")
	cmd.Flags().StringVar(&opts.Pwd, "pwd", "",
		"record file access password. Optional for forms: minutes(file|url), todos")
	cmd.Flags().StringVar(&opts.Pid, "pid", "",
		"start paragraph id. Optional for form: transcript(details) only")
	cmd.Flags().StringVar(&opts.Limit, "limit", "",
		"number of paragraphs to query. Optional for form: transcript(details) only")
	cmd.Flags().StringVar(&opts.Text, "text", "",
		"in-file search keyword. Required for form: transcript(search); only valid with --action transcript. Mutually exclusive with --share-url")
	cmd.Flags().StringVar(&opts.PageToken, "page-token", "",
		"page token for list-by-url forms. Optional for forms: minutes(url), transcript(url)")
	cmd.Flags().IntVar(&opts.PageSize, "page-size", 5,
		"page size for list-by-url forms (max 5). Optional for forms: minutes(url), transcript(url)")
}
