package analyzer

import (
	"fmt"

	highbase "github.com/pb33f/libopenapi/datamodel/high/base"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	naming "github.com/giraffesyo/openapi-go-naming"
	"github.com/parallelworks/openapi-client-generator/internal/ir"
	"github.com/parallelworks/openapi-client-generator/internal/templates"
)

// Analyzer walks a parsed OpenAPI 3.1 model and produces IR types.
type Analyzer struct {
	model *v3high.Document
	namer *naming.Scope
	// typesBySchema tracks already-converted schema names to avoid duplicates.
	typesBySchema map[string]*ir.TypeDef
	// synthesized holds union types created for inline oneOf/anyOf schemas,
	// deduplicated on their variant set and discriminator.
	synthesized      []*ir.TypeDef
	synthesizedByKey map[string]*ir.TypeDef
	// multipartBodies holds the schema names a multipart request body refers to.
	multipartBodies map[string]bool
	// goNameBySchema maps every component schema to its Go type name, filled in
	// before any conversion so a reference to a schema that has not been converted
	// yet still resolves to the name it will end up with.
	goNameBySchema map[string]string
	// multiContentResponses counts responses offering more than one media type,
	// of which the generated method decodes one.
	multiContentResponses int
	// Keywords read and not expressible in a Go type, reported once per spec.
	prefixItemsSeen           bool
	dependentSchemasSeen      bool
	manyPatternPropertiesSeen bool
	// warnings collects what the generator had to skip, for the CLI to report.
	warnings []string
}

// New creates an Analyzer for the given high-level OpenAPI model.
func New(model *v3high.Document) *Analyzer {
	return &Analyzer{
		model:            model,
		namer:            naming.NewScope(templates.ReservedIdentifiers...),
		typesBySchema:    make(map[string]*ir.TypeDef),
		synthesizedByKey: make(map[string]*ir.TypeDef),
		goNameBySchema:   make(map[string]string),
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

	pkg.Servers = a.convertServers()

	// A multipart body's binary properties are generated as file parts rather
	// than as byte slices, which has to be settled before the schemas holding
	// them are converted.
	a.multipartBodies = a.collectMultipartBodySchemas()

	// Schema names are assigned next, and must avoid the identifiers the templates
	// derive from operations and error bodies.
	a.reserveDerivedNames()

	// Analyze component schemas.
	if err := a.analyzeComponentSchemas(pkg); err != nil {
		return nil, err
	}

	// Analyze operations.
	if err := a.analyzeOperations(pkg); err != nil {
		return nil, err
	}

	// Payloads the API sends rather than receives.
	a.analyzeWebhooks(pkg)

	// Analyze security schemes.
	if err := a.analyzeSecuritySchemes(pkg); err != nil {
		return nil, err
	}

	pkg.Warnings = append(pkg.Warnings, a.warnings...)

	if a.multiContentResponses > 0 {
		noun := "responses offer"
		if a.multiContentResponses == 1 {
			noun = "response offers"
		}
		pkg.Warnings = append(pkg.Warnings, fmt.Sprintf("%d %s more than one media type; each generated method requests and decodes one, preferring JSON", a.multiContentResponses, noun))
	}

	for _, note := range []struct {
		seen bool
		text string
	}{
		{a.prefixItemsSeen, "prefixItems describes a tuple, which has no Go shape a struct can hold, so those arrays stay slices of one element type"},
		{a.dependentSchemasSeen, "dependentSchemas makes a property's shape conditional, which a Go struct cannot express, so it is not enforced"},
		{a.manyPatternPropertiesSeen, "patternProperties with more than one pattern disagrees about what a key holds, so those maps take an any value type"},
	} {
		if note.seen {
			pkg.Warnings = append(pkg.Warnings, note.text)
		}
	}

	// Append union types synthesized for inline oneOf/anyOf schemas.
	pkg.Types = append(pkg.Types, a.synthesized...)

	// A spec is free to define a type in terms of itself; Go aliases are not, and
	// a struct may only do it through an indirection.
	breakAliasCycles(pkg.Types)
	breakStructCycles(pkg.Types)

	// A union whose variants all carry the same properties can expose them
	// directly, which depends on the variants' final field shapes.
	a.linkUnionBases(pkg)

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
		goName := a.namer.Unique(naming.Exported(name))
		a.goNameBySchema[name] = goName
		pending = append(pending, pendingSchema{name, goName, schema})
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
