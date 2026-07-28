package output

import (
	"bytes"
	"encoding/json"
	"fmt"

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
	if cmd == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), fmt.Sprintf(format, args...))
}

// PrintErrorf prints error messages.
func PrintErrorf(cmd *cobra.Command, format string, args ...interface{}) {
	if cmd == nil {
		fmt.Printf("Error: "+format+"\n", args...)
		return
	}
	_, _ = fmt.Fprintln(cmd.OutOrStderr(), "Error:", fmt.Sprintf(format, args...))
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
