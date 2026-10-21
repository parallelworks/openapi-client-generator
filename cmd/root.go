package cmd

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is set by the release build; `go install` and `go tool` builds
// report their module version instead.
var version string

func init() {
	if version == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			version = info.Main.Version
		}
	}
	rootCmd.Version = version
}

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
