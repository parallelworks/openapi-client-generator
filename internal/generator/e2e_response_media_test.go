package generator

import "testing"

const responseMediaSpec = `openapi: 3.1.0
info: { title: files, version: "1" }
paths:
  /file:
    get:
      operationId: downloadFile
      responses:
        "200":
          description: ok
          content:
            application/octet-stream:
              schema: { type: string, format: binary }
  /notes:
    get:
      operationId: getNote
      responses:
        "200":
          description: ok
          content:
            text/plain:
              schema: { type: string }
  /docs:
    get:
      operationId: getDoc
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Doc" }
components:
  schemas:
    Doc:
      type: object
      properties:
        id: { type: string }
`

// TestE2E_ResponseBodyByMediaType covers bodies that are the value rather than a
// document to parse. A binary download was run through json.Unmarshal, so it
// failed on the first byte and no file could be fetched at all.
func TestE2E_ResponseBodyByMediaType(t *testing.T) {
	files, _ := generateFromSpec(t, responseMediaSpec, "filesapi")

	runGeneratedWireTest(t, files, "responsemedia", `package filesapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, contentType string, body []byte) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL)
}

func TestBinaryBodyArrivesVerbatim(t *testing.T) {
	payload := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0xff}
	got, err := serve(t, "application/octet-stream", payload).DownloadFile(t.Context())
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if got == nil || !bytes.Equal(*got, payload) {
		t.Errorf("body = %v, want the bytes the server sent", got)
	}
}

func TestTextBodyArrivesVerbatim(t *testing.T) {
	got, err := serve(t, "text/plain", []byte("not json {")).GetNote(t.Context())
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got == nil || *got != "not json {" {
		t.Errorf("body = %v, want the text the server sent", got)
	}
}

// A server that labels JSON as something else is common enough to try anyway.
func TestMislabeledJSONStillDecodes(t *testing.T) {
	doc, err := serve(t, "text/html", []byte(`+"`"+`{"id":"d-1"}`+"`"+`)).GetDoc(t.Context())
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if doc == nil || doc.ID == nil || *doc.ID != "d-1" {
		t.Errorf("doc = %+v, want the decoded body", doc)
	}
}

// When it is not JSON either, the error names both media types instead of
// reporting a parse failure with no context.
func TestMediaTypeMismatchNamesBoth(t *testing.T) {
	_, err := serve(t, "application/xml", []byte("<doc><id>d-1</id></doc>")).GetDoc(t.Context())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "application/json") || !strings.Contains(err.Error(), "application/xml") {
		t.Errorf("error = %q, want it to name what was asked for and what arrived", err)
	}
}
`)
}
