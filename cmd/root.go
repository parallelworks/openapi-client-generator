package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "openapi-client-generator",
	Short: "Generate Go HTTP clients from OpenAPI 3.1 specs",
	Long:  "A code generator that produces feature-rich Go HTTP client packages from OpenAPI 3.1 specifications.",
}

// setupErr carries a failure from a command's init, which cannot return one.
var setupErr error

// Execute runs the root command.
func Execute() error {
	if setupErr != nil {
		return setupErr
	}
	return rootCmd.Execute()
}
