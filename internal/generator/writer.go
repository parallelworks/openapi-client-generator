package generator

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
)

// stdImports resolves a package name a template references without importing it.
var stdImports = map[string]string{
	"base64":    "encoding/base64",
	"bufio":     "bufio",
	"bytes":     "bytes",
	"context":   "context",
	"encoding":  "encoding",
	"errors":    "errors",
	"fmt":       "fmt",
	"http":      "net/http",
	"io":        "io",
	"json":      "encoding/json",
	"math":      "math",
	"mime":      "mime",
	"multipart": "mime/multipart",
	"rand":      "math/rand",
	"reflect":   "reflect",
	"sort":      "sort",
	"strconv":   "strconv",
	"strings":   "strings",
	"textproto": "net/textproto",
	"time":      "time",
	"tls":       "crypto/tls",
	"url":       "net/url",
}

// WriteFiles fixes imports in and gofmts each generated file, then writes them to outputDir.
func WriteFiles(outputDir string, files []GeneratedFile) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	fset := token.NewFileSet()
	parsed := make([]*ast.File, len(files))
	parseErrs := make([]error, len(files))
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Go(func() {
			parsed[i], parseErrs[i] = parser.ParseFile(fset, f.Name, f.Content, parser.ParseComments)
		})
	}
	wg.Wait()

	// A name declared in a sibling file is not a package reference, even where
	// it shares a name with one.
	declared := map[string]bool{}
	for _, af := range parsed {
		if af != nil {
			for name := range af.Scope.Objects {
				declared[name] = true
			}
		}
	}

	writeErrs := make([]error, len(files))
	for i, f := range files {
		wg.Go(func() {
			out := f.Content
			if parseErrs[i] != nil {
				log.Printf("WARNING: formatting failed for %s: %v (writing unformatted)", f.Name, parseErrs[i])
			} else {
				fixImports(parsed[i], declared)
				formatted, err := formatFile(fset, parsed[i], f.Content)
				if err != nil {
					log.Printf("WARNING: formatting failed for %s: %v (writing unformatted)", f.Name, err)
				} else {
					out = formatted
				}
			}
			p := filepath.Join(outputDir, f.Name)
			if err := os.WriteFile(p, out, 0o644); err != nil {
				writeErrs[i] = fmt.Errorf("writing %s: %w", p, err)
			}
		})
	}
	wg.Wait()
	return errors.Join(writeErrs...)
}

// fixImports drops imports the file never references and adds the standard
// library ones it references without importing.
func fixImports(f *ast.File, declared map[string]bool) {
	used := map[string]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Obj == nil && !declared[id.Name] {
				used[id.Name] = true
			}
		}
		return true
	})

	var have, want []string
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return
		}
		have = append(have, p)
		if used[path.Base(p)] {
			want = append(want, p)
			delete(used, path.Base(p))
		}
	}
	for name := range used {
		if p, ok := stdImports[name]; ok {
			want = append(want, p)
		}
	}
	slices.Sort(have)
	slices.Sort(want)
	if slices.Equal(have, want) {
		return
	}

	var decl *ast.GenDecl
	decls := f.Decls[:0]
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			if decl == nil && len(want) > 0 {
				decl = gd
				decls = append(decls, d)
			}
			continue
		}
		decls = append(decls, d)
	}
	f.Decls = decls
	f.Imports = nil
	if len(want) == 0 {
		return
	}
	if decl == nil {
		decl = &ast.GenDecl{Tok: token.IMPORT, TokPos: f.Name.End()}
		f.Decls = slices.Insert(f.Decls, 0, ast.Decl(decl))
	}
	decl.Specs = decl.Specs[:0]
	for _, p := range want {
		spec := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(p)}}
		decl.Specs = append(decl.Specs, spec)
		f.Imports = append(f.Imports, spec)
	}
	switch {
	case len(want) == 1:
		decl.Lparen, decl.Rparen = token.NoPos, token.NoPos
	case !decl.Lparen.IsValid():
		decl.Lparen, decl.Rparen = decl.TokPos, decl.TokPos
	}
}

const formatChunkSize = 64 << 10

// formatFile gofmts a file, printing a large one in parallel chunks. A chunk
// boundary is a blank line between top-level declarations: gofmt prints that
// gap as exactly one blank line, and no alignment or comment spans it.
func formatFile(fset *token.FileSet, f *ast.File, src []byte) ([]byte, error) {
	tf := fset.File(f.Pos())
	offset := func(p token.Pos) int { return tf.Offset(p) }
	var cuts []int
	last := 0
	for i := 1; i < len(f.Decls); i++ {
		start := f.Decls[i].Pos()
		if doc := declDoc(f.Decls[i]); doc != nil {
			start = doc.Pos()
		}
		// fixImports may have rebuilt the import decl without positions.
		if gd, ok := f.Decls[i-1].(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			continue
		}
		from, to := offset(f.Decls[i-1].End()), offset(start)
		if from-last < formatChunkSize || !isBlankGap(src[from:to]) {
			continue
		}
		cuts = append(cuts, i)
		last = from
	}
	if len(cuts) == 0 {
		var buf bytes.Buffer
		err := format.Node(&buf, fset, f)
		return buf.Bytes(), err
	}

	bounds := append(append([]int{0}, cuts...), len(f.Decls))
	chunks := make([][]byte, len(bounds)-1)
	errs := make([]error, len(chunks))
	var wg sync.WaitGroup
	for c := range chunks {
		wg.Go(func() {
			decls := f.Decls[bounds[c]:bounds[c+1]]
			lo, hi := token.NoPos, decls[len(decls)-1].End()
			if c > 0 {
				lo = decls[0].Pos()
				if doc := declDoc(decls[0]); doc != nil {
					lo = doc.Pos()
				}
			}
			var comments []*ast.CommentGroup
			for _, cg := range f.Comments {
				if cg.Pos() >= lo && (c == len(chunks)-1 || cg.End() <= hi) {
					comments = append(comments, cg)
				}
			}
			// The printer only takes comments alongside a whole file, so every
			// chunk prints as one and the later ones drop the package clause.
			part := *f
			part.Decls, part.Comments = decls, comments
			if c > 0 {
				part.Doc = nil
			}
			// token.File locks on every position lookup, so chunks sharing one
			// serialize; a private copy at the same base keeps positions valid.
			own := token.NewFileSet()
			own.AddFile(tf.Name(), tf.Base(), tf.Size()).SetLines(tf.Lines())
			var buf bytes.Buffer
			if errs[c] = format.Node(&buf, own, &part); errs[c] != nil {
				return
			}
			out := buf.Bytes()
			if c > 0 {
				clause := "package " + f.Name.Name + "\n\n"
				if !bytes.HasPrefix(out, []byte(clause)) {
					errs[c] = fmt.Errorf("chunk %d does not start with %q", c, clause)
					return
				}
				out = out[len(clause):]
			}
			chunks[c] = bytes.TrimRight(out, "\n")
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return append(bytes.Join(chunks, []byte("\n\n")), '\n'), nil
}

func declDoc(d ast.Decl) *ast.CommentGroup {
	switch d := d.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		return d.Doc
	}
	return nil
}

func isBlankGap(gap []byte) bool {
	return len(bytes.TrimSpace(gap)) == 0 && bytes.Count(gap, []byte("\n")) >= 2
}
