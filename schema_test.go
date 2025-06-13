package main

import (
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// schemaFromXML unmarshals the given XML string and builds a SchemaTree.
func schemaFromXML(xmlStr string) (SchemaTree, error) {
	var dict FixDictionary
	if err := xml.Unmarshal([]byte(xmlStr), &dict); err != nil {
		return SchemaTree{}, err
	}
	return buildSchema(dict), nil
}

func TestBuildFieldNodes(t *testing.T) {
	fieldMap := map[string]Field{
		"X": {Name: "X", Number: 1, Type: "STRING"},
	}
	refs := []FieldRef{
		{Name: "X", Required: "Y"},
		{Name: "Y", Required: "N"},
	}
	nodes := buildFieldNodes(refs, fieldMap)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	expected := FieldNode{Ref: FieldRef{Name: "X", Required: "Y"}, Field: fieldMap["X"]}
	if !reflect.DeepEqual(nodes[0], expected) {
		t.Errorf("unexpected node; want %+v, got %+v", expected, nodes[0])
	}
}

func TestBuildComponentNode(t *testing.T) {
	dict := FixDictionary{
		Major: "1", Minor: "0",
		Fields: []Field{
			{Name: "X", Number: 1, Type: "STRING"},
		},
		Components: []Component{
			{Name: "C1", Fields: []FieldRef{{Name: "X", Required: "Y"}}},
			{Name: "C2", Components: []ComponentRef{{Name: "C1", Required: "N"}}},
		},
		Header:  Component{},
		Trailer: Component{},
	}
	schema := buildSchema(dict)

	c2, ok := schema.Components["C2"]
	if !ok {
		t.Fatal("C2 not found in Components")
	}
	if c2.Name != "C2" {
		t.Errorf("expected Name C2, got %s", c2.Name)
	}
	if len(c2.Components) != 1 || c2.Components[0].Name != "C1" {
		t.Errorf("expected C1 subcomponent, got %+v", c2.Components)
	}
}

func TestBuildGroupNode(t *testing.T) {
	dict := FixDictionary{
		Major:      "1",
		Minor:      "0",
		Fields:     []Field{{Name: "X", Number: 1, Type: "STRING"}},
		Components: []Component{},
		Header:     Component{},
		Trailer:    Component{},
	}
	fieldMap := map[string]Field{"X": dict.Fields[0]}
	group := Group{Name: "G1", Required: "Y", Fields: []FieldRef{{Name: "X", Required: "Y"}}}

	gn := buildGroupNode(group, fieldMap, nil)
	if gn.Name != "G1" {
		t.Errorf("expected group Name G1, got %s", gn.Name)
	}
	if len(gn.Fields) != 1 || gn.Fields[0].Ref.Name != "X" {
		t.Errorf("expected field X in group, got %+v", gn.Fields)
	}
}

func TestBuildMessageNode(t *testing.T) {
	dict := FixDictionary{
		Major:      "1",
		Minor:      "0",
		Fields:     []Field{{Name: "X", Number: 1, Type: "STRING"}},
		Components: []Component{{Name: "C1", Fields: []FieldRef{{Name: "X", Required: "Y"}}}},
		Header:     Component{},
		Trailer:    Component{},
		Messages: []Message{
			{
				Name:       "M",
				MsgType:    "T",
				MsgCat:     "CAT",
				Fields:     []FieldRef{{Name: "X", Required: "N"}},
				Components: []ComponentRef{{Name: "C1", Required: "N"}},
			},
		},
	}
	schema := buildSchema(dict)

	m, ok := schema.Messages["M"]
	if !ok {
		t.Fatal("Message M not found")
	}
	if m.Name != "M" || m.MsgType != "T" || m.MsgCat != "CAT" {
		t.Errorf("unexpected message metadata; got %+v", m)
	}
	if len(m.Fields) != 1 || m.Fields[0].Ref.Name != "X" {
		t.Errorf("unexpected message field; got %+v", m.Fields)
	}
	if len(m.Components) != 1 || m.Components[0].Name != "C1" {
		t.Errorf("unexpected message component; got %+v", m.Components)
	}
}

func TestBuildSchemaHeaderTrailer(t *testing.T) {
	xmlStr := `<fix major="2" minor="0">
	<fields/>
	<components/>
	<messages/>
	<header/>
	<trailer/>
</fix>`
	s, err := schemaFromXML(xmlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Version != "2.0" {
		t.Errorf("expected version 2.0, got %s", s.Version)
	}
	if _, ok := s.Components["Header"]; !ok {
		t.Error("Header component missing")
	}
	if _, ok := s.Components["Trailer"]; !ok {
		t.Error("Trailer component missing")
	}
}

// TestLoadSchemaFromOpts_FileMissing exercises the final `return loadSchema(...)`
// when the path does not exist, so we get an error.
func TestLoadSchemaFromOptsFileMissing(t *testing.T) {
	opts := CLIOptions{XMLPath: "does_not_exist_12345.xml"}
	_, err := loadSchemaFromOpts(opts)
	if err == nil {
		t.Fatal("expected error loading nonexistent file, got nil")
	}
}

// TestLoadSchemaFromOpts_FileValid exercises the same branch but with a real file,
// so loadSchemaFromOpts should return successfully.
func TestLoadSchemaFromOptsFileValid(t *testing.T) {
	// write a minimal-but-valid FIX XML to a temp file
	dir := t.TempDir()
	filename := filepath.Join(dir, "minimal.xml")
	const xmlContent = `<?xml version="1.0" encoding="UTF-8"?>
<fix major="1" minor="0">
  <fields>
    <field name="Foo" number="100" type="STRING"/>
  </fields>
  <components/>
  <messages/>
  <header/>
  <trailer/>
</fix>`
	if err := os.WriteFile(filename, []byte(xmlContent), 0o644); err != nil {
		t.Fatalf("failed to write test XML: %v", err)
	}

	opts := CLIOptions{XMLPath: filename}
	schema, err := loadSchemaFromOpts(opts)
	if err != nil {
		t.Fatalf("unexpected error loading valid file: %v", err)
	}

	// ensure the Version was read from major/minor
	if got := schema.Version; got != "1.0" {
		t.Errorf("schema.Version = %q; want \"1.0\"", got)
	}
	// ensure our named field exists
	if f, ok := schema.Fields["Foo"]; !ok {
		t.Fatal(`schema.Fields missing key "Foo"`)
	} else if f.Number != 100 || f.Type != "STRING" {
		t.Errorf("schema.Fields[\"Foo\"] = %+v; want Number=100, Type=STRING", f)
	}
}

// TestLoadSchemaFromOpts_EmbeddedValid verifies that when XMLPath is empty,
// we parse the embedded XML for a given FIX version.
func TestLoadSchemaFromOptsEmbeddedValid(t *testing.T) {
	// Use the known-good embedded FIX44 schema
	opts := CLIOptions{
		XMLPath:    "",
		FixVersion: "44",
	}

	schema, err := loadSchemaFromOpts(opts)
	if err != nil {
		t.Fatalf("unexpected error parsing embedded FIX44 XML: %v", err)
	}

	// The FIX44 spec has major="4" minor="4" → Version "4.4"
	if got := schema.Version; got != "4.4" {
		t.Errorf("schema.Version = %q; want \"4.4\"", got)
	}

	// And we should have at least one field defined
	if len(schema.Fields) == 0 {
		t.Error("schema.Fields is empty; expected at least one field")
	}
}

// TestLoadSchemaFromOpts_FileInvalidXML verifies that malformed XML produces an error.
func TestLoadSchemaFromOptsFileInvalidXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.xml")
	// Write malformed XML
	if err := os.WriteFile(path, []byte("<fix><broken></fix>"), 0644); err != nil {
		t.Fatalf("failed to write bad XML: %v", err)
	}

	opts := CLIOptions{XMLPath: path}
	_, err := loadSchemaFromOpts(opts)
	if err == nil {
		t.Fatal("expected unmarshal error for invalid XML, got nil")
	}
	// Optionally assert that it's an XML syntax error
	if !isXMLSyntaxError(err) {
		t.Errorf("unexpected error type: %v", err)
	}
}

// isXMLSyntaxError reports whether err was an XML unmarshalling error.
func isXMLSyntaxError(err error) bool {
	var syntaxErr *xml.SyntaxError
	return errors.As(err, &syntaxErr)
}
