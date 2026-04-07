package log

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Infof prints informational messages.
func Infof(cmd *cobra.Command, format string, args ...interface{}) {
	if cmd == nil {
		fmt.Printf(format+"\n", args...)
		return
	}
	cmd.Println(fmt.Sprintf(format, args...))
}

// Errorf prints error messages.
func Errorf(cmd *cobra.Command, format string, args ...interface{}) {
	if cmd == nil {
		fmt.Printf("Error: "+format+"\n", args...)
		return
	}
	cmd.Println("Error:", fmt.Sprintf(format, args...))
}

// GetFormat 从根命令的 PersistentFlags 中读取 --format 值，默认返回 "json"
func GetFormat(cmd *cobra.Command) string {
	f, err := cmd.Root().PersistentFlags().GetString("format")
	if err != nil || f == "" {
		return "json"
	}
	return f
}
