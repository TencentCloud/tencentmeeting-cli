package cmdutil

import (
	"strings"
	"tmeet/internal/exception"

	"github.com/spf13/pflag"
)

// EnumValue is a custom cobra Flag Value for enum type.
// It implements the pflag.Value interface.
type EnumValue struct {
	Value   *string
	Allowed []string
}

// String returns the string representation of the enum value.
func (e *EnumValue) String() string {
	if e.Value == nil || *e.Value == "" {
		return ""
	}
	return *e.Value
}

// Set sets the enum value from a string, validating against the allowed values.
func (e *EnumValue) Set(value string) error {
	for _, allowed := range e.Allowed {
		if value == allowed {
			*e.Value = value
			return nil
		}
	}
	return exception.InvalidArgsError.With("invalid value %q, allowed values: %s", value, strings.Join(e.Allowed, ", "))
}

// Type returns the type string for the enum flag.
func (e *EnumValue) Type() string {
	return "enum"
}

// ensure EnumValue implements pflag.Value at compile time
var _ pflag.Value = &EnumValue{}
