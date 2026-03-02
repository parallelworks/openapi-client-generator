package analyzer

import (
	"fmt"

	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/naming"
)

// Analyzer walks a parsed OpenAPI 3.1 model and produces IR types.
type Analyzer struct {
	model *v3high.Document
	namer *naming.Namer
	// typesBySchema tracks already-converted schema names to avoid duplicates.
	typesBySchema map[string]*ir.TypeDef
}

// New creates an Analyzer for the given high-level OpenAPI model.
func New(model *v3high.Document) *Analyzer {
	return &Analyzer{
		model:         model,
		namer:         naming.NewNamer(),
		typesBySchema: make(map[string]*ir.TypeDef),
	}
}

// Analyze walks the OpenAPI model and returns a complete IR Package.
func (a *Analyzer) Analyze(packageName string) (*ir.Package, error) {
	pkg := &ir.Package{
		Name: packageName,
	}

	// Extract API info.
	if a.model.Info != nil {
		pkg.Info = &ir.APIInfo{
			Title:       a.model.Info.Title,
			Version:     a.model.Info.Version,
			Description: a.model.Info.Description,
		}
	}

	// Extract server URLs.
	for _, server := range a.model.Servers {
		if server != nil {
			pkg.ServerURLs = append(pkg.ServerURLs, server.URL)
		}
	}

	// Analyze component schemas.
	if err := a.analyzeComponentSchemas(pkg); err != nil {
		return nil, err
	}

	// Analyze operations.
	if err := a.analyzeOperations(pkg); err != nil {
		return nil, err
	}

	// Analyze security schemes.
	if err := a.analyzeSecuritySchemes(pkg); err != nil {
		return nil, err
	}

	// Detect paginated operations.
	a.detectPagination(pkg)

	return pkg, nil
}

// analyzeComponentSchemas walks Components.Schemas and populates pkg.Types.
func (a *Analyzer) analyzeComponentSchemas(pkg *ir.Package) error {
	if a.model.Components == nil || a.model.Components.Schemas == nil {
		return nil
	}

	for name, schemaProxy := range a.model.Components.Schemas.FromOldest() {
		if _, exists := a.typesBySchema[name]; exists {
			continue
		}

		schema, err := schemaProxy.BuildSchema()
		if err != nil {
			return fmt.Errorf("building schema %q: %w", name, err)
		}
		if schema == nil {
			continue
		}

		goName := a.namer.RegisterName(naming.ToGoName(name))
		td, err := a.convertSchema(goName, name, schema)
		if err != nil {
			return fmt.Errorf("converting schema %q: %w", name, err)
		}
		if td != nil {
			a.typesBySchema[name] = td
		}
	}

	// Collect types in spec-defined order.
	for name := range a.model.Components.Schemas.FromOldest() {
		if td, ok := a.typesBySchema[name]; ok {
			pkg.Types = append(pkg.Types, td)
		}
	}

	return nil
}
