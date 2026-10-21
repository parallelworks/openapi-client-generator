package parser

import (
	"errors"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

// errNotFastJSON sends a document the JSON fast path cannot reproduce
// exactly back to the YAML parser.
var errNotFastJSON = errors.New("not handled by the JSON fast path")

// parseJSONNode builds the node tree the YAML parser produces for a JSON
// document (same kinds, tags, styles, values, lines and columns), several
// times faster than tokenizing the JSON as YAML flow syntax.
func parseJSONNode(data []byte) (*yaml.Node, error) {
	p := &jsonNodeParser{data: data, line: 1, col: 1, strs: map[string]string{}}
	p.skipSpace()
	doc := p.node()
	doc.Kind, doc.Line, doc.Column = yaml.DocumentNode, p.line, p.col
	root, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos != len(data) {
		return nil, errNotFastJSON
	}
	doc.Content = []*yaml.Node{root}
	return doc, nil
}

type jsonNodeParser struct {
	data      []byte
	pos       int
	line, col int
	slab      []yaml.Node
	stack     []*yaml.Node
	strs      map[string]string
	buf       []byte
}

func (p *jsonNodeParser) node() *yaml.Node {
	if len(p.slab) == 0 {
		p.slab = make([]yaml.Node, 1024)
	}
	n := &p.slab[0]
	p.slab = p.slab[1:]
	return n
}

func (p *jsonNodeParser) skipSpace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t':
			p.col++
		case '\n':
			p.line++
			p.col = 1
		case '\r':
			if p.pos+1 < len(p.data) && p.data[p.pos+1] == '\n' {
				p.pos++
			}
			p.line++
			p.col = 1
		default:
			return
		}
		p.pos++
	}
}

func (p *jsonNodeParser) value() (*yaml.Node, error) {
	if p.pos >= len(p.data) {
		return nil, errNotFastJSON
	}
	n := p.node()
	n.Line, n.Column = p.line, p.col
	switch p.data[p.pos] {
	case '{':
		return n, p.container(n, yaml.MappingNode, "!!map", '}')
	case '[':
		return n, p.container(n, yaml.SequenceNode, "!!seq", ']')
	case '"':
		s, err := p.str()
		if err != nil {
			return nil, err
		}
		n.Kind, n.Tag, n.Style, n.Value = yaml.ScalarNode, "!!str", yaml.DoubleQuotedStyle, s
		return n, nil
	default:
		return n, p.literal(n)
	}
}

func (p *jsonNodeParser) container(n *yaml.Node, kind yaml.Kind, tag string, end byte) error {
	n.Kind, n.Tag, n.Style = kind, tag, yaml.FlowStyle
	p.pos++
	p.col++
	base := len(p.stack)
	p.skipSpace()
	if p.pos < len(p.data) && p.data[p.pos] == end {
		p.pos++
		p.col++
		return nil
	}
	for {
		p.skipSpace()
		if kind == yaml.MappingNode {
			if p.pos >= len(p.data) || p.data[p.pos] != '"' {
				return errNotFastJSON
			}
			key, err := p.value()
			if err != nil {
				return err
			}
			p.stack = append(p.stack, key)
			p.skipSpace()
			if p.pos >= len(p.data) || p.data[p.pos] != ':' {
				return errNotFastJSON
			}
			p.pos++
			p.col++
			p.skipSpace()
		}
		v, err := p.value()
		if err != nil {
			return err
		}
		p.stack = append(p.stack, v)
		p.skipSpace()
		if p.pos >= len(p.data) {
			return errNotFastJSON
		}
		c := p.data[p.pos]
		p.pos++
		p.col++
		if c == end {
			break
		}
		if c != ',' {
			return errNotFastJSON
		}
	}
	n.Content = append([]*yaml.Node(nil), p.stack[base:]...)
	clear(p.stack[base:])
	p.stack = p.stack[:base]
	return nil
}

func (p *jsonNodeParser) str() (string, error) {
	p.pos++
	p.col++
	start := p.pos
	escaped := false
	for {
		if p.pos >= len(p.data) {
			return "", errNotFastJSON
		}
		c := p.data[p.pos]
		if c == '"' {
			break
		}
		if c < 0x20 {
			return "", errNotFastJSON
		}
		if c == '\\' {
			escaped = true
			p.pos++
			p.col++
			if p.pos >= len(p.data) {
				return "", errNotFastJSON
			}
			c = p.data[p.pos]
		}
		if c < utf8.RuneSelf || c >= 0xC0 {
			p.col++
		}
		p.pos++
	}
	raw := p.data[start:p.pos]
	p.pos++
	p.col++
	if !escaped {
		return p.intern(raw), nil
	}
	return p.unescape(raw)
}

func (p *jsonNodeParser) intern(b []byte) string {
	if len(b) > 64 {
		return string(b)
	}
	if s, ok := p.strs[string(b)]; ok {
		return s
	}
	s := string(b)
	p.strs[s] = s
	return s
}

func (p *jsonNodeParser) unescape(raw []byte) (string, error) {
	out := p.buf[:0]
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c != '\\' {
			out = append(out, c)
			continue
		}
		i++
		switch raw[i] {
		case '"', '\\':
			out = append(out, raw[i])
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			if i+5 > len(raw) {
				return "", errNotFastJSON
			}
			r, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
			if err != nil || utf16.IsSurrogate(rune(r)) {
				return "", errNotFastJSON
			}
			out = utf8.AppendRune(out, rune(r))
			i += 4
		default:
			// Includes \/, which the YAML parser only accepts after
			// libopenapi rewrites it, shifting the columns that follow.
			return "", errNotFastJSON
		}
	}
	p.buf = out
	return p.intern(out), nil
}

func (p *jsonNodeParser) literal(n *yaml.Node) error {
	start := p.pos
	for p.pos < len(p.data) {
		c := p.data[p.pos]
		if c == ',' || c == '}' || c == ']' || c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		p.pos++
	}
	raw := p.data[start:p.pos]
	p.col += len(raw)
	n.Kind = yaml.ScalarNode
	switch string(raw) {
	case "true", "false":
		n.Tag, n.Value = "!!bool", string(raw)
		return nil
	case "null":
		n.Tag, n.Value = "!!null", "null"
		return nil
	}
	tag, ok := numberTag(raw)
	if !ok {
		return errNotFastJSON
	}
	n.Tag, n.Value = tag, p.intern(raw)
	return nil
}

// numberTag resolves a JSON number the way the YAML core schema does.
func numberTag(raw []byte) (string, bool) {
	i := 0
	if i < len(raw) && raw[i] == '-' {
		i++
	}
	digits := func() int {
		n := 0
		for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
			i++
			n++
		}
		return n
	}
	intDigits := digits()
	if intDigits == 0 || (intDigits > 1 && raw[i-intDigits] == '0') {
		return "", false
	}
	isFloat := false
	if i < len(raw) && raw[i] == '.' {
		i++
		if digits() == 0 {
			return "", false
		}
		isFloat = true
	}
	if i < len(raw) && (raw[i] == 'e' || raw[i] == 'E') {
		i++
		if i < len(raw) && (raw[i] == '+' || raw[i] == '-') {
			i++
		}
		if digits() == 0 {
			return "", false
		}
		isFloat = true
	}
	if i != len(raw) {
		return "", false
	}
	if !isFloat {
		s := string(raw)
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return "!!int", true
		}
		if _, err := strconv.ParseUint(s, 10, 64); err == nil {
			return "!!int", true
		}
		return "", false
	}
	return "!!float", true
}
