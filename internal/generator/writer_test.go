package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFiles_FixesImports(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "adds missing and drops unused",
			in: `package p

import (
	"fmt"
	"net/http"
)

func f(r *http.Request) time.Time { return time.Now() }
`,
			want: `package p

import (
	"net/http"
	"time"
)

func f(r *http.Request) time.Time { return time.Now() }
`,
		},
		{
			name: "collapses a block left with one import",
			in: `package p

import (
	"fmt"
	"net/http"
)

var c http.Client
`,
			want: `package p

import "net/http"

var c http.Client
`,
		},
		{
			name: "removes an import block nothing uses",
			in: `package p

import "fmt"

var x = 1
`,
			want: `package p

var x = 1
`,
		},
		{
			name: "adds a block where there was none",
			in: `package p

func f() error { return errors.New(strings.ToUpper("x")) }
`,
			want: `package p

import (
	"errors"
	"strings"
)

func f() error { return errors.New(strings.ToUpper("x")) }
`,
		},
		{
			name: "ignores locals and literal keys named like packages",
			in: `package p

import "net/url"

type T struct{ url string }

func f(url *T) T { return T{url: url.url} }
`,
			want: `package p

type T struct{ url string }

func f(url *T) T { return T{url: url.url} }
`,
		},
		{
			name: "ignores names declared in a sibling file",
			in: `package p

func f() string { return rand.Name }
`,
			want: `package p

func f() string { return rand.Name }
`,
		},
		{
			name: "leaves a correct file as it was",
			in: `package p

import (
	"context"

	"net/http"
)

func f(ctx context.Context, c *http.Client) {}
`,
			want: `package p

import (
	"context"

	"net/http"
)

func f(ctx context.Context, c *http.Client) {}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			sibling := GeneratedFile{Name: "sibling.go", Content: []byte("package p\n\nvar rand struct{ Name string }\n")}
			if err := WriteFiles(dir, []GeneratedFile{{Name: "f.go", Content: []byte(tt.in)}, sibling}); err != nil {
				t.Fatalf("WriteFiles: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "f.go"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
