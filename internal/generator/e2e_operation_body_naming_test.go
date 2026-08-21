package generator

import "testing"

const repeatedInlineBodySpec = `openapi: 3.1.0
info: { title: uploads, version: "1" }
paths:
  /avatar:
    post:
      operationId: uploadUserAvatar
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file: { type: string, format: binary }
              required: [file]
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  url: { type: string }
  /icon:
    post:
      operationId: uploadWorkflowIcon
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                file: { type: string, format: binary }
              required: [file]
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                properties:
                  url: { type: string }
`

// TestE2E_RepeatedInlineBodyNaming covers two operations whose inline bodies
// happen to share a shape. They were collapsed onto one type, so uploading a
// workflow icon meant constructing a type named for the avatar endpoint.
func TestE2E_RepeatedInlineBodyNaming(t *testing.T) {
	files, _ := generateFromSpec(t, repeatedInlineBodySpec, "uploadsapi")

	runGeneratedWireTest(t, files, "repeatedinlinebody", `package uploadsapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEachOperationHasItsOwnBodyAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`+"`"+`{"url":"`+"`"+` + r.URL.Path + `+"`"+`"}`+"`"+`))
	}))
	defer srv.Close()
	client := NewClient(srv.URL)

	avatar, err := client.UploadUserAvatar(t.Context(), UploadUserAvatarBody{
		File: FormFile{Filename: "a.png", Content: []byte("a")},
	})
	if err != nil {
		t.Fatalf("UploadUserAvatar: %v", err)
	}
	var _ *UploadUserAvatarResponse = avatar
	if avatar.URL == nil || *avatar.URL != "/avatar" {
		t.Errorf("avatar url = %v", avatar.URL)
	}

	icon, err := client.UploadWorkflowIcon(t.Context(), UploadWorkflowIconBody{
		File: FormFile{Filename: "b.png", Content: []byte("b")},
	})
	if err != nil {
		t.Fatalf("UploadWorkflowIcon: %v", err)
	}
	var _ *UploadWorkflowIconResponse = icon
	if icon.URL == nil || *icon.URL != "/icon" {
		t.Errorf("icon url = %v", icon.URL)
	}
}
`)
}
