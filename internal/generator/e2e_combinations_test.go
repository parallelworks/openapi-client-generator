package generator

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/analyzer"
	"github.com/parallelworks/openapi-client-generator/internal/parser"
)

// TestE2E_Combinations generates from a spec that crosses features rather than
// exercising them one at a time, then compiles and vets the result.
//
// The per-feature tests around it each drive one thing through a spec written
// for it. What they cannot catch is a feature that quietly stops happening in
// the presence of another, or one that disappears entirely while the package
// still builds: iterators generated for no operation, a union base that stops
// being found, a file part that turns back into base64 text. Every assertion
// here is a feature that has broken that way at least once.
func TestE2E_Combinations(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "combinations.yaml")

	result, err := parser.Parse(specPath, parser.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pkg, err := analyzer.New(result.Model).Analyze("combinations")
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	byName := make(map[string]string, len(files))
	for _, f := range files {
		byName[f.Name] = string(f.Content)
	}

	for _, want := range []struct{ file, decl, why string }{
		// One iterator per pagination style, in one package.
		{"pagination.go", "func (c *Client) ListThingsIter", "cursor pagination"},
		{"pagination.go", "func (c *Client) ListAlertsIter", "offset pagination over a bare array"},
		{"pagination.go", "func (c *Client) ListReportsIter", "page pagination named perPage"},
		{"pagination.go", "*PageIterator[Thing]", "an iterator over a union"},

		// A discriminated union whose variants compose a base and pin their tag
		// with const.
		{"types.go", "func (u Thing) Base() *ThingBase", "the base every variant composes"},
		{"types.go", "Kind  string `json:\"kind\"`", "a const tag typed as a string"},

		// Multipart whose body composes a schema through allOf.
		{"types.go", "File FormFile", "a file part in a composed multipart body"},
		{"types.go", "Meta", "the schema the multipart body composes"},
		{"operations.go", "body UploadThingBody", "a typed multipart body"},

		// Names the templates declare, and names that normalize together.
		{"types.go", "type Client2 struct", "a schema renamed off a reserved name"},
		{"types.go", "type DefaultBaseURL2 struct", "a schema renamed off a server constant"},
		{"types.go", "UserID2 *string `json:\"user_id,omitempty\"`", "properties that normalize together"},
		{"responses.go", "XTrace2", "response headers that normalize together"},

		// The rest of the surface, each of which has regressed once.
		{"client.go", "const DefaultBaseURL", "the server URL the spec declares"},
		{"client.go", "func ServerURL(region string, basePath string) string", "a templated server"},
		{"webhooks.go", "func ParseThingCreatedWebhook", "a webhook payload"},
		{"webhooks.go", "func ParseUploadThingOnStoredCallback", "a callback payload"},
		{"responses.go", "XRateLimitRemaining *int64", "a typed response header"},
		{"operations.go", "addContentQueryParam", "a parameter serialized as its media type"},
		{"operations.go", "encodeQueryAllowingReserved", "allowReserved on a query parameter"},
		{"operations.go", "params.Either", "a union-typed parameter"},
		{"errors.go", "func (e *ProblemResponse) Error() string", "a typed error wrapper"},
		{"errors.go", "e.Detail.Detail", "a message field found by its conventional name"},
		{"errors.go", "func parseFailureResponse", "a second error shape in one package"},
		{"errors.go", "func parseListAlertsResponse422Error", "an error body with no name of its own"},
		{"types.go", "Name string `json:\"name\"`", "a property two allOf entries declare, required by one"},
	} {
		if !containsCollapsed(byName[want.file], want.decl) {
			t.Errorf("%s is missing %s (%s)", want.file, want.why, want.decl)
		}
	}

	// The public operation opts out of the credential the document requires.
	if !containsCollapsed(byName["operations.go"], `"application/json", false`) {
		t.Error("operations.go: the operation declaring security: [] should not authenticate")
	}

	buildGenerated(t, files, "combinations")
}

// containsCollapsed reports whether haystack holds needle once the runs of
// whitespace in both are flattened. The generated files reach this test before
// goimports aligns them, so an assertion written the way the output looks would
// depend on alignment that has not happened yet.
func containsCollapsed(haystack, needle string) bool {
	return strings.Contains(strings.Join(strings.Fields(haystack), " "), strings.Join(strings.Fields(needle), " "))
}

// TestGenerationIsDeterministic generates one spec twice and compares the bytes.
// Generated clients are committed and reviewed, so output that shifts between
// runs shows up as churn in a diff nobody made, and the usual cause is a map
// iterated somewhere on the way out.
func TestGenerationIsDeterministic(t *testing.T) {
	specPath := filepath.Join(projectRoot(), "testdata", "combinations.yaml")

	generate := func() map[string]string {
		t.Helper()
		result, err := parser.Parse(specPath, parser.Config{})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		pkg, err := analyzer.New(result.Model).Analyze("combinations")
		if err != nil {
			t.Fatalf("Analyze: %v", err)
		}
		gen, err := New(pkg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		files, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		out := make(map[string]string, len(files))
		for _, f := range files {
			out[f.Name] = string(f.Content)
		}
		return out
	}

	first, second := generate(), generate()

	if len(first) != len(second) {
		t.Fatalf("file counts differ: %d and %d", len(first), len(second))
	}
	for name, content := range first {
		other, ok := second[name]
		if !ok {
			t.Errorf("%s was generated once and not the other time", name)
			continue
		}
		if content != other {
			t.Errorf("%s differs between two runs of the same spec", name)
		}
	}
}
