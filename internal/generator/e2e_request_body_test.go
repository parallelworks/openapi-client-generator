package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

const requestBodySpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /recipes/{slug}/image:
    put:
      operationId: updateRecipeImage
      parameters: [{ name: slug, in: path, required: true, schema: { type: string } }]
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema: { $ref: "#/components/schemas/ImageUpload" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/UploadResult" }
  /token:
    post:
      operationId: createToken
      requestBody:
        required: true
        content:
          application/x-www-form-urlencoded:
            schema: { $ref: "#/components/schemas/TokenRequest" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/UploadResult" }
  /recipes:
    post:
      operationId: createRecipe
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/TokenRequest" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/UploadResult" }
  /notes:
    post:
      operationId: createNote
      requestBody:
        required: true
        content:
          application/xml:
            schema: { $ref: "#/components/schemas/ImageMeta" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/UploadResult" }
  /labels:
    post:
      operationId: createLabel
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema: { $ref: "#/components/schemas/Label" }
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: { $ref: "#/components/schemas/UploadResult" }
components:
  schemas:
    ImageUpload:
      type: object
      required: [image, extension]
      properties:
        image: { type: string, format: binary }
        extension: { type: string }
        attempt: { type: integer, format: int32 }
        tags: { type: array, items: { type: string } }
        meta: { $ref: "#/components/schemas/ImageMeta" }
        attachments: { type: array, items: { type: string, format: binary } }
        signature: { type: string, format: byte }
    ImageMeta:
      type: object
      required: [source]
      properties:
        source: { type: string }
    TokenRequest:
      type: object
      required: [username, password]
      properties:
        username: { type: string }
        password: { type: string }
        scopes: { type: array, items: { type: string } }
        client: { $ref: "#/components/schemas/ImageMeta" }
    Label:
      type: object
      required: [name]
      properties:
        name: { type: string }
      additionalProperties: { type: string }
    UploadResult:
      type: object
      properties:
        ok: { type: boolean }
`

// TestE2E_RequestBodyContentTypes covers issue #17: an operation whose spec
// declares multipart/form-data used to marshal its body as JSON and send it with
// a Content-Type of application/json. The generated client is compiled and RUN
// against a real server that parses the request, so a template that encodes the
// wrong thing fails here rather than at a user's API.
func TestE2E_RequestBodyContentTypes(t *testing.T) {
	specDir := t.TempDir()
	specPath := filepath.Join(specDir, "spec.yaml")
	if err := os.WriteFile(specPath, []byte(requestBodySpec), 0o644); err != nil {
		t.Fatalf("writing spec: %v", err)
	}

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	pkg, err := analyzer.New(result.Model).Analyze("uploads")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New generator: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	tmpDir := t.TempDir()
	goMod := []byte("module requestbody-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}

	runtimeTest := []byte(`package uploads

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type capture struct {
	contentType string
	body        []byte
}

// serve records the one request the client makes and answers it with an empty
// JSON object.
func serve(t *testing.T, got *capture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		got.contentType = r.Header.Get("Content-Type")
		got.body = body
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(` + "`" + `{"ok":true}` + "`" + `))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMultipartBodyIsSentAsMultipart(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	res, err := client.UpdateRecipeImage(t.Context(), "carrot-cake", ImageUpload{
		Image: FormFile{
			Filename:    "carrot cake.png",
			ContentType: "image/png",
			Content:     []byte("\x89PNG\r\n binary \x00 payload"),
		},
		Extension:   "png",
		Attempt:     ptr(int32(2)),
		Tags:        []string{"dinner", "quick"},
		Meta:        &ImageMeta{Source: "phone"},
		Attachments: []FormFile{{Filename: "notes.txt", Content: []byte("first")}, {Content: []byte("second")}},
		Signature:   []byte("sig"),
	})
	if err != nil {
		t.Fatalf("UpdateRecipeImage: %v", err)
	}
	if res == nil || res.Ok == nil || !*res.Ok {
		t.Errorf("response = %+v, want ok=true", res)
	}

	mediaType, params, err := mime.ParseMediaType(got.contentType)
	if err != nil {
		t.Fatalf("parsing Content-Type %q: %v", got.contentType, err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q, want multipart/form-data", mediaType)
	}
	if params["boundary"] == "" {
		t.Fatal("Content-Type carries no boundary")
	}

	// Parse the body the way a real server would.
	req := httptest.NewRequest("PUT", "/", strings.NewReader(string(got.body)))
	req.Header.Set("Content-Type", got.contentType)
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}

	fileHeaders := req.MultipartForm.File["image"]
	if len(fileHeaders) != 1 {
		t.Fatalf("image file parts = %d, want 1", len(fileHeaders))
	}
	if name := fileHeaders[0].Filename; name != "carrot cake.png" {
		t.Errorf("image filename = %q, want carrot cake.png", name)
	}
	if ct := fileHeaders[0].Header.Get("Content-Type"); ct != "image/png" {
		t.Errorf("image part Content-Type = %q, want image/png", ct)
	}
	f, err := fileHeaders[0].Open()
	if err != nil {
		t.Fatalf("opening image part: %v", err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("reading image part: %v", err)
	}
	if string(content) != "\x89PNG\r\n binary \x00 payload" {
		t.Errorf("image part = %q, want the raw bytes verbatim", content)
	}

	values := req.MultipartForm.Value
	if got := values["extension"]; len(got) != 1 || got[0] != "png" {
		t.Errorf("extension part = %v, want [png]", got)
	}
	if got := values["attempt"]; len(got) != 1 || got[0] != "2" {
		t.Errorf("attempt part = %v, want [2]", got)
	}
	if got := values["tags"]; len(got) != 2 || got[0] != "dinner" || got[1] != "quick" {
		t.Errorf("tags parts = %v, want one part per element", got)
	}
	if got := values["meta"]; len(got) != 1 || got[0] != ` + "`" + `{"source":"phone"}` + "`" + ` {
		t.Errorf("meta part = %v, want the object as JSON", got)
	}

	// format: byte is base64 text, not an upload, so it is a value part.
	if got := values["signature"]; len(got) != 1 || got[0] != "c2ln" {
		t.Errorf("signature part = %v, want the base64 text [c2ln]", got)
	}
	if _, ok := req.MultipartForm.File["signature"]; ok {
		t.Error("format: byte was sent as a file part")
	}

	// An array of files becomes one file part per element, and a file with no
	// name of its own falls back to the property name.
	attachments := req.MultipartForm.File["attachments"]
	if len(attachments) != 2 {
		t.Fatalf("attachment parts = %d, want 2", len(attachments))
	}
	if attachments[0].Filename != "notes.txt" {
		t.Errorf("attachment[0] filename = %q, want notes.txt", attachments[0].Filename)
	}
	if attachments[1].Filename != "attachments" {
		t.Errorf("attachment[1] filename = %q, want the property name", attachments[1].Filename)
	}
	if ct := attachments[1].Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("attachment[1] Content-Type = %q, want application/octet-stream", ct)
	}
}

func TestOptionalMultipartFieldsAreOmitted(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	if _, err := client.UpdateRecipeImage(t.Context(), "carrot-cake", ImageUpload{
		Image:     FormFile{Content: []byte("data")},
		Extension: "png",
	}); err != nil {
		t.Fatalf("UpdateRecipeImage: %v", err)
	}

	req := httptest.NewRequest("PUT", "/", strings.NewReader(string(got.body)))
	req.Header.Set("Content-Type", got.contentType)
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	for _, unset := range []string{"attempt", "meta"} {
		if _, ok := req.MultipartForm.Value[unset]; ok {
			t.Errorf("unset optional field %q was sent", unset)
		}
	}
	if _, ok := req.MultipartForm.File["attachments"]; ok {
		t.Error("unset optional file field was sent")
	}
	// A file with no name of its own is still a file part, not a text part.
	if files := req.MultipartForm.File["image"]; len(files) != 1 || files[0].Filename != "image" {
		t.Errorf("image parts = %v, want one named after the property", files)
	}
}

func TestFormURLEncodedBodyIsSentAsFormValues(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	if _, err := client.CreateToken(t.Context(), TokenRequest{
		Username: "ada",
		Password: "a b&c=d",
		Scopes:   []string{"read", "write"},
		Client:   &ImageMeta{Source: "cli"},
	}); err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	if mediaType, _, err := mime.ParseMediaType(got.contentType); err != nil || mediaType != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", got.contentType)
	}
	values, err := url.ParseQuery(string(got.body))
	if err != nil {
		t.Fatalf("parsing form body %q: %v", got.body, err)
	}
	if values.Get("username") != "ada" {
		t.Errorf("username = %q, want ada", values.Get("username"))
	}
	if values.Get("password") != "a b&c=d" {
		t.Errorf("password = %q, want the value escaped, not split", values.Get("password"))
	}
	// An array is one pair per element (form/explode, the OpenAPI default), not a
	// single comma-joined value.
	if got := values["scopes"]; len(got) != 2 || got[0] != "read" || got[1] != "write" {
		t.Errorf("scopes = %v, want one pair per element", got)
	}
	if got := values.Get("client"); got != ` + "`" + `{"source":"cli"}` + "`" + ` {
		t.Errorf("client = %q, want the object as JSON", got)
	}
	// A space is '+' in x-www-form-urlencoded, not the query string's %20.
	if !strings.Contains(string(got.body), "password=a+b") {
		t.Errorf("body = %q, want a space encoded as '+'", got.body)
	}
}

// TestSharedBodySchemaSentBothWays covers a schema a spec offers as both JSON and
// multipart: the generated code has to compile and each operation has to send the
// media type it declared.
func TestSharedBodySchemaSentBothWays(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	if _, err := client.CreateRecipe(t.Context(), TokenRequest{Username: "ada", Password: "p"}); err != nil {
		t.Fatalf("CreateRecipe: %v", err)
	}
	if mediaType, _, _ := mime.ParseMediaType(got.contentType); mediaType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got.contentType)
	}
}

