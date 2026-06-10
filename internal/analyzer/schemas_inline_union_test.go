package analyzer

import (
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

func TestInlineOneOf_AdditionalPropertiesSynthesizesUnion(t *testing.T) {
	_, typeMap := parseComplexSchemas(t)

	sc := typeMap["ShapeCollection"]
	if sc == nil {
		t.Fatal("ShapeCollection type not found")
	}
	fields := make(map[string]*ir.Field)
	for _, f := range sc.Fields {
		fields[f.JSONName] = f
	}

	shapes := fields["shapes"]
	if shapes == nil {
		t.Fatal("ShapeCollection: missing shapes field")
	}
	if shapes.Type != "map[string]ShapeCollectionShapesValue" {
		t.Errorf("shapes type = %q, want map[string]ShapeCollectionShapesValue", shapes.Type)
	}

	// An identical inline union elsewhere reuses the synthesized type.
	backup := fields["backupShapes"]
	if backup == nil {
		t.Fatal("ShapeCollection: missing backupShapes field")
	}
	if backup.Type != "map[string]ShapeCollectionShapesValue" {
		t.Errorf("backupShapes type = %q, want the deduplicated map[string]ShapeCollectionShapesValue", backup.Type)
	}

	union := typeMap["ShapeCollectionShapesValue"]
	if union == nil {
		t.Fatal("synthesized union ShapeCollectionShapesValue not found in package types")
	}
	if union.Kind != ir.TypeKindUnion {
		t.Errorf("union kind = %v, want union", union.Kind)
	}
	if len(union.UnionTypes) != 2 {
		t.Fatalf("union variants = %d, want 2", len(union.UnionTypes))
	}
	if union.Discriminator == nil {
		t.Fatal("synthesized union has no discriminator")
	}
	if union.Discriminator.PropertyName != "shapeType" {
		t.Errorf("discriminator property = %q, want shapeType", union.Discriminator.PropertyName)
	}
	if union.Discriminator.Mapping["circle"] != "Circle" || union.Discriminator.Mapping["rectangle"] != "Rectangle" {
		t.Errorf("discriminator mapping = %v, want circle->Circle, rectangle->Rectangle", union.Discriminator.Mapping)
	}
}
