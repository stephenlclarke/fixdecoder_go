// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-FileCopyrightText: 2026 Steve Clarke <stephenlclarke@mac.com> - https://xyzzy.tools
//
/// fixdecoder command-line entry point and CLI orchestration.
///
/// The binary ties together the dictionary tooling and the streaming FIX log
/// prettifier.  This file is intentionally light on protocol logic; it wires
/// user input into the focused modules under `src/decoder` and `src/fix`.
/// The comments favour UK English and aim to give future maintainers a quick
/// reminder of why each function exists and how it cooperates with the rest
/// of the app.

package decoder

import (
	"bytes"
	"testing"
)

func TestPrintMessageStart(t *testing.T) {
	msg := MessageNode{Name: "OrderSingle", MsgType: "D"}
	out := captureStdout(func() {
		printMessageStart(msg)
	})

	want := "Message: OrderSingle (D)\n"
	if out != want {
		t.Errorf("printMessageStart output = %q; want %q", out, want)
	}
}

func TestDisplayMessageStructureWithOptionsBasic(t *testing.T) {
	msg := MessageNode{Name: "Msg", MsgType: "T"}
	schema := SchemaTree{}

	out := captureStdout(func() {
		DisplayMessageStructureWithOptions(schema, msg, false, false, false, false, 0)
	})

	want := "Message: Msg (T)\n"
	if out != want {
		t.Errorf("Basic: got %q; want %q", out, want)
	}
}

func TestDisplayMessageStructureWithOptionsHeaderAndTrailer(t *testing.T) {
	msg := MessageNode{Name: "M", MsgType: "X"}
	schema := SchemaTree{
		Components: map[string]ComponentNode{
			"Header":  {Name: "Header"},
			"Trailer": {Name: "Trailer"},
		},
	}

	out := captureStdout(func() {
		DisplayMessageStructureWithOptions(schema, msg, false, true, true, false, 2)
	})

	want := "Message: M (X)\n  Component: Header\n  Component: Trailer\n"
	if out != want {
		t.Errorf("Header+Trailer: got %q; want %q", out, want)
	}
}

func TestDisplayMessageStructureWithOptionsFieldsAndComponentsAndGroups(t *testing.T) {
	msg := MessageNode{
		Name:    "Msg",
		MsgType: "Z",
		Fields: []FieldNode{
			{Ref: FieldRef{Name: "F1", Required: "N"}, Field: Field{Name: "F1", Number: 1, Type: "STRING"}},
		},
		Components: []ComponentNode{{Name: "Comp1"}},
		Groups:     []GroupNode{{Name: "Grp1"}},
	}

	schema := SchemaTree{}
	out := captureStdout(func() {
		DisplayMessageStructureWithOptions(schema, msg, false, false, false, false, 1)
	})

	expectedLines := []string{
		"Message: Msg (Z)",
		" 1   : F1 (STRING)", // 1 space before 3 spaces after
		" Component: Comp1",  // 1 space before
		" Group: Grp1",       // 1 space before
	}

	for _, want := range expectedLines {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("output missing %q\nFull output:\n%s", want, out)
		}
	}
}

func TestDisplayMessageStructureWithOptionsAllVerboseColumn(t *testing.T) {
	msg := MessageNode{
		Name:    "Msg",
		MsgType: "Y",
		Fields: []FieldNode{
			{Ref: FieldRef{Name: "F2", Required: "Y"}, Field: Field{Name: "F2", Number: 2, Type: "INT", Values: []Value{{Enum: "A", Description: "Alpha"}}}},
		},
	}

	schema := SchemaTree{
		Components: map[string]ComponentNode{
			"Header":  {Name: "Header"},
			"Trailer": {Name: "Trailer"},
		},
	}

	out := captureStdout(func() {
		DisplayMessageStructureWithOptions(schema, msg, true, true, true, true, 0)
	})

	// Should contain message, header, field (with values), trailer
	expectedSnippets := []string{
		"Message: Msg (Y)",
		"Component: Header",
		"2   : F2 (INT) - (Y)",
		"A: Alpha",
		"Component: Trailer",
	}

	for _, want := range expectedSnippets {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("output missing %q\nFull output:\n%s", want, out)
		}
	}
}