func TestJSONBodyStillSentAsJSON(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	if _, err := client.CreateRecipe(t.Context(), TokenRequest{
		Username: "ada",
		Password: "secret",
	}); err != nil {
		t.Fatalf("CreateRecipe: %v", err)
	}

	if mediaType, _, err := mime.ParseMediaType(got.contentType); err != nil || mediaType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got.contentType)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got.body, &decoded); err != nil {
		t.Fatalf("body %q is not JSON: %v", got.body, err)
	}
	if decoded["username"] != "ada" || decoded["password"] != "secret" {
		t.Errorf("body = %v, want the JSON object", decoded)
	}
}

// TestUnencodableMediaTypeTakesRawBytes covers a body whose media type the client
// cannot build from the schema: it must take raw bytes and send them verbatim
// rather than accept a struct and marshal it as JSON under an XML Content-Type.
func TestUnencodableMediaTypeTakesRawBytes(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	if _, err := client.CreateNote(t.Context(), []byte("<meta><source>phone</source></meta>")); err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if mediaType, _, _ := mime.ParseMediaType(got.contentType); mediaType != "application/xml" {
		t.Errorf("Content-Type = %q, want application/xml", got.contentType)
	}
	if string(got.body) != "<meta><source>phone</source></meta>" {
		t.Errorf("body = %q, want the bytes verbatim", got.body)
	}
}

