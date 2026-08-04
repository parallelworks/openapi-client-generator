package analyzer

import (
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

func operationByName(t *testing.T, pkg *ir.Package, name string) *ir.OperationDef {
	t.Helper()
	for _, op := range pkg.Operations {
		if op.Name == name {
			return op
		}
	}
	t.Fatalf("operation %q not found", name)
	return nil
}

// A union used directly as a request or response body used to degrade to any,
// because the body path resolved its type without a naming hint.
func TestInlineUnionInBodyStaysTyped(t *testing.T) {
	pkg, typeMap := parseComplexSchemas(t)

	op := operationByName(t, pkg, "CreateShape")
	if op.RequestBody == nil {
		t.Fatal("CreateShape: missing request body")
	}
	if op.RequestBody.TypeName != "CreateShapeBody" {
		t.Errorf("request body type = %q, want CreateShapeBody", op.RequestBody.TypeName)
	}
	if op.SuccessResponse == nil || op.SuccessResponse.TypeName != "CreateShapeResponse" {
		t.Errorf("response type = %q, want CreateShapeResponse", op.SuccessResponse.TypeName)
	}

	for _, name := range []string{"CreateShapeBody", "CreateShapeResponse"} {
		td := typeMap[name]
		if td == nil {
			t.Fatalf("synthesized union %s not found", name)
		}
		if td.Kind != ir.TypeKindUnion {
			t.Errorf("%s kind = %v, want union", name, td.Kind)
		}
		if len(td.UnionTypes) != 2 {
			t.Errorf("%s has %d variants, want 2", name, len(td.UnionTypes))
		}
	}
}

// A titled union names itself, so the generated name does not move when the
// operation that reaches it first changes.
func TestTitledBodyUnionNamesItself(t *testing.T) {
	pkg, typeMap := parseComplexSchemas(t)

	op := operationByName(t, pkg, "CreateTitledShape")
	if op.RequestBody == nil {
		t.Fatal("CreateTitledShape: missing request body")
	}
	if op.RequestBody.TypeName != "NamedShape" {
		t.Errorf("request body type = %q, want NamedShape from the schema title", op.RequestBody.TypeName)
	}
	if typeMap["NamedShape"] == nil {
		t.Fatal("synthesized union NamedShape not found")
	}
	if typeMap["CreateTitledShapeBody"] != nil {
		t.Error("the operation hint should not be used when the schema is titled")
	}
}
