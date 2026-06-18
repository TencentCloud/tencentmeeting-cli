package output

import (
	"encoding/json"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// optionsMsg defines the message.
type optionsMsg struct {
	cmd *cobra.Command

	data    string   // output data
	traceId string   // output traceId
	message string   // output message
	hints   []string // output hints
}

// Option defines the options.
type Option func(msg *optionsMsg)

// getOptions gets the options.
func getOptions(msg *optionsMsg, opts ...Option) {
	for _, o := range opts {
		o(msg)
	}
}

// WithCompact defines the compact format.
func WithCompact(compactFields []string) Option {
	return func(msg *optionsMsg) {
		compact, _ := msg.cmd.Root().PersistentFlags().GetBool("compact")
		if !compact {
			return
		}

		// compact only keep the specified fields
		// traceId and message are not needed
		msg.traceId = ""
		msg.message = ""
		msg.data = string(utils.KeepFields([]byte(msg.data), 10, compactFields))
	}
}

// WithConvert defines the convert format.
func WithConvert(convertFields map[string]utils.FieldConverter) Option {
	return func(msg *optionsMsg) {
		msg.data = string(utils.ConvertFields([]byte(msg.data), 10, convertFields))
	}
}

// WithContactSearchLogic defines the contact search logic.
func WithContactSearchLogic() Option {
	return func(msg *optionsMsg) {
		dataMap := make(map[string]interface{})
		err := json.Unmarshal([]byte(msg.data), &dataMap)
		if err != nil {
			// do nothing
			return
		}
		if dataUsers, ok := dataMap["users"].([]interface{}); ok && len(dataUsers) == 1 {
			// if the users only one, we need to keep the field open_id only
			msg.data = string(utils.KeepFields([]byte(msg.data), 10, []string{"open_id"}))
		}
	}
}

// WithHints defines the hints format.
// fn is a lazy function that generates hints, called only when output is rendered.
func WithHints(fn func(string) []string) Option {
	return func(msg *optionsMsg) {
		if fn != nil {
			msg.hints = fn(msg.data)
		}
	}
}

// WithFilterFields defines the filter fields format.
func WithFilterFields(filterFields []string) Option {
	return func(msg *optionsMsg) {
		if len(filterFields) == 0 {
			return
		}

		msg.data = string(utils.DeleteFields([]byte(msg.data), 10, filterFields))
	}
}
