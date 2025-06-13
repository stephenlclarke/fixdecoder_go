package main

import (
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
