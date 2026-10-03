package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestFormatPrintPreservesPercentCharacters(t *testing.T) {
	for _, format := range []string{"json", "json-pretty"} {
		for _, tt := range []struct {
			name string
			data string
		}{
			{"object", `{"text":"100% complete; %s %d %% %20","nested":{"value":"50%"},"url":"https://example.com/a%20b?rate=75%25"}`},
			{"plain text", `plain text: 100% complete; %s %d %% %20`},
		} {
			t.Run(format+"/"+tt.name, func(t *testing.T) {
				var buf bytes.Buffer
				cmd := &cobra.Command{}
				cmd.SetOut(&buf)
				cmd.PersistentFlags().String("format", format, "")
				traceID := "trace%04d"
				message := "success 100%; %s %%"
				FormatPrint(cmd, traceID, message, tt.data)

				var got map[string]interface{}
				if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
					t.Fatalf("output is not valid JSON: %v; output=%q", err, buf.String())
				}
				if got["trace_id"] != traceID || got["message"] != message {
					t.Fatalf("metadata changed: got trace=%q message=%q", got["trace_id"], got["message"])
				}
				var want interface{}
				if err := json.Unmarshal([]byte(tt.data), &want); err != nil {
					want = tt.data
				}
				if !reflect.DeepEqual(got["data"], want) {
					t.Fatalf("data changed: got %#v, want %#v", got["data"], want)
				}
				if !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
					t.Fatal("output must end in a newline")
				}
			})
		}
	}
}
