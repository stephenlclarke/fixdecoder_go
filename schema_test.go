package main

import (
	"encoding/xml"
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
