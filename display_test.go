package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Utility to capture all stdout from a function call
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()
	fn()
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

// Helper for Value (enum).
func makeEnum(val, desc string) Value {
	return Value{Enum: val, Description: desc}
}

func TestPrintTagDetailsAllBranches(t *testing.T) {
	field := Field{
		Number: 42,
		Name:   "TestTag",
		Type:   "STRING",
		Values: []Value{
			{Enum: "A", Description: "Apple"},
			{Enum: "B", Description: "Banana"},
		},
	}
	// 1. Not verbose: only header printed
	out := captureStdout(func() {
		printTagDetails(field, false, false)
	})
	want := "42: TestTag (STRING)\n"
	if out != want {
		t.Errorf("not verbose: got %q, want %q", out, want)
	}

	// 2. Verbose, column = false: prints each value using printEnum
	out = captureStdout(func() {
		printTagDetails(field, true, false)
	})
	if !strings.Contains(out, "  A: Apple") || !strings.Contains(out, "  B: Banana") {
		t.Errorf("verbose no-column: missing values, got %q", out)
	}
	if !strings.Contains(out, "42: TestTag (STRING)\n") {
		t.Errorf("verbose no-column: missing header, got %q", out)
	}

	// 3. Verbose, column = true: triggers printEnumColumns
	// (output comes from printEnumColumns)
	out = captureStdout(func() {
		printTagDetails(field, true, true)
	})
	// The output will include both values, and the header.
	if !strings.Contains(out, "42: TestTag (STRING)\n") {
		t.Errorf("verbose column: missing header, got %q", out)
	}
	if !strings.Contains(out, "A: Apple") || !strings.Contains(out, "B: Banana") {
		t.Errorf("verbose column: missing value strings, got %q", out)
	}

	// 4. Empty Values: only header, nothing else (with verbose)
	emptyField := Field{Number: 7, Name: "NoEnums", Type: "INT", Values: nil}
	out = captureStdout(func() {
		printTagDetails(emptyField, true, false)
	})
	if out != "7: NoEnums (INT)\n" {
		t.Errorf("empty values: got %q, want header only", out)
	}
}

func TestDisplayComponentBasic(t *testing.T) {
	schema := SchemaTree{}
	comp := ComponentNode{
		Name: "Comp1",
		Fields: []FieldNode{
			{FieldRef{Name: "F1"}, Field{Name: "F1", Number: 1, Type: "STRING"}},
		},
	}

	out := captureStdout(func() {
		displayComponent(schema, comp, false, false, 0)
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
		displayComponent(schema, comp, true, true, 0)
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
		displayComponent(schema, comp, true, false, 0)
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
		displayComponent(SchemaTree{}, comp, false, false, 0)
	})

	// Check nested component/group presence
	if !bytes.Contains([]byte(out), []byte("Component: Nested")) {
		t.Errorf("Missing nested component: %q", out)
	}
	if !bytes.Contains([]byte(out), []byte("Group: G1")) {
		t.Errorf("Missing group: %q", out)
	}
}

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
		displayGroup(SchemaTree{}, group, false, false, 0)
	})
	if want := "Group: Group1 - (Y)\n    10: F1 (INT) - (Y)\n"; got[:len(want)] != want {
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
		displayGroup(SchemaTree{}, group, true, false, 2)
	})
	if !bytes.Contains([]byte(got), []byte("B: Beta")) {
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
		displayGroup(SchemaTree{}, group, true, true, 0)
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
		displayGroup(SchemaTree{}, group, false, false, 0)
	})
	if !bytes.Contains([]byte(got), []byte("Component: InnerComp")) {
		t.Errorf("expected inner component, got %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Group: InnerGroup")) {
		t.Errorf("expected inner group, got %q", got)
	}
}

func TestPrintMessageStart(t *testing.T) {
	msg := MessageNode{Name: "OrderSingle", MsgType: "D"}
	out := captureOutput(func() {
		printMessageStart(msg)
	})
	want := "Message: OrderSingle (D)\n"
	if out != want {
		t.Errorf("printMessageStart output = %q; want %q", out, want)
	}
}

func TestPrintHeaderIncludeFalse(t *testing.T) {
	schema := SchemaTree{
		Components: map[string]ComponentNode{
			"Header": {Name: "Header"},
		},
	}
	// Should print nothing
	out := captureOutput(func() {
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
	out := captureOutput(func() {
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
	out := captureOutput(func() {
		printHeader(schema, true, false, false, 0)
	})
	// Should print nothing, as no Header exists
	if out != "" {
		t.Errorf("printHeader(header missing) output = %q; want empty", out)
	}
}
