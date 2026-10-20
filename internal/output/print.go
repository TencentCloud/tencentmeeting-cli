package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// marshalNoEscape marshals v to JSON without escaping HTML special characters
// (<, >, &). The standard library's json.Marshal escapes these by default to
// be safe for HTML embedding, but for CLI output (URLs, query strings, etc.)
// we want them rendered verbatim.
func marshalNoEscape(v interface{}, indent bool) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder.Encode appends a trailing newline; strip it so callers
	// can rely on "no extra whitespace at the end" the same way they used
	// to with json.Marshal.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// formatOutput defines the output format.
type formatOutput struct {
	TraceId string      `json:"trace_id,omitempty"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Hints   []string    `json:"hints,omitempty"`
}

// PrintInfof prints informational messages.
func PrintInfof(cmd *cobra.Command, format string, args ...interface{}) {
	// When there are no args, treat format as a plain string to avoid
	// stray '%' in the content being interpreted as format verbs
	// (which would produce output like "%!(MISSING)").
	var msg string
	if len(args) == 0 {
		msg = format
	} else {
		msg = fmt.Sprintf(format, args...)
	}
	if cmd == nil {
		fmt.Println(msg)
		return
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
}

// PrintErrorf prints error messages.
func PrintErrorf(cmd *cobra.Command, format string, args ...interface{}) {
	// Same rationale as PrintInfof: skip formatting when no args are
	// provided so that literal '%' characters are preserved.
	var msg string
	if len(args) == 0 {
		msg = format
	} else {
		msg = fmt.Sprintf(format, args...)
	}
	if cmd == nil {
		fmt.Fprintln(os.Stderr, "Error:", msg)
		return
	}
	_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "Error:", msg)
}

// FormatPrint prints data in the specified format.
func FormatPrint(cmd *cobra.Command, traceId, message, data string, opts ...Option) {
	// get options
	optMsg := &optionsMsg{
		cmd:     cmd,
		data:    data,
		traceId: traceId,
		message: message,
	}
	getOptions(optMsg, opts...)

	fo := &formatOutput{
		TraceId: optMsg.traceId,
		Message: optMsg.message,
		Hints:   optMsg.hints,
	}
	dataMap := make(map[string]interface{})
	err := json.Unmarshal([]byte(optMsg.data), &dataMap)
	if err != nil {
		fo.Data = optMsg.data
	} else {
		fo.Data = dataMap
	}

	var b []byte
	f := GetFormat(cmd)
	switch f {
	case "json-pretty":
		b, _ = marshalNoEscape(fo, true)
	case "json":
		fallthrough
	default:
		b, _ = marshalNoEscape(fo, false)
	}
	PrintInfof(cmd, string(b))
}

// GetFormat gets the format.
func GetFormat(cmd *cobra.Command) string {
	f, err := cmd.Root().PersistentFlags().GetString("format")
	if err != nil || f == "" {
		return "json"
	}
	return f
}
