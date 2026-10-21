package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/pb33f/libopenapi/datamodel"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	v3low "github.com/pb33f/libopenapi/datamodel/low/v3"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/utils"
)

// ParseResult holds the parsed OpenAPI document's high-level model.
type ParseResult struct {
	Model   *v3high.Document
	Version string
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

	// A $ref is relative to the document holding it, not to wherever the
	// generator was run from, so file references start at the spec's directory.
	basePath, err := filepath.Abs(filepath.Dir(specPath))
	if err != nil {
		return nil, fmt.Errorf("resolving the spec's directory: %w", err)
	}

	docConfig := &datamodel.DocumentConfiguration{
		BasePath:              basePath,
		AllowFileReferences:   true,
		AllowRemoteReferences: cfg.AllowRemoteRefs,
		// The analyzer reads only the model, never the index's diagnostic
		// metadata, and collecting it is a large share of parse allocations.
		SkipMetadataCollection: true,
	}

	// The root libopenapi package would do these steps for us, but importing it
	// compiles its diffing and Arazzo models too, which dominate a cold build.
	info, err := extractSpecInfo(specBytes, docConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing spec: %w", err)
	}
	switch info.SpecFormat {
	case datamodel.OAS3, datamodel.OAS31, datamodel.OAS32:
	default:
		return nil, fmt.Errorf("parsing spec: unsupported spec version %q, only OpenAPI 3.x is supported", info.Version)
	}

	lowDoc, docErr := v3low.CreateDocumentFromConfig(info, docConfig)
	if lowDoc == nil {
		return nil, fmt.Errorf("failed to build OpenAPI v3 model: %w", docErr)
	}
	for _, err := range utils.UnwrapErrors(docErr) {
		var refErr *index.ResolvingError
		if errors.As(err, &refErr) && refErr.CircularReference == nil {
			return nil, fmt.Errorf("failed to build OpenAPI v3 model: %w", docErr)
		}
	}
	if docErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", docErr)
	}

	model := v3high.NewDocument(lowDoc)
	model.Rolodex = lowDoc.Index.GetRolodex()
	lowbase.SchemaQuickHashMap.Clear()

	return &ParseResult{Model: model, Version: info.Version}, nil
}

// extractSpecInfo is datamodel.ExtractSpecInfoWithConfig with a faster parse
// for JSON documents, which are the common machine-generated case.
func extractSpecInfo(spec []byte, cfg *datamodel.DocumentConfiguration) (*datamodel.SpecInfo, error) {
	trimmed := bytes.TrimSpace(spec)
	if len(trimmed) == 0 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' || !utf8.Valid(spec) {
		return datamodel.ExtractSpecInfoWithConfig(spec, cfg)
	}
	root, err := parseJSONNode(spec)
	if err != nil {
		return datamodel.ExtractSpecInfoWithConfig(spec, cfg)
	}
	_, openAPI3 := utils.FindKeyNode(utils.OpenApi3, root.Content)
	_, openAPI2 := utils.FindKeyNode(utils.OpenApi2, root.Content)
	_, asyncAPI := utils.FindKeyNode(utils.AsyncApi, root.Content)
	if openAPI3 == nil || openAPI2 != nil || asyncAPI != nil {
		return datamodel.ExtractSpecInfoWithConfig(spec, cfg)
	}

	// Every other SpecInfo field follows from the version alone, so a one-key
	// document reuses libopenapi's version handling without its slow parse.
	stub, err := json.Marshal(map[string]string{utils.OpenApi3: openAPI3.Value})
	if err != nil {
		return nil, err
	}
	info, err := datamodel.ExtractSpecInfoWithConfig(stub, cfg)
	if err != nil {
		return datamodel.ExtractSpecInfoWithConfig(spec, cfg)
	}
	info.SpecBytes = &spec
	info.RootNode = root
	info.NumLines = bytes.Count(spec, []byte{'\n'}) + 1
	info.OriginalIndentation = utils.DetermineWhitespaceLengthBytes(spec)
	if info.SpecFormat == datamodel.OAS31 || info.SpecFormat == datamodel.OAS32 {
		if _, self := utils.FindKeyNode("$self", root.Content); self != nil && self.Value != "" {
			info.Self = self.Value
		}
	}
	return info, nil
}
