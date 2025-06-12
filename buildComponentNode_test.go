package main

import (
	"testing"
)

// TestBuildComponentNode_Simple tests building a component with only fields.
func TestBuildComponentNodeSimple(t *testing.T) {
	comp := Component{
		Name:   "C0",
		Fields: []FieldRef{{Name: "F1", Required: "Y"}},
	}
	fieldMap := map[string]Field{
		"F1": {Name: "F1", Number: 1, Type: "STRING"},
	}
	// No sub-components or groups
	node := buildComponentNode(comp, fieldMap, nil)
	// Verify name
	if node.Name != "C0" {
		t.Errorf("node.Name = %q; want %q", node.Name, "C0")
	}
	// Verify fields
	if len(node.Fields) != 1 {
		t.Fatalf("len(node.Fields) = %d; want 1", len(node.Fields))
	}
	fnode := node.Fields[0]
	if fnode.Field.Number != 1 || fnode.Field.Name != "F1" {
		t.Errorf("field = %+v; want Number=1,Name=F1", fnode.Field)
	}
	// No components
	if len(node.Components) != 0 {
		t.Errorf("len(node.Components) = %d; want 0", len(node.Components))
	}
	// No groups
	if len(node.Groups) != 0 {
		t.Errorf("len(node.Groups) = %d; want 0", len(node.Groups))
	}
}

// TestBuildComponentNode_WithSubcomponents tests that only known subcomponents are included.
func TestBuildComponentNodeWithSubcomponents(t *testing.T) {
	c1 := Component{Name: "C1", Fields: []FieldRef{{Name: "F1", Required: "N"}}}
	c2 := Component{
		Name:       "C2",
		Components: []ComponentRef{{Name: "C1", Required: "Y"}, {Name: "C3", Required: "N"}},
	}
	fieldMap := map[string]Field{"F1": {Name: "F1", Number: 1, Type: "STRING"}}
	compMap := map[string]Component{"C1": c1}
	node := buildComponentNode(c2, fieldMap, compMap)
	// Should include C1 but not C3
	if len(node.Components) != 1 {
		t.Fatalf("len(node.Components) = %d; want 1", len(node.Components))
	}
	if sub := node.Components[0]; sub.Name != "C1" {
		t.Errorf("subcomponent.Name = %q; want C1", sub.Name)
	}
}

// TestBuildComponentNode_NestedSubcomponents tests recursive nesting of subcomponents.
func TestBuildComponentNodeNestedSubcomponents(t *testing.T) {
	c1 := Component{Name: "C1"}
	c2 := Component{Name: "C2", Components: []ComponentRef{{Name: "C1", Required: "N"}}}
	c3 := Component{Name: "C3", Components: []ComponentRef{{Name: "C2", Required: "Y"}}}
	compMap := map[string]Component{"C1": c1, "C2": c2, "C3": c3}
	node := buildComponentNode(c3, nil, compMap)
	// Level 1 should be C2
	if len(node.Components) != 1 || node.Components[0].Name != "C2" {
		t.Fatalf("level1 = %+v; want C2", node.Components)
	}
	// Level 2 should be C1
	sub := node.Components[0]
	if len(sub.Components) != 1 || sub.Components[0].Name != "C1" {
		t.Errorf("level2 = %+v; want C1", sub.Components)
	}
}

// TestBuildComponentNode_WithGroups tests that groups and nested groups are built.
func TestBuildComponentNodeWithGroups(t *testing.T) {
	// group with one field
	fieldMap := map[string]Field{"X": {Name: "X", Number: 10, Type: "INT"}}
	g1 := Group{Name: "G1", Required: "Y", Fields: []FieldRef{{Name: "X", Required: "Y"}}}
	comp := Component{Name: "C0", Groups: []Group{g1}}
	node := buildComponentNode(comp, fieldMap, nil)
	if len(node.Groups) != 1 {
		t.Fatalf("len(node.Groups) = %d; want 1", len(node.Groups))
	}
	grp := node.Groups[0]
	if grp.Name != "G1" {
		t.Errorf("grp.Name = %q; want G1", grp.Name)
	}
	if len(grp.Fields) != 1 || grp.Fields[0].Field.Number != 10 {
		t.Errorf("grp.Fields = %+v; want FIELD X Number=10", grp.Fields)
	}
	// nested group inside g2
	g2 := Group{Name: "G2", Groups: []Group{g1}}
	comp2 := Component{Name: "C1", Groups: []Group{g2}}
	node2 := buildComponentNode(comp2, fieldMap, nil)
	if len(node2.Groups) != 1 {
		t.Fatalf("len(node2.Groups) = %d; want 1", len(node2.Groups))
	}
	nested := node2.Groups[0].Groups
	if len(nested) != 1 || nested[0].Name != "G1" {
		t.Errorf("nested groups = %+v; want one G1", nested)
	}
}
