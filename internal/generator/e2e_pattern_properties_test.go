package generator

import "testing"

const patternPropertiesAPISpec = `openapi: 3.1.0
info: { title: bags, version: "1" }
paths:
  /bags:
    get:
      operationId: getBag
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Bag" }
components:
  schemas:
    Bag:
      type: object
      properties:
        name: { type: string }
        extensions:
          type: object
          patternProperties:
            "^x-": { type: string }
`

// TestE2E_PatternPropertiesTypesTheMap covers a bag of keys a single pattern
// admits, which resolved to map[string]any and made every read a type assertion.
func TestE2E_PatternPropertiesTypesTheMap(t *testing.T) {
	files, _ := generateFromSpec(t, patternPropertiesAPISpec, "bagsapi")

	runGeneratedWireTest(t, files, "patternprops", `package bagsapi

import (
	"encoding/json"
	"testing"
)

func TestExtensionsDecodeAsStrings(t *testing.T) {
	var bag Bag
	payload := `+"`"+`{"name":"b","extensions":{"x-owner":"ada","x-team":"core"}}`+"`"+`
	if err := json.Unmarshal([]byte(payload), &bag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The value type is string, so this is a plain read rather than an assertion.
	if bag.Extensions["x-owner"] != "ada" {
		t.Errorf("x-owner = %q, want ada", bag.Extensions["x-owner"])
	}
	if len(bag.Extensions) != 2 {
		t.Errorf("extensions = %v, want both keys", bag.Extensions)
	}

	bag.Extensions["x-added"] = "later"
	out, err := json.Marshal(bag)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Bag
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if back.Extensions["x-added"] != "later" {
		t.Errorf("round trip lost the added key: %v", back.Extensions)
	}
}
`)
}
