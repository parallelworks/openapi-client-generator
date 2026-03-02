package parser

import (
	"fmt"
	"os"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ParseResult holds the parsed OpenAPI document and its high-level model.
type ParseResult struct {
	Document libopenapi.Document
	Model    *v3high.Document
	Version  string
}

// Config controls parser behavior.
type Config struct {
	AllowRemoteRefs bool
}

// Parse reads an OpenAPI spec file and returns a parsed result.
func Parse(specPath string, cfg Config) (*ParseResult, error) {
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	docConfig := &datamodel.DocumentConfiguration{
		AllowFileReferences:   true,
		AllowRemoteReferences: cfg.AllowRemoteRefs,
	}

	doc, err := libopenapi.NewDocumentWithConfiguration(specBytes, docConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing spec: %w", err)
	}

	docModel, err := doc.BuildV3Model()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if docModel == nil {
		return nil, fmt.Errorf("failed to build OpenAPI v3 model")
	}

	return &ParseResult{
		Document: doc,
		Model:    &docModel.Model,
		Version:  doc.GetVersion(),
	}, nil
}
