package generator

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/tools/imports"
)

// WriteFiles formats each generated file with goimports and writes them to outputDir.
func WriteFiles(outputDir string, files []GeneratedFile) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	for _, f := range files {
		formatted, err := imports.Process(f.Name, f.Content, &imports.Options{
			Comments:  true,
			TabIndent: true,
			TabWidth:  8,
		})
		if err != nil {
			log.Printf("WARNING: goimports failed for %s: %v (writing unformatted)", f.Name, err)
			formatted = f.Content
		}

		path := filepath.Join(outputDir, f.Name)
		if err := os.WriteFile(path, formatted, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}
