// +build ignore

// Generate Go structs from the OpenAPI Specification.
// https://github.com/OAI/OpenAPI-Specification/blob/master/versions/2.0.md

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var specialTypes = []struct {
	Name string
	Val  string
}{
	{"Definitions", `map[string]Schema`},
	{"Example", `map[string]interface{}`},
	{"Paths", `map[string]PathItem`},
	{"ParametersDefinitions", `map[string]Parameter`},
	{"Responses", `map[string]Response`},
	{"ResponsesDefinitions", `map[string]Response`},
	{"Scopes", `map[string]string`},
	{"SecurityDefinitions", `map[string]SecurityScheme`},
	{"SecurityRequirement", `map[string][]string`},
	{"Headers", `map[string]Header`},
}

func specialType(name string) bool {
	for _, t := range specialTypes {
		if t.Name == name {
			return true
		}
	}
	return false
}

var omitType = map[string]bool{
	"Reference": true,
}

// canBeReference refers to
var canBeReference = map[string]bool{
	"Parameter": true,
	"Response":  true,
	"Schema":    true,
}

var typeMappings = map[string]string{
	"string":  "string",
	"number":  "float64",
	"boolean": "bool",
	"integer": "int",
	"Any":     "interface{}",
	"*":       "interface{}",
	"[*]":     "[]interface{}",
}

func objName(s string) string {
	if s == "$ref" {
		return "Ref"
	}
	return strings.Title(s)
}

func fieldType(s string) string {
	if specialType(s) {
		return s
	}
	if r, _ := utf8.DecodeRuneInString(s); unicode.IsUpper(r) {
		return "*" + s
	}
	return s
}

func objTypeName(s string) string {
	s = strings.TrimSpace(s)
	// handle arrays
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		return "[]" + objTypeName(s[1:len(s)-1])
	}
	// prefer explicit mappings
	if t, ok := typeMappings[s]; ok {
		return t
	}
	if n := strings.Index(s, "|"); n >= 0 {
		return objTypeName(s[:n])
	}
	// fallback to formatting things like "Swagger Object"
	return strings.Join(strings.Fields(strings.TrimSuffix(s, "Object")), "")
}

func wrapStringAfter(s string, i int) []string {
	pos := 0
	for {
		n := strings.Index(s[pos:], " ")
		if n < 0 {
			if pos < i {
				return []string{s}
			}
			return append([]string{s[:pos-1]}, wrapStringAfter(s[pos:], i)...)
		}
		if (pos + n) > i {
			return append([]string{s[:pos-1]}, wrapStringAfter(s[pos:], i)...)
		}
		pos = pos + n + 1
	}
}

func main() {
	root, err := parseFile("2.0.html")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	// find a node that has the child <a href="#schema">
	matcher := func(n *html.Node) bool {
		return hasChild(n, func(n *html.Node) bool {
			return n.DataAtom == atom.A && hasAttr(n, "href", "#schema")
		})
	}

	schema := find(root, matcher)
	if schema == nil {
		fmt.Fprintln(os.Stderr, "could not find schema tag")
		os.Exit(2)
	}

	var doc bytes.Buffer
	doc.WriteString(`// This file was generated by gen.go. DO NOT EDIT.

package spec
`)

	commentStrings := make(map[string]string)

	var name string

	parseTable := func(c *html.Node) {
		table := nextSibling(c, byAtom(atom.Table))
		if table == nil {
			fmt.Fprintf(os.Stderr, "<table> does not follow field fields for %s\n", name)
			os.Exit(2)
		}
		p, err := newTableParser(table)
		if err != nil {
			fmt.Fprintf(os.Stderr, "table %s failed %v\n", name, err)
			os.Exit(2)
		}

		fmt.Fprintln(&doc, "\n"+commentStrings[name])

		fmt.Fprintln(&doc, "type", name, "struct {")
		for _, field := range p.fields() {
			fmt.Fprintln(&doc, field)
		}
		fmt.Fprintln(&doc, "}")
	}

	for c := schema.NextSibling; c != nil && c.DataAtom != atom.H3; c = c.NextSibling {
		switch c.DataAtom {
		case atom.H4:
			name = objTypeName(text(c))
			var lines []string
			for s := c.NextSibling; ; s = s.NextSibling {
				if s.Type != html.ElementNode {
					continue
				}
				if s.DataAtom != atom.P {
					break
				}
				lines = append(lines, "// "+strings.Join(wrapStringAfter(text(s), 85), "\n// "))
				commentStrings[name] = strings.Join(lines, "\n//\n")
			}
			// For some reason "Header Object" does not have a "Fixed Fields" field.
			if name == "Header" {
				parseTable(c)
			}
		case atom.H5:
			if text(c) != "Fixed Fields" || specialType(name) {
				continue
			}
			parseTable(c)
		}
	}
	for _, t := range specialTypes {
		fmt.Fprintf(&doc, "\n%s\ntype %s %s\n", commentStrings[t.Name], t.Name, t.Val)
	}
	if err := ioutil.WriteFile("schema.go", doc.Bytes(), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write schema.go", err)
		os.Exit(2)
	}
}

