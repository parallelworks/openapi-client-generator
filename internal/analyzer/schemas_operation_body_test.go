package analyzer

import "testing"

const operationBodySpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /avatar:
    post:
      operationId: uploadUserAvatar
      requestBody:
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

func TestOperationBody_InlineBodiesOfOneShapeStayPerOperation(t *testing.T) {
	pkg, typeMap := analyzeSpec(t, operationBodySpec)

	want := map[string]struct{ body, response string }{
		"UploadUserAvatar":   {"UploadUserAvatarBody", "UploadUserAvatarResponse"},
		"UploadWorkflowIcon": {"UploadWorkflowIconBody", "UploadWorkflowIconResponse"},
	}
	for _, op := range pkg.Operations {
		w, ok := want[op.Name]
		if !ok {
			t.Fatalf("unexpected operation %s", op.Name)
		}
		if op.RequestBody == nil || op.RequestBody.TypeName != w.body {
			t.Errorf("%s body = %v, want %s", op.Name, op.RequestBody, w.body)
		}
		if got := op.Responses[0].TypeName; got != w.response {
			t.Errorf("%s response = %q, want %q", op.Name, got, w.response)
		}
		if typeMap[w.body] == nil {
			t.Errorf("%s not generated", w.body)
		}
		if typeMap[w.response] == nil {
			t.Errorf("%s not generated", w.response)
		}
	}
}

const sharedComponentBodySpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /a:
    post:
      operationId: createA
      requestBody:
        $ref: '#/components/requestBodies/Upload'
      responses:
        "400": { $ref: '#/components/responses/Failure' }
  /b:
    post:
      operationId: createB
      requestBody:
        $ref: '#/components/requestBodies/Upload'
      responses:
        "400": { $ref: '#/components/responses/Failure' }
components:
  requestBodies:
    Upload:
      content:
        application/json:
          schema:
            type: object
            properties:
              file: { type: string }
  responses:
    Failure:
      description: nope
      content:
        application/json:
          schema:
            type: object
            properties:
              message: { type: string }
`

func TestOperationBody_ComponentBodyStaysShared(t *testing.T) {
	pkg, _ := analyzeSpec(t, sharedComponentBodySpec)

	var bodies, responses []string
	for _, op := range pkg.Operations {
		bodies = append(bodies, op.RequestBody.TypeName)
		responses = append(responses, op.Responses[0].TypeName)
	}
	if len(bodies) != 2 || bodies[0] != bodies[1] {
		t.Errorf("component request body types = %v, want both operations on one type", bodies)
	}
	if len(responses) != 2 || responses[0] != responses[1] {
		t.Errorf("component response types = %v, want both operations on one type", responses)
	}
}

const titledBodySpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /a:
    post:
      operationId: createA
      requestBody:
        content:
          application/json:
            schema:
              type: object
              title: Upload
              properties:
                file: { type: string }
      responses:
        "204": { description: ok }
  /b:
    post:
      operationId: createB
      requestBody:
        content:
          application/json:
            schema:
              type: object
              title: Upload
              properties:
                file: { type: string }
      responses:
        "204": { description: ok }
`

func TestOperationBody_TitledBodyStaysShared(t *testing.T) {
	pkg, _ := analyzeSpec(t, titledBodySpec)

	for _, op := range pkg.Operations {
		if got := op.RequestBody.TypeName; got != "Upload" {
			t.Errorf("%s body = %q, want Upload", op.Name, got)
		}
	}
}

const inlineEventStreamSpec = `openapi: 3.1.0
info: { title: t, version: "1" }
paths:
  /events:
    get:
      operationId: streamEvents
      responses:
        "200":
          description: ok
          content:
            text/event-stream:
              schema:
                type: object
                properties:
                  id: { type: string }
`

// An event payload is resolved twice, once as the response body and once as the
// event, so scoping it to the response keeps the two on one declared type.
func TestOperationBody_EventPayloadDeclaresOneType(t *testing.T) {
	pkg, _ := analyzeSpec(t, inlineEventStreamSpec)

	op := pkg.Operations[0]
	if op.EventType != op.Responses[0].TypeName {
		t.Errorf("event type %q, response type %q, want one type", op.EventType, op.Responses[0].TypeName)
	}
	var synthesized []string
	for _, td := range pkg.Types {
		synthesized = append(synthesized, td.Name)
	}
	if len(synthesized) != 1 {
		t.Errorf("generated types = %v, want one", synthesized)
	}
}
