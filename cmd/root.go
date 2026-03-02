package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "openapi-client-generator",
	Short: "Generate Go HTTP clients from OpenAPI 3.1 specs",
	Long:  "A code generator that produces feature-rich Go HTTP client packages from OpenAPI 3.1 specifications.",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
