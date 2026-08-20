package generator

import "testing"

const webhookSpec = `openapi: 3.1.0
info: { title: hooks, version: "1" }
webhooks:
  petCreated:
    post:
      description: Sent when a pet is added.
      requestBody:
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Pet" }
  inventoryChanged:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                sku: { type: string }
                count: { type: integer }
              required: [sku, count]
paths:
  /subscribe:
    post:
      operationId: subscribe
      responses:
        "202": { description: accepted }
      callbacks:
        onData:
          "{$request.body#/callbackUrl}":
            post:
              requestBody:
                content:
                  application/json:
                    schema: { $ref: "#/components/schemas/Pet" }
components:
  schemas:
    Pet:
      type: object
      properties:
        name: { type: string }
      required: [name]
`

// TestE2E_WebhookPayloadsParse covers the half of a spec the client receives
// rather than sends. None of it was generated, so a receiver hand-wrote the
// structs and the dispatch.
func TestE2E_WebhookPayloadsParse(t *testing.T) {
	files, _ := generateFromSpec(t, webhookSpec, "hooksapi")

	runGeneratedWireTest(t, files, "webhooks", `package hooksapi

import (
	"slices"
	"strings"
	"testing"
)

func TestPerWebhookParseIsTyped(t *testing.T) {
	pet, err := ParsePetCreatedWebhook([]byte(`+"`"+`{"name":"Rex"}`+"`"+`))
	if err != nil {
		t.Fatalf("ParsePetCreatedWebhook: %v", err)
	}
	// No assertion: the function returns the declared type.
	if pet.Name != "Rex" {
		t.Errorf("name = %q, want Rex", pet.Name)
	}

	// An inline body still names a type rather than decoding into a map.
	inv, err := ParseInventoryChangedWebhook([]byte(`+"`"+`{"sku":"abc","count":3}`+"`"+`))
	if err != nil {
		t.Fatalf("ParseInventoryChangedWebhook: %v", err)
	}
	if inv.Sku != "abc" || inv.Count != 3 {
		t.Errorf("payload = %+v, want the decoded body", inv)
	}
}

func TestDispatchByName(t *testing.T) {
	payload, err := ParseWebhook("petCreated", []byte(`+"`"+`{"name":"Momo"}`+"`"+`))
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	switch p := payload.(type) {
	case Pet:
		if p.Name != "Momo" {
			t.Errorf("name = %q, want Momo", p.Name)
		}
	default:
		t.Fatalf("payload = %T, want Pet", payload)
	}

	if !slices.Contains(WebhookNames, "inventoryChanged") {
		t.Errorf("WebhookNames = %v, want every declared webhook", WebhookNames)
	}
}

// A name the spec does not declare is an error rather than a nil payload the
// caller has to notice.
func TestUnknownWebhookIsAnError(t *testing.T) {
	if _, err := ParseWebhook("somethingElse", []byte("{}")); err == nil {
		t.Fatal("expected an error for an undeclared webhook")
	} else if !strings.Contains(err.Error(), "somethingElse") {
		t.Errorf("error = %q, want it to name the webhook", err)
	}
}

func TestMalformedBodyReportsTheWebhook(t *testing.T) {
	_, err := ParsePetCreatedWebhook([]byte("not json"))
	if err == nil {
		t.Fatal("expected a decode error")
	}
	if !strings.Contains(err.Error(), "petCreated") {
		t.Errorf("error = %q, want it to name the webhook", err)
	}
}

// A callback arrives at a URL the caller registered, so it dispatches on the
// operation and callback name.
func TestCallbackPayloadsParse(t *testing.T) {
	pet, err := ParseSubscribeOnDataCallback([]byte(`+"`"+`{"name":"Ada"}`+"`"+`))
	if err != nil {
		t.Fatalf("ParseSubscribeOnDataCallback: %v", err)
	}
	if pet.Name != "Ada" {
		t.Errorf("name = %q, want Ada", pet.Name)
	}

	payload, err := ParseCallback("Subscribe.onData", []byte(`+"`"+`{"name":"Ada"}`+"`"+`))
	if err != nil {
		t.Fatalf("ParseCallback: %v", err)
	}
	if _, ok := payload.(Pet); !ok {
		t.Errorf("payload = %T, want Pet", payload)
	}
	if !slices.Contains(CallbackNames, "Subscribe.onData") {
		t.Errorf("CallbackNames = %v", CallbackNames)
	}
}
`)
}
