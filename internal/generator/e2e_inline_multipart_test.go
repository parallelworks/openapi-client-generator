package generator

import "testing"

const inlineMultipartSpec = `openapi: 3.1.0
info: { title: files, version: "1" }
paths:
  /upload:
    post:
      operationId: upload
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file: { type: string, format: binary }
                name: { type: string }
                meta:
                  type: object
                  properties:
                    thumbnail: { type: string, format: binary }
              required: [file]
      responses:
        "204": { description: ok }
  /upload-ref:
    post:
      operationId: uploadRef
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema: { $ref: "#/components/schemas/UploadForm" }
      responses:
        "204": { description: ok }
components:
  schemas:
    UploadForm:
      type: object
      properties:
        file: { type: string, format: binary }
      required: [file]
`

// TestE2E_InlineMultipartUpload covers a multipart body written inline. Only a
// $ref'd body was treated as multipart, so an inline one sent its file as base64
// text in an ordinary field and no server could read it as an upload.
func TestE2E_InlineMultipartUpload(t *testing.T) {
	files, _ := generateFromSpec(t, inlineMultipartSpec, "filesapi")

	runGeneratedWireTest(t, files, "inlinemultipart", `package filesapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type upload struct {
	filename    string
	contentType string
	content     string
	fields      map[string]string
}

func serve(t *testing.T) (*Client, *upload) {
	t.Helper()
	got := &upload{fields: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		for k, v := range r.MultipartForm.Value {
			got.fields[k] = v[0]
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		defer f.Close()
		got.filename = hdr.Filename
		got.contentType = hdr.Header.Get("Content-Type")
		b, _ := io.ReadAll(f)
		got.content = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), got
}

func TestInlineBodyUploadsAFile(t *testing.T) {
	client, got := serve(t)

	name := "readme"
	err := client.Upload(t.Context(), UploadBody{
		File: FormFile{Filename: "a.txt", ContentType: "text/plain", Content: []byte("hello")},
		Name: &name,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if got.filename != "a.txt" || got.content != "hello" {
		t.Errorf("upload arrived as filename=%q content=%q", got.filename, got.content)
	}
	if got.contentType != "text/plain" {
		t.Errorf("part Content-Type = %q, want text/plain", got.contentType)
	}
	if got.fields["name"] != "readme" {
		t.Errorf("name field = %q", got.fields["name"])
	}
}

// A binary property nested inside the body is not a part of its own: the object
// holding it is encoded as JSON, so the bytes stay base64 there.
func TestNestedBinaryStaysEncoded(t *testing.T) {
	client, got := serve(t)

	err := client.Upload(t.Context(), UploadBody{
		File: FormFile{Filename: "a.txt", Content: []byte("hello")},
		Meta: &UploadBodyMeta{Thumbnail: []byte("thumb")},
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	var meta struct {
		Thumbnail []byte `+"`"+`json:"thumbnail"`+"`"+`
	}
	if err := json.Unmarshal([]byte(got.fields["meta"]), &meta); err != nil {
		t.Fatalf("meta part was not JSON: %q: %v", got.fields["meta"], err)
	}
	if string(meta.Thumbnail) != "thumb" {
		t.Errorf("nested bytes = %q, want thumb", meta.Thumbnail)
	}
}

// The $ref'd form keeps working exactly as before.
func TestRefBodyStillUploads(t *testing.T) {
	client, got := serve(t)

	err := client.UploadRef(t.Context(), UploadForm{
		File: FormFile{Filename: "b.txt", Content: []byte("world")},
	})
	if err != nil {
		t.Fatalf("UploadRef: %v", err)
	}
	if got.filename != "b.txt" || got.content != "world" {
		t.Errorf("upload arrived as filename=%q content=%q", got.filename, got.content)
	}
}
`)
}
