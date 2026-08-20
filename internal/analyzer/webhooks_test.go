package analyzer

import (
	"strings"
	"testing"
)

const webhookAnalyzerSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
webhooks:
  petCreated:
    post:
      requestBody:
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Pet" }
  bothWays:
    post:
      requestBody:
        content:
          application/json:
            schema: { $ref: "#/components/schemas/Pet" }
    put:
      requestBody:
        content:
          application/json:
            schema: { type: object, properties: { id: { type: string } } }
  ping:
    get: { description: no body }
  binaryOnly:
    post:
      requestBody:
        content:
          application/octet-stream:
            schema: { type: string, format: binary }
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
`

func TestWebhooks_PayloadsAndNames(t *testing.T) {
	pkg, _ := analyzeSpec(t, webhookAnalyzerSpec)

	byName := map[string]string{}
	for _, w := range pkg.Webhooks {
		byName[w.Name] = w.GoName + ":" + w.PayloadType
	}

	if got := byName["petCreated"]; got != "PetCreated:Pet" {
		t.Errorf("petCreated = %q, want PetCreated:Pet", got)
	}
	// A webhook sent under two methods carries two payloads, so each names itself.
	if got := byName["bothWays.POST"]; got != "BothWaysPost:Pet" {
		t.Errorf("bothWays.POST = %q", got)
	}
	if got := byName["bothWays.PUT"]; got != "BothWaysPut:BothWaysPutPayload" {
		t.Errorf("bothWays.PUT = %q, want its inline body named", got)
	}
	// A callback dispatches on the operation and callback name, since it arrives
	// at a URL the caller registered rather than under a name of its own.
	if got := byName["Subscribe.onData"]; got != "SubscribeOnData:Pet" {
		t.Errorf("Subscribe.onData = %q, want SubscribeOnData:Pet", got)
	}
	for _, w := range pkg.Webhooks {
		if (w.Name == "Subscribe.onData") != w.Callback {
			t.Errorf("%s Callback = %v", w.Name, w.Callback)
		}
	}
}

// A body the generator cannot decode gets no parse function, and says why.
func TestWebhooks_UndecodableBodiesWarn(t *testing.T) {
	pkg, _ := analyzeSpec(t, webhookAnalyzerSpec)

	for _, name := range []string{"ping", "binaryOnly"} {
		for _, w := range pkg.Webhooks {
			if w.Name == name {
				t.Errorf("%s should have no parse function: %+v", name, w)
			}
		}
		var warned bool
		for _, msg := range pkg.Warnings {
			if strings.Contains(msg, name) {
				warned = true
			}
		}
		if !warned {
			t.Errorf("no warning naming %q: %v", name, pkg.Warnings)
		}
	}
}
