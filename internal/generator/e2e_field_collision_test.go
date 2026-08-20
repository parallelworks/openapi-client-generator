package generator

import "testing"

const fieldCollisionAPISpec = `openapi: 3.1.0
info: { title: coll, version: "1" }
paths:
  /things:
    get:
      operationId: getThing
      responses:
        "200":
          description: ok
          content:
            application/json: { schema: { $ref: "#/components/schemas/Thing" } }
components:
  schemas:
    Thing:
      type: object
      properties:
        user-id: { type: string }
        user_id: { type: string }
        userId: { type: string }
      required: [user-id, user_id, userId]
`

// TestE2E_CollidingPropertyNames covers property names that differ on the wire
// and normalize to one Go identifier. The struct redeclared the field, so the
// whole generated package failed to compile.
func TestE2E_CollidingPropertyNames(t *testing.T) {
	files, _ := generateFromSpec(t, fieldCollisionAPISpec, "collapi")

	runGeneratedWireTest(t, files, "fieldcollision", `package collapi

import (
	"encoding/json"
	"testing"
)

// Each field still carries the property it came from, so a payload that uses
// all three round-trips with them kept apart.
func TestCollidingPropertiesStayDistinct(t *testing.T) {
	var thing Thing
	payload := `+"`"+`{"user-id":"dash","user_id":"under","userId":"camel"}`+"`"+`
	if err := json.Unmarshal([]byte(payload), &thing); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if thing.UserID != "dash" || thing.UserID2 != "under" || thing.UserID3 != "camel" {
		t.Fatalf("decoded %+v, want each property in its own field", thing)
	}

	out, err := json.Marshal(thing)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]string
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	for key, want := range map[string]string{"user-id": "dash", "user_id": "under", "userId": "camel"} {
		if back[key] != want {
			t.Errorf("%s = %q, want %q", key, back[key], want)
		}
	}
}
`)
}
