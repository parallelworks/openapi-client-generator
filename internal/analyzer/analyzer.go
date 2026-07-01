package analyzer

import (
	"fmt"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
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
	// synthesized holds union types created for inline oneOf/anyOf schemas,
	// deduplicated on their variant set and discriminator.
	synthesized      []*ir.TypeDef
	synthesizedByKey map[string]*ir.TypeDef
}

// New creates an Analyzer for the given high-level OpenAPI model.
func New(model *v3high.Document) *Analyzer {
	return &Analyzer{
		model:            model,
		namer:            naming.NewNamer(),
		typesBySchema:    make(map[string]*ir.TypeDef),
		synthesizedByKey: make(map[string]*ir.TypeDef),
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

	// Append union types synthesized for inline oneOf/anyOf schemas.
	pkg.Types = append(pkg.Types, a.synthesized...)

	// Detect paginated operations.
	a.detectPagination(pkg)

	return pkg, nil
}

// analyzeComponentSchemas walks Components.Schemas and populates pkg.Types.
func (a *Analyzer) analyzeComponentSchemas(pkg *ir.Package) error {
	if a.model.Components == nil || a.model.Components.Schemas == nil {
		return nil
	}

	// Register every component's Go type name before converting any schema, so an
	// enum constant that sanitizes to a schema-named type (e.g. Color.red -> const
	// ColorRed vs a ColorRed schema) yields the numeric suffix to the const, not to
	// the user's public API type.
	type pendingSchema struct {
		name   string
		goName string
		schema *highbase.Schema
	}
	var pending []pendingSchema
	for name, schemaProxy := range a.model.Components.Schemas.FromOldest() {
		schema, err := schemaProxy.BuildSchema()
		if err != nil {
			return fmt.Errorf("building schema %q: %w", name, err)
		}
		if schema == nil {
			continue
		}
		pending = append(pending, pendingSchema{name, a.namer.RegisterName(naming.ToGoName(name)), schema})
	}

	for _, p := range pending {
		td, err := a.convertSchema(p.goName, p.name, p.schema)
		if err != nil {
			return fmt.Errorf("converting schema %q: %w", p.name, err)
		}
		if td != nil {
			a.typesBySchema[p.name] = td
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
