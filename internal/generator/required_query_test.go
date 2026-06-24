package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parallelworks/openapi-client-generator/internal/ir"
)

func TestHasRequiredQueryParams(t *testing.T) {
	tests := []struct {
		name string
		op   *ir.OperationDef
		want bool
	}{
		{name: "no params", op: &ir.OperationDef{}, want: false},
		{
			name: "required query param",
			op:   &ir.OperationDef{QueryParams: []*ir.ParamDef{{Name: "startDate", Required: true}}},
			want: true,
		},
		{
			name: "only optional query param",
			op:   &ir.OperationDef{QueryParams: []*ir.ParamDef{{Name: "limit", Required: false}}},
			want: false,
		},
		{
			name: "required header param does not count",
			op:   &ir.OperationDef{HeaderParams: []*ir.ParamDef{{Name: "X-Request-Id", Required: true}}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasRequiredQueryParams(tt.op); got != tt.want {
				t.Errorf("hasRequiredQueryParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGenerate_RequiredQueryParam_Compiles is a regression test for required
// query parameters being dropped from the generated client. A required query
// param must become a positional method argument and always be encoded; an
// optional one stays in the params struct. The generated package must compile.
func TestGenerate_RequiredQueryParam_Compiles(t *testing.T) {
	pkg := &ir.Package{
		Name: "reqquery",
		Operations: []*ir.OperationDef{
			{
				Name:       "GetUsageSummary",
				HTTPMethod: "GET",
				Path:       "/things/{id}/usage",
				PathParams: []*ir.ParamDef{
					{Name: "id", FieldName: "ID", OrigName: "id", Location: "path", Type: "string", Required: true},
				},
				QueryParams: []*ir.ParamDef{
					{Name: "startDate", FieldName: "StartDate", OrigName: "startDate", Location: "query", Type: "string", Required: true},
					{Name: "endDate", FieldName: "EndDate", OrigName: "endDate", Location: "query", Type: "string", Required: false},
				},
			},
		},
	}

	gen, err := New(pkg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	files, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	var ops string
	for _, f := range files {
		if f.Name == "operations.go" {
			ops = string(f.Content)
		}
	}
	if ops == "" {
		t.Fatal("operations.go not generated")
	}

	// The required query param must be a positional argument, not a struct field.
	if !strings.Contains(ops, "startDate string") {
		t.Errorf("required query param missing from method signature:\n%s", ops)
	}
	if strings.Contains(ops, "StartDate") {
		t.Errorf("required query param leaked into the params struct (StartDate):\n%s", ops)
	}
	// The required param must be encoded unconditionally (from the positional arg).
	if !strings.Contains(ops, `addQueryParam(queryValues, "startDate", startDate)`) {
		t.Errorf("required query param not encoded into the query string:\n%s", ops)
	}
	// The optional param stays in the params struct and is encoded from opts.
	if !strings.Contains(ops, "EndDate *string") {
		t.Errorf("optional query param missing from params struct:\n%s", ops)
	}
	if !strings.Contains(ops, `addQueryParam(queryValues, "endDate", params.EndDate)`) {
		t.Errorf("optional query param not encoded from opts:\n%s", ops)
	}

	// The whole generated package must compile.
	tmpDir := t.TempDir()
	goMod := []byte("module reqquery-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("generated code with a required query param failed to compile: %v\n%s", err, string(output))
	}
}
