package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pb33f/libopenapi/datamodel"
	"go.yaml.in/yaml/v4"
)

// diffNodes reports the first place two node trees differ, or "".
func diffNodes(path string, got, want *yaml.Node) string {
	if got.Kind != want.Kind || got.Style != want.Style || got.Tag != want.Tag || got.Value != want.Value ||
		got.Line != want.Line || got.Column != want.Column || len(got.Content) != len(want.Content) {
		return fmt.Sprintf("%s:\n got  kind=%v style=%v tag=%q value=%q at %d:%d with %d children\n want kind=%v style=%v tag=%q value=%q at %d:%d with %d children",
			path, got.Kind, got.Style, got.Tag, got.Value, got.Line, got.Column, len(got.Content),
			want.Kind, want.Style, want.Tag, want.Value, want.Line, want.Column, len(want.Content))
	}
	for i := range got.Content {
		if d := diffNodes(fmt.Sprintf("%s/%d", path, i), got.Content[i], want.Content[i]); d != "" {
			return d
		}
	}
	return ""
}

func assertSpecInfoMatches(t *testing.T, spec []byte) {
	t.Helper()
	got, err := extractSpecInfo(spec, &datamodel.DocumentConfiguration{})
	if err != nil {
		t.Fatalf("extractSpecInfo: %v", err)
	}
	want, err := datamodel.ExtractSpecInfoWithConfig(spec, &datamodel.DocumentConfiguration{})
	if err != nil {
		t.Fatalf("ExtractSpecInfoWithConfig: %v", err)
	}
	type fields struct {
		SpecType, Version, SpecFormat, SpecFileType, APISchema, Self string
		VersionNumeric                                               float32
		NumLines, OriginalIndentation                                int
	}
	project := func(i *datamodel.SpecInfo) fields {
		return fields{i.SpecType, i.Version, i.SpecFormat, i.SpecFileType, i.APISchema, i.Self, i.VersionNumeric, i.NumLines, i.OriginalIndentation}
	}
	if g, w := project(got), project(want); g != w {
		t.Errorf("SpecInfo differs:\n got  %+v\n want %+v", g, w)
	}
	if d := diffNodes("", got.RootNode, want.RootNode); d != "" {
		t.Error(d)
	}
}

func assertMatchesYAML(t *testing.T, data []byte) {
	t.Helper()
	got, err := parseJSONNode(data)
	if err != nil {
		t.Fatalf("parseJSONNode: %v", err)
	}
	var want yaml.Node
	if err := yaml.Unmarshal(data, &want); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if d := diffNodes("", got, &want); d != "" {
		t.Error(d)
	}
}

func TestParseJSONNode_MatchesYAMLParser(t *testing.T) {
	cases := map[string]string{
		"scalars": `{"s": "x", "e": "", "t": true, "f": false, "n": null, "i": 42, "neg": -7, "zero": 0,
			"fl": 1.5, "exp": 1e5, "Exp": -2.5E-3, "big": 18446744073709551615, "nested": {"a": [1, [2, {}], []]}}`,
		"escapes":   `{"q": "a\"b\\c\nd\te\r\b\f", "u": "caf\u00e9 \u4e2d", "after": "x"}`,
		"unicode":   "{\"k\u00e9\": \"\u4e2d\u6587\", \"emoji\": \"\U0001F600\", \"after\": [1, 2]}",
		"crlf":      "{\r\n  \"a\": 1,\r\n  \"b\": [\r\n    true\r\n  ]\r\n}\r\n",
		"compact":   `{"a":{"b":[1,2,{"c":"d"}]},"e":[]}`,
		"leadingWS": "\n\n   {\"a\": 1}\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) { assertMatchesYAML(t, []byte(in)) })
	}

	specs, err := filepath.Glob(filepath.Join(projectRoot(), "testdata", "*.yaml"))
	if err != nil || len(specs) == 0 {
		t.Fatalf("no testdata specs: %v", err)
	}
	for _, spec := range specs {
		t.Run(filepath.Base(spec), func(t *testing.T) {
			src, err := os.ReadFile(spec)
			if err != nil {
				t.Fatal(err)
			}
			var v any
			if err := yaml.Unmarshal(src, &v); err != nil {
				t.Fatal(err)
			}
			indented, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			assertMatchesYAML(t, indented)
			assertSpecInfoMatches(t, indented)
			compact, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			assertMatchesYAML(t, compact)
		})
	}
}

func TestParseJSONNode_DefersToYAMLParser(t *testing.T) {
	for name, in := range map[string]string{
		"escaped slash":   `{"a": "x\/y"}`,
		"surrogate pair":  `{"a": "\ud83d\ude00"}`,
		"lone surrogate":  `{"a": "\ud83d"}`,
		"bom":             "\ufeff{\"a\": 1}",
		"trailing comma":  `{"a": 1,}`,
		"leading zero":    `{"a": 01}`,
		"int overflow":    `{"a": 99999999999999999999}`,
		"bare word":       `{"a": yes}`,
		"trailing data":   `{"a": 1} x`,
		"unterminated":    `{"a": "b`,
		"control in text": "{\"a\": \"b\nc\"}",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseJSONNode([]byte(in)); !errors.Is(err, errNotFastJSON) {
				t.Errorf("got %v, want errNotFastJSON", err)
			}
		})
	}
}
