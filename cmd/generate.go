package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/generator"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

var generateFlags struct {
	specPath        string
	outputDir       string
	packageName     string
	userAgent       string
	allowRemoteRefs bool
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringVarP(&generateFlags.specPath, "spec", "s", "", "path to OpenAPI spec file (required)")
	generateCmd.Flags().StringVarP(&generateFlags.outputDir, "out", "o", "", "output directory for generated code (required)")
	generateCmd.Flags().StringVarP(&generateFlags.packageName, "package", "p", "", "Go package name (default: derived from output dir)")
	generateCmd.Flags().StringVar(&generateFlags.userAgent, "user-agent", "", `default User-Agent for generated clients (default "openapi-client-generator/1.0")`)
	generateCmd.Flags().BoolVar(&generateFlags.allowRemoteRefs, "allow-remote-refs", false, "allow fetching remote $ref targets")

	generateCmd.MarkFlagRequired("spec")
	generateCmd.MarkFlagRequired("out")
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a Go client from an OpenAPI spec",
	Long:  "Parses an OpenAPI 3.1 specification and generates a complete Go HTTP client package.",
	Example: `  openapi-client-generator generate -s petstore.yaml -o ./gen/petstore
  openapi-client-generator generate --spec api.yaml --out ./client --package myclient`,
	RunE: func(cmd *cobra.Command, args []string) error {
		packageName := generateFlags.packageName
		if packageName == "" {
			packageName = derivePackageName(generateFlags.outputDir)
		}

		return generate(generateFlags.specPath, generateFlags.outputDir, packageName, generateFlags.userAgent, generateFlags.allowRemoteRefs)
	},
}

func generate(specPath, outputDir, packageName, userAgent string, allowRemoteRefs bool) error {
	result, err := parser.Parse(specPath, parser.Config{
		AllowRemoteRefs: allowRemoteRefs,
	})
	if err != nil {
		return fmt.Errorf("parsing spec: %w", err)
	}
	fmt.Printf("Parsed OpenAPI %s spec: %s\n", result.Version, specPath)

	a := analyzer.New(result.Model)
	pkg, err := a.Analyze(packageName)
	if err != nil {
		return fmt.Errorf("analyzing spec: %w", err)
	}
	pkg.UserAgent = userAgent

	gen, err := generator.New(pkg)
	if err != nil {
		return fmt.Errorf("initializing generator: %w", err)
	}

	files, err := gen.Generate()
	if err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	if err := generator.WriteFiles(outputDir, files); err != nil {
		return fmt.Errorf("writing files: %w", err)
	}

	fmt.Printf("Generated %d types into %s (package %s)\n", len(pkg.Types), outputDir, packageName)
	return nil
}

func derivePackageName(outputDir string) string {
	base := filepath.Base(outputDir)
	var b strings.Builder
	for _, r := range strings.ToLower(base) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		return "client"
	}
	return name
}
