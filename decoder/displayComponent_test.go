package decoder

import (
	"bytes"
	"io"
	"os"
	"strings"
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
		DisplayComponent(schema, MessageNode{}, comp, false, false, 0)
	})

	if want := "Component: Comp1\n    1   : F1 (STRING)\n"; out != want {
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
		DisplayComponent(schema, MessageNode{}, comp, true, true, 0)
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
		DisplayComponent(schema, MessageNode{}, comp, true, false, 0)
	})

	// Should contain indented enums
	if !bytes.Contains([]byte(out), []byte("A : Alpha")) || !bytes.Contains([]byte(out), []byte("B : Beta")) {
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
		DisplayComponent(SchemaTree{}, MessageNode{}, comp, false, false, 0)
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
		printHeader(schema, MessageNode{}, false, true, false, 0)
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
		printHeader(schema, MessageNode{}, true, false, false, 1)
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
		printHeader(schema, MessageNode{}, true, false, false, 0)
	})
	// Should print nothing, as no Header exists
	if out != "" {
		t.Errorf("printHeader(header missing) output = %q; want empty", out)
	}
}

func TestListAllComponents(t *testing.T) {
	schema := SchemaTree{
		Components: map[string]ComponentNode{
			"Instrument":    {},
			"Parties":       {},
			"OrderQtyData":  {},
			"NestedParties": {},
		},
	}

	// Capture stdout
	var buf bytes.Buffer
	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ListAllComponents(schema)

	// Restore stdout
	w.Close()
	os.Stdout = stdout
	io.Copy(&buf, r)

	output := buf.String()
	expected := strings.Join([]string{
		"Instrument",
		"NestedParties",
		"OrderQtyData",
		"Parties",
		"",
	}, "\n")

	if output != expected {
		t.Errorf("Expected sorted component list:\nGot:\n%s\nWant:\n%s", output, expected)
	}
}

func TestPrintMatchingEnumMatch(t *testing.T) {
	called := false
	var gotEnum, gotDesc string
	var gotIndent int

	original := printEnumFunc
	printEnumFunc = func(enum, desc string, indent int) {
		called = true
		gotEnum = enum
		gotDesc = desc
		gotIndent = indent
	}
	defer func() { printEnumFunc = original }()

	values := []Value{
		{Enum: "0", Description: "New"},
		{Enum: "1", Description: "Replace"},
		{Enum: "2", Description: "Cancel"},
	}

	printMatchingEnum(values, "1", 2)

	if !called {
		t.Fatal("Expected printEnumFunc to be called")
	}
	if gotEnum != "1" || gotDesc != "Replace" || gotIndent != 2 {
		t.Errorf("Unexpected values: got (%s, %s, %d)", gotEnum, gotDesc, gotIndent)
	}
}

func TestPrintMatchingEnumNoMatch(t *testing.T) {
	called := false

	original := printEnumFunc
	printEnumFunc = func(enum, desc string, indent int) {
		called = true
	}
	defer func() { printEnumFunc = original }()

	values := []Value{
		{Enum: "0", Description: "New"},
		{Enum: "1", Description: "Replace"},
	}

	printMatchingEnum(values, "X", 0)

	if called {
		t.Error("Expected printEnumFunc NOT to be called")
	}
}

func TestDisplayComponentMsgTypeEnumOnly(t *testing.T) {
	field := Field{
		Name:   "MsgType",
		Number: 35,
		Type:   "STRING",
		Values: []Value{
			{Enum: "D", Description: "NewOrderSingle"},
			{Enum: "F", Description: "OrderCancelRequest"},
		},
	}

	fieldNode := FieldNode{Field: field}

	component := ComponentNode{
		Fields: []FieldNode{fieldNode},
	}

	msg := MessageNode{
		MsgType: "F",
		Name:    "OrderCancelRequest",
		MsgCat:  "app",
	}

	var output bytes.Buffer
	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call with verbose=true, columnOutput=false, indent=0
	DisplayComponent(SchemaTree{}, msg, component, true, false, 0)

	w.Close()
	os.Stdout = stdout
	io.Copy(&output, r)

	got := output.String()
	if !strings.Contains(got, "F") || !strings.Contains(got, "OrderCancelRequest") {
		t.Errorf("Expected to print only matching MsgType enum, got:\n%s", got)
	}
}
