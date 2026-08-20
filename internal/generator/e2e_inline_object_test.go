package generator

import "testing"

const inlineObjectAPISpec = `openapi: 3.1.0
info: { title: orders, version: "1" }
paths:
  /orders:
    post:
      operationId: createOrder
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Order" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Order" }
components:
  schemas:
    Order:
      type: object
      properties:
        id: { type: string }
        customer:
          type: object
          properties:
            name: { type: string }
            address:
              type: object
              properties:
                city: { type: string }
              required: [city]
          required: [name]
        lines:
          type: array
          items:
            type: object
            properties:
              sku: { type: string }
              qty: { type: integer }
            required: [sku, qty]
      required: [id, customer]
`

// TestE2E_InlineObjectsAreConstructible covers a nested object written inline,
// which resolved to any: the properties it declares were unreadable without a
// type assertion and unwritable without building a map by hand.
func TestE2E_InlineObjectsAreConstructible(t *testing.T) {
	files, _ := generateFromSpec(t, inlineObjectAPISpec, "ordersapi")

	runGeneratedWireTest(t, files, "inlineobject", `package ordersapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The value has to be constructible in Go, which a map behind an any is not.
func TestConstructAndSend(t *testing.T) {
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		w.Write(got)
	}))
	defer srv.Close()

	order := Order{
		ID: "o-1",
		Customer: OrderCustomer{
			Name:    "Ada",
			Address: &OrderCustomerAddress{City: "London"},
		},
		Lines: []OrderLinesItem{{Sku: "abc", Qty: 2}},
	}

	back, err := NewClient(srv.URL).CreateOrder(t.Context(), order)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(got, &sent); err != nil {
		t.Fatalf("request body: %v", err)
	}
	customer, ok := sent["customer"].(map[string]any)
	if !ok || customer["name"] != "Ada" {
		t.Errorf("customer on the wire = %#v", sent["customer"])
	}
	if addr, ok := customer["address"].(map[string]any); !ok || addr["city"] != "London" {
		t.Errorf("address on the wire = %#v", customer["address"])
	}

	// And the same shape decodes back into the same types.
	if back.Customer.Address == nil || back.Customer.Address.City != "London" {
		t.Errorf("decoded address = %+v", back.Customer.Address)
	}
	if len(back.Lines) != 1 || back.Lines[0].Sku != "abc" || back.Lines[0].Qty != 2 {
		t.Errorf("decoded lines = %+v", back.Lines)
	}
}

// An optional inline object is a pointer, so absent stays distinguishable from
// present-and-empty.
func TestAbsentInlineObjectIsNil(t *testing.T) {
	var o Order
	if err := json.Unmarshal([]byte(`+"`"+`{"id":"o-2","customer":{"name":"Bo"}}`+"`"+`), &o); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if o.Customer.Address != nil {
		t.Errorf("Address = %+v, want nil", o.Customer.Address)
	}
	if o.Customer.Name != "Bo" {
		t.Errorf("Name = %q, want Bo", o.Customer.Name)
	}
}
`)
}
