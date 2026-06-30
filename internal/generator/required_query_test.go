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
					{Name: "tags", FieldName: "Tags", OrigName: "tags", Location: "query", Type: "[]string", Required: true},
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

	// A required param is a value field in the params struct (not a pointer), and
	// because the op has a required param the struct is a mandatory argument.
	if !strings.Contains(ops, "StartDate string `json:\"startDate\"`") {
		t.Errorf("required query param not a value field in the params struct:\n%s", ops)
	}
	if !strings.Contains(ops, "params GetUsageSummaryParams)") {
		t.Errorf("required param did not make the params struct a mandatory argument:\n%s", ops)
	}
	// Required and optional params alike are encoded via addQueryParam, which sends
	// any present value (a nil optional pointer is the only thing it skips), so a
	// required param — and an explicitly-set optional zero value — always reaches
	// the server.
	if !strings.Contains(ops, `addQueryParam(queryValues, "startDate", params.StartDate)`) {
		t.Errorf("required query param not encoded into the query string:\n%s", ops)
	}
	// A required slice param is a value []T field, encoded as repeated keys.
	if !strings.Contains(ops, "Tags []string `json:\"tags\"`") {
		t.Errorf("required slice query param not a value field in the params struct:\n%s", ops)
	}
	if !strings.Contains(ops, `addQueryParam(queryValues, "tags", params.Tags)`) {
		t.Errorf("required slice query param not encoded into the query string:\n%s", ops)
	}
	// The optional param is a pointer field, also encoded via addQueryParam.
	if !strings.Contains(ops, "EndDate *string") {
		t.Errorf("optional query param missing from params struct:\n%s", ops)
	}
	if !strings.Contains(ops, `addQueryParam(queryValues, "endDate", params.EndDate)`) {
		t.Errorf("optional query param not encoded from params:\n%s", ops)
	}

	// The whole generated package must compile, and the param encoder must keep
	// zero values (a required param dropped from the URL makes the server reject
	// the request as missing a required parameter).
	tmpDir := t.TempDir()
	goMod := []byte("module reqquery-e2e-test\n\ngo 1.25.5\n")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), goMod, 0o644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	if err := WriteFiles(tmpDir, files); err != nil {
		t.Fatalf("WriteFiles: %v", err)
	}
	// A generated unit test that exercises addQueryParam directly, proving a
	// required value and an explicitly-set optional zero value are both encoded.
	zeroTest := []byte(`package reqquery

import (
	"net/url"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// A required param (a plain value) must always be encoded, even at its zero value.
func TestAddQueryParamKeepsRequiredZeroValues(t *testing.T) {
	for _, v := range []any{0, false, ""} {
		vals := url.Values{}
		addQueryParam(vals, "k", v)
		if _, ok := vals["k"]; !ok {
			t.Errorf("addQueryParam dropped required zero value %#v; required params must always be encoded", v)
		}
	}
}

// An optional param is a pointer: a non-nil pointer means "explicitly set", so an
// explicit zero value must be sent — only a nil pointer is skipped.
func TestAddQueryParamSendsExplicitOptionalZeroValues(t *testing.T) {
	for _, v := range []any{ptr(0), ptr(false), ptr("")} {
		vals := url.Values{}
		addQueryParam(vals, "k", v)
		if _, ok := vals["k"]; !ok {
			t.Errorf("addQueryParam dropped explicitly-set optional zero value %#v", v)
		}
	}
	var nilPtr *int
	vals := url.Values{}
	addQueryParam(vals, "k", nilPtr)
	if _, ok := vals["k"]; ok {
		t.Error("addQueryParam encoded an unset (nil) optional param")
	}
}

func TestQueryParamsEncodeSlicesAsRepeatedKeys(t *testing.T) {
	vals := url.Values{}
	addQueryParam(vals, "ids", []string{"a", "b", "c"})
	if got := vals["ids"]; len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("addQueryParam([]string) = %#v; want repeated keys [a b c]", got)
	}
	vals = url.Values{}
	addQueryParam(vals, "nums", []int{1, 2, 3})
	if got := vals["nums"]; len(got) != 3 || got[0] != "1" || got[2] != "3" {
		t.Errorf("addQueryParam([]int) = %#v; want repeated keys [1 2 3]", got)
	}
}
`)
	if err := os.WriteFile(filepath.Join(tmpDir, "zero_value_test.go"), zeroTest, 0o644); err != nil {
		t.Fatalf("writing zero_value_test.go: %v", err)
	}
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		for _, f := range files {
			t.Logf("=== %s ===\n%s", f.Name, string(f.Content))
		}
		t.Fatalf("generated code with a required query param failed to build/test: %v\n%s", err, string(output))
	}
}
