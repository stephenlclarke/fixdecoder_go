package decoder

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

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
		PrintTagDetails(field, false, false)
	})

	want := "42  : TestTag (STRING)\n"
	if out != want {
		t.Errorf("not verbose: got %q, want %q", out, want)
	}

	// 2. Verbose, column = false: prints each value using printEnum
	out = captureStdout(func() {
		PrintTagDetails(field, true, false)
	})

	if !strings.Contains(out, "  A : Apple") || !strings.Contains(out, "  B : Banana") {
		t.Errorf("verbose no-column: missing values, got %q", out)
	}

	if !strings.Contains(out, "42  : TestTag (STRING)\n") {
		t.Errorf("verbose no-column: missing header, got %q", out)
	}

	// 3. Verbose, column = true: triggers printEnumColumns
	// (output comes from printEnumColumns)
	out = captureStdout(func() {
		PrintTagDetails(field, true, true)
	})

	// The output will include both values, and the header.
	if !strings.Contains(out, "42  : TestTag (STRING)\n") {
		t.Errorf("verbose column: missing header, got %q", out)
	}

	if !strings.Contains(out, "    A: Apple   B: Banana  \n") {
		t.Errorf("verbose column: missing value strings, got %q", out)
	}

	// 4. Empty Values: only header, nothing else (with verbose)
	emptyField := Field{Number: 7, Name: "NoEnums", Type: "INT", Values: nil}
	out = captureStdout(func() {
		PrintTagDetails(emptyField, true, false)
	})

	if out != "7   : NoEnums (INT)\n" {
		t.Errorf("empty values: got %q, want header only", out)
	}
}

func TestListAllTags(t *testing.T) {
	schema := SchemaTree{
		Fields: map[string]Field{
			"Account": {Name: "Account", Number: 1, Type: "STRING"},
			"ClOrdID": {Name: "ClOrdID", Number: 11, Type: "STRING"},
			"OrderID": {Name: "OrderID", Number: 37, Type: "STRING"},
		},
	}

	// Capture stdout
	var buf bytes.Buffer
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ListAllTags(schema)

	// Restore stdout
	w.Close()
	os.Stdout = orig
	io.Copy(&buf, r)

	output := buf.String()

	expected := "1   : Account (STRING)\n" +
		"11  : ClOrdID (STRING)\n" +
		"37  : OrderID (STRING)\n"

	if output != expected {
		t.Errorf("Unexpected output:\nGot:\n%s\nWant:\n%s", output, expected)
	}
}

func TestPrintTagsInColumns(t *testing.T) {
	schema := SchemaTree{
		Fields: map[string]Field{
			"ClOrdID": {Name: "ClOrdID", Number: 11, Type: "STRING"},
			"Account": {Name: "Account", Number: 1, Type: "STRING"},
			"OrderID": {Name: "OrderID", Number: 37, Type: "STRING"},
		},
	}

	var got []string
	original := printStringColumns
	printStringColumns = func(lines []string) {
		got = lines
	}
	defer func() { printStringColumns = original }()

	PrintTagsInColumns(schema)

	want := []string{
		"1   : Account (STRING)",
		"11  : ClOrdID (STRING)",
		"37  : OrderID (STRING)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unexpected column output.\nGot:  %#v\nWant: %#v", got, want)
	}
}
