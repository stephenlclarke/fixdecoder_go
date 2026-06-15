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
	"strings"
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
	if want := "Group: Group1 - (Y)\n        10: F1 (INT) - (Y)\n"; got[:len(want)] != want {
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

func TestDisplayGroupCountFieldStaysOutsideMemberIndent(t *testing.T) {
	schema := SchemaTree{
		Fields: map[string]Field{
			"NoPartyIDs":    {Name: "NoPartyIDs", Number: 453, Type: "NUMINGROUP"},
			"PartyID":       {Name: "PartyID", Number: 448, Type: "STRING"},
			"NoPartySubIDs": {Name: "NoPartySubIDs", Number: 802, Type: "NUMINGROUP"},
			"PartySubID":    {Name: "PartySubID", Number: 523, Type: "STRING"},
		},
	}
	group := GroupNode{
		Name: "NoPartyIDs",
		Entries: []ContainerNode{
			{
				Kind:  containerField,
				Field: FieldNode{Ref: FieldRef{Name: "PartyID"}, Field: schema.Fields["PartyID"]},
			},
			{
				Kind: containerGroup,
				Group: GroupNode{
					Name: "NoPartySubIDs",
					Entries: []ContainerNode{
						{
							Kind:  containerField,
							Field: FieldNode{Ref: FieldRef{Name: "PartySubID"}, Field: schema.Fields["PartySubID"]},
						},
					},
				},
			},
		},
	}

	got := captureStdout(func() {
		DisplayGroup(schema, group, false, false, 0)
	})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected four rendered lines, got %d:\n%s", len(lines), got)
	}
	if leadingSpaces(lines[1]) != leadingSpaces(lines[0])+schemaGroupChildIndent {
		t.Fatalf("group count should be outside member indent:\n%s", got)
	}
	if leadingSpaces(lines[3]) != leadingSpaces(lines[2])+schemaGroupChildIndent {
		t.Fatalf("nested group count should be outside member indent:\n%s", got)
	}
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