// TestCatchAllPropertiesReachTheWire covers additionalProperties on a multipart
// body: the catch-all map is tagged json:"-" so that encoding/json inlines it by
// hand, and the form encoders have to inline it too rather than drop it.
func TestCatchAllPropertiesReachTheWire(t *testing.T) {
	var got capture
	srv := serve(t, &got)

	client := NewClient(srv.URL)
	if _, err := client.CreateLabel(t.Context(), Label{
		Name:                 "ada",
		AdditionalProperties: map[string]string{"team": "core"},
	}); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}

	req := httptest.NewRequest("POST", "/", strings.NewReader(string(got.body)))
	req.Header.Set("Content-Type", got.contentType)
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm: %v", err)
	}
	if v := req.MultipartForm.Value["team"]; len(v) != 1 || v[0] != "core" {
		t.Errorf("team = %v, want [core]: an undeclared property was dropped", v)
	}
	if _, ok := req.MultipartForm.Value["-"]; ok {
		t.Error("the catch-all map was sent under a literal \"-\" part name")
	}
}

func ptr[T any](v T) *T { return &v }
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "request_body_test.go"), runtimeTest, 0o644); err != nil {
		t.Fatalf("writing runtime test: %v", err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		for _, f := range files {
			if f.Name == "helpers.go" || f.Name == "operations.go" || f.Name == "types.go" {
				t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
			}
		}
		t.Fatalf("go test on generated code failed: %v\n%s", err, string(output))
	}
	t.Logf("request body encoding test passed:\n%s", string(output))
}
