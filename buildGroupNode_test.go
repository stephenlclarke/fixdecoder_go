package main

import (
	"testing"
)

// makeTestGroupSchema returns maps for fields and components used in tests.
func makeTestGroupSchema() (map[string]Field, map[string]Component) {
	fieldMap := map[string]Field{
		"X": {Name: "X", Number: 1, Type: "STRING"},
		"Y": {Name: "Y", Number: 2, Type: "INT"},
	}
	compMap := map[string]Component{
		"C1": {Name: "C1", Fields: []FieldRef{{Name: "X", Required: "Y"}}},
		"C2": {Name: "C2"},
	}
	return fieldMap, compMap
}

// TestBuildGroupNodeBasic covers lines 161-165: basic node initialization.
func TestBuildGroupNodeBasic(t *testing.T) {
	fieldMap, compMap := makeTestGroupSchema()
	group := Group{
		Name:     "G0",
		Required: "Y",
		Fields:   []FieldRef{{Name: "X", Required: "Y"}},
	}
	node := buildGroupNode(group, fieldMap, compMap)

	// Verify Name and Required
	if node.Name != "G0" {
		t.Errorf("Name = %q; want G0", node.Name)
	}
	if node.Required != "Y" {
		t.Errorf("Required = %q; want Y", node.Required)
	}

	// Verify Fields via buildFieldNodes
	if len(node.Fields) != 1 {
		t.Fatalf("len(Fields) = %d; want 1", len(node.Fields))
	}
	fnode := node.Fields[0]
	if fnode.Field.Number != 1 || fnode.Field.Name != "X" {
		t.Errorf("FieldNode = %+v; want Number=1, Name=X", fnode.Field)
	}

	// No Components or Groups
	if len(node.Components) != 0 {
		t.Errorf("len(Components) = %d; want 0", len(node.Components))
	}
	if len(node.Groups) != 0 {
		t.Errorf("len(Groups) = %d; want 0", len(node.Groups))
	}
}

// TestBuildGroupNodeWithComponent covers lines 167-168: the if sub,ok branch.
func TestBuildGroupNodeWithComponent(t *testing.T) {
	fieldMap, compMap := makeTestGroupSchema()
	// Include C1 (known) and C3 (unknown)
	group := Group{
		Components: []ComponentRef{{Name: "C1", Required: "Y"}, {Name: "C3", Required: "N"}},
	}
	node := buildGroupNode(group, fieldMap, compMap)

	// Only C1 should be included
	if len(node.Components) != 1 {
		t.Fatalf("len(Components) = %d; want 1", len(node.Components))
	}
	if node.Components[0].Name != "C1" {
		t.Errorf("Component name = %q; want C1", node.Components[0].Name)
	}
}

// TestBuildGroupNodeWithGroups covers lines 171-172: the nested group branch.
func TestBuildGroupNodeWithGroups(t *testing.T) {
	fieldMap, compMap := makeTestGroupSchema()
	innerGroup := Group{Name: "G1", Fields: []FieldRef{{Name: "Y", Required: "N"}}}
	group := Group{
		Groups: []Group{innerGroup},
	}
	node := buildGroupNode(group, fieldMap, compMap)

	// Should include one nested group
	if len(node.Groups) != 1 {
		t.Fatalf("len(Groups) = %d; want 1", len(node.Groups))
	}
	if node.Groups[0].Name != "G1" {
		t.Errorf("Nested Group name = %q; want G1", node.Groups[0].Name)
	}

	// Check inner group's fields
	if len(node.Groups[0].Fields) != 1 || node.Groups[0].Fields[0].Field.Number != 2 {
		t.Errorf("Nested Fields = %+v; want Field Number=2", node.Groups[0].Fields)
	}
}

// TestBuildGroupNodeComplex covers both component and group loops together.
func TestBuildGroupNodeComplex(t *testing.T) {
	fieldMap, compMap := makeTestGroupSchema()
	// Add C2 to compMap for subcomponent test
	compMap["C2"] = Component{Name: "C2"}
	nestedGroup := Group{Name: "G2"}
	group := Group{
		Components: []ComponentRef{{Name: "C2", Required: "Y"}},
		Groups:     []Group{nestedGroup},
	}
	node := buildGroupNode(group, fieldMap, compMap)

	// Component present
	if len(node.Components) != 1 || node.Components[0].Name != "C2" {
		t.Errorf("Components = %v; want [C2]", node.Components)
	}
	// Group present
	if len(node.Groups) != 1 || node.Groups[0].Name != "G2" {
		t.Errorf("Groups = %v; want [G2]", node.Groups)
	}
}