type field struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

func (f field) String() string {
	name := f.Name
	if !f.Required {
		name = f.Name + ",omitempty"
	}
	commentLines := wrapStringAfter(f.Description, 80)
	comment := "\t// " + strings.Join(commentLines, "\n\t// ")
	return fmt.Sprintf("%s\n\t%s %s `json:\"%s\" yaml:\"%s\"`", comment, objName(f.Name), fieldType(objTypeName(f.Type)), name, name)
}

const (
	colFieldName   = "Field Name"
	colType        = "Type"
	colValidity    = "Validity"
	colDescription = "Description"
)

type tableParser struct {
	table            *html.Node
	nameIndex        int
	typeIndex        int
	validityIndex    int
	descriptionIndex int
}

func newTableParser(n *html.Node) (*tableParser, error) {
	if n.Type != html.ElementNode || n.DataAtom != atom.Table {
		return nil, errors.New("node is not a <table> element")
	}
	t := tableParser{table: n}
	fields := map[string]bool{
		colFieldName:   false,
		colType:        false,
		colDescription: false,
	}

	for i, th := range findAll(n, byAtom(atom.Th)) {
		txt := text(th)
		switch txt {
		case colFieldName:
			t.nameIndex = i
		case colType:
			t.typeIndex = i
		case colValidity:
			t.validityIndex = i
		case colDescription:
			t.descriptionIndex = i
		}
		fields[txt] = true
	}

	for name, found := range fields {
		if !found {
			return nil, fmt.Errorf("table header did not contain field %q", name)
		}
	}

	return &t, nil
}

func (t *tableParser) fields() []field {
	body := find(t.table, byAtom(atom.Tbody))
	if body == nil {
		return nil
	}
	rows := findAll(body, byAtom(atom.Tr))
	f := make([]field, len(rows))
	for i, row := range rows {
		tds := findAll(row, byAtom(atom.Td))
		desc := text(tds[t.descriptionIndex])
		required := false
		if strings.HasPrefix(desc, "Required. ") {
			desc = strings.TrimPrefix(desc, "Required. ")
			required = true
		}
		f[i] = field{
			Name:        text(tds[t.nameIndex]),
			Type:        text(tds[t.typeIndex]),
			Description: desc,
			Required:    required,
		}
	}
	return f
}

func parseFile(filename string) (*html.Node, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return html.Parse(f)
}

func text(n *html.Node) string {
	matcher := func(n *html.Node) bool {
		return n.Type == html.TextNode
	}
	var strs []string
	for _, node := range findAll(n, matcher) {
		if txt := strings.TrimSpace(node.Data); txt != "" {
			strs = append(strs, strings.Replace(strings.Trim(node.Data, "\n"), "\n", " ", -1))
		}
	}
	return strings.Join(strs, "")
}

func find(n *html.Node, match func(n *html.Node) bool) *html.Node {
	if match(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := find(c, match); found != nil {
			return found
		}
	}
	return nil
}

func findAll(n *html.Node, match func(n *html.Node) bool) []*html.Node {
	if match(n) {
		return []*html.Node{n}
	}
	var found []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		found = append(found, findAll(c, match)...)
	}
	return found
}

func nextSibling(n *html.Node, match func(n *html.Node) bool) *html.Node {
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if match(s) {
			return s
		}
	}
	return nil
}

func byAtom(a atom.Atom) func(n *html.Node) bool {
	return func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.DataAtom == a
	}
}

func hasChild(n *html.Node, match func(n *html.Node) bool) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if match(c) {
			return true
		}
	}
	return false
}

func hasAttr(n *html.Node, key, val string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key && attr.Val == val {
			return true
		}
	}
	return false
}
