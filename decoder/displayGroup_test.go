package decoder

import (
	"bytes"
	"testing"
)

func TestDisplayGroupBasic(t *testing.T) {
	group := GroupNode{
		Name:     "Group1",
		Required: "Y",
		Fields: []FieldNode{
			{FieldRef{Name: "F1", Required: "Y"}, Field{
				Name: "F1", Number: 10, Type: "INT",
				Values: []Value{{Enum: "A", Description: "Alpha"}},
			}},
		},
	}
	got := captureStdout(func() {
		DisplayGroup(SchemaTree{}, group, false, false, 0)
	})
	if want := "Group: Group1 - (Y)\n    10  : F1 (INT) - (Y)\n"; got[:len(want)] != want {
		t.Errorf("unexpected output: got %q, want %q", got, want)
	}
}

func TestDisplayGroupVerbose(t *testing.T) {
	group := GroupNode{
		Name: "Group2",
		Fields: []FieldNode{
			{FieldRef{Name: "F2"}, Field{
				Name: "F2", Number: 20, Type: "ENUM",
				Values: []Value{{Enum: "B", Description: "Beta"}},
			}},
		},
	}
	got := captureStdout(func() {
		DisplayGroup(SchemaTree{}, group, true, false, 2)
	})
	if !bytes.Contains([]byte(got), []byte("B : Beta")) {
		t.Errorf("expected verbose enum in output, got: %q", got)
	}
}

func TestDisplayGroupVerboseColumn(t *testing.T) {
	group := GroupNode{
		Name: "Group3",
		Fields: []FieldNode{
			{FieldRef{Name: "F3"}, Field{
				Name: "F3", Number: 30, Type: "ENUM",
				Values: []Value{{Enum: "C", Description: "Charlie"}},
			}},
		},
	}
	got := captureStdout(func() {
		DisplayGroup(SchemaTree{}, group, true, true, 0)
	})
	if !bytes.Contains([]byte(got), []byte("C: Charlie")) {
		t.Errorf("expected column enum output, got: %q", got)
	}
}

func TestDisplayGroupNestedComponentsAndGroups(t *testing.T) {
	nestedGroup := GroupNode{
		Name: "InnerGroup",
		Fields: []FieldNode{
			{FieldRef{Name: "F4"}, Field{Name: "F4", Number: 40, Type: "STR"}},
		},
	}
	nestedComp := ComponentNode{
		Name: "InnerComp",
		Fields: []FieldNode{
			{FieldRef{Name: "F5"}, Field{Name: "F5", Number: 50, Type: "FLOAT"}},
		},
	}
	group := GroupNode{
		Name:       "Outer",
		Fields:     []FieldNode{},
		Components: []ComponentNode{nestedComp},
		Groups:     []GroupNode{nestedGroup},
	}
	got := captureStdout(func() {
		DisplayGroup(SchemaTree{}, group, false, false, 0)
	})
	if !bytes.Contains([]byte(got), []byte("Component: InnerComp")) {
		t.Errorf("expected inner component, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Group: InnerGroup")) {
		t.Errorf("expected inner group, got %q", got)
	}
}
