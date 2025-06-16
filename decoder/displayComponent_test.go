package decoder

import (
	"bytes"
	"testing"
)

func TestDisplayComponentBasic(t *testing.T) {
	schema := SchemaTree{}
	comp := ComponentNode{
		Name: "Comp1",
		Fields: []FieldNode{
			{FieldRef{Name: "F1"}, Field{Name: "F1", Number: 1, Type: "STRING"}},
		},
	}

	out := captureStdout(func() {
		DisplayComponent(schema, comp, false, false, 0)
	})

	if want := "Component: Comp1\n     1: F1 (STRING)\n"; out != want {
		t.Errorf("output = %q; want %q", out, want)
	}
}

func TestDisplayComponentVerboseColumn(t *testing.T) {
	schema := SchemaTree{}
	comp := ComponentNode{
		Name: "CompCol",
		Fields: []FieldNode{
			{FieldRef{Name: "F2"}, Field{
				Name: "F2", Number: 2, Type: "ENUM",
				Values: []Value{makeEnum("A", "Alpha"), makeEnum("B", "Beta")},
			}},
		},
	}

	out := captureStdout(func() {
		DisplayComponent(schema, comp, true, true, 0)
	})

	// Look for column output
	if !bytes.Contains([]byte(out), []byte("A: Alpha")) || !bytes.Contains([]byte(out), []byte("B: Beta")) {
		t.Errorf("Missing column enums in output: %q", out)
	}
}

func TestDisplayComponentVerboseNoColumn(t *testing.T) {
	schema := SchemaTree{}
	comp := ComponentNode{
		Name: "CompNoCol",
		Fields: []FieldNode{
			{FieldRef{Name: "F2"}, Field{
				Name: "F2", Number: 2, Type: "ENUM",
				Values: []Value{makeEnum("A", "Alpha"), makeEnum("B", "Beta")},
			}},
		},
	}

	out := captureStdout(func() {
		DisplayComponent(schema, comp, true, false, 0)
	})

	// Should contain indented enums
	if !bytes.Contains([]byte(out), []byte("A: Alpha")) || !bytes.Contains([]byte(out), []byte("B: Beta")) {
		t.Errorf("Missing enums in output: %q", out)
	}
}

func TestDisplayComponentNestedComponentsAndGroups(t *testing.T) {
	// Nested component inside main, and a group inside main
	group := GroupNode{
		Name: "G1",
		Fields: []FieldNode{
			{FieldRef{Name: "F3"}, Field{Name: "F3", Number: 3, Type: "INT"}},
		},
	}
	nested := ComponentNode{
		Name: "Nested",
		Fields: []FieldNode{
			{FieldRef{Name: "F4"}, Field{Name: "F4", Number: 4, Type: "STRING"}},
		},
	}
	comp := ComponentNode{
		Name:       "Main",
		Fields:     []FieldNode{{FieldRef{Name: "F1"}, Field{Name: "F1", Number: 1, Type: "INT"}}},
		Components: []ComponentNode{nested},
		Groups:     []GroupNode{group},
	}

	out := captureStdout(func() {
		DisplayComponent(SchemaTree{}, comp, false, false, 0)
	})

	// Check nested component/group presence
	if !bytes.Contains([]byte(out), []byte("Component: Nested")) {
		t.Errorf("Missing nested component: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("Group: G1")) {
		t.Errorf("Missing group: %q", out)
	}
}

func TestPrintHeaderIncludeFalse(t *testing.T) {
	schema := SchemaTree{
		Components: map[string]ComponentNode{
			"Header": {Name: "Header"},
		},
	}
	// Should print nothing
	out := captureStdout(func() {
		printHeader(schema, false, true, false, 0)
	})
	if out != "" {
		t.Errorf("printHeader(includeHeader=false) output = %q; want empty", out)
	}
}

func TestPrintHeaderIncludeTrueHeaderExists(t *testing.T) {
	schema := SchemaTree{
		Components: map[string]ComponentNode{
			"Header": {Name: "Header"},
		},
	}
	// Should print header component
	out := captureStdout(func() {
		printHeader(schema, true, false, false, 1)
	})
	if want := " Component: Header\n"; out != want {
		t.Errorf("printHeader(includeHeader=true) = %q; want %q", out, want)
	}
}

func TestPrintHeaderIncludeTrueHeaderMissing(t *testing.T) {
	schema := SchemaTree{
		Components: map[string]ComponentNode{},
	}
	out := captureStdout(func() {
		printHeader(schema, true, false, false, 0)
	})
	// Should print nothing, as no Header exists
	if out != "" {
		t.Errorf("printHeader(header missing) output = %q; want empty", out)
	}
}
