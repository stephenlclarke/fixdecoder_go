package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureOutput captures stdout during f().
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = old
	return string(out)
}

// makeTestSchema constructs a test schema:
//   - Fields "A":1, "B":2
//   - Components "Comp1", "Comp2"
func makeTestSchema() SchemaTree {
	fields := map[string]Field{
		"A": {Name: "A", Number: 1, Type: "STRING"},
		"B": {Name: "B", Number: 2, Type: "INT"},
	}
	comps := map[string]ComponentNode{
		"Comp1": {Name: "Comp1"},
		"Comp2": {Name: "Comp2"},
	}
	return SchemaTree{Fields: fields, Components: comps}
}

// TestHandleTagCases covers the various -tag branches.
func TestHandleTagCases(t *testing.T) {
	schema := makeTestSchema()

	cases := []struct {
		tagValue     string
		column       bool
		wantContains []string
	}{
		{"", false, []string{"Usage:"}},              // explicit -tag=
		{"true", false, []string{"1: A", "2: B"}},    // bare -tag
		{"true", true, []string{"1:", "2:"}},         // bare -tag -column
		{"abc", false, []string{"Invalid tag: abc"}}, // parse error
		{"3", false, []string{"Tag not found: 3"}},   // not found
		{"2", false, []string{"2: B"}},               // exact match
	}

	for _, tc := range cases {
		opts := CLIOptions{
			Tag:          tagFlag{value: tc.tagValue, isSet: true},
			Verbose:      false,
			ColumnOutput: tc.column,
		}
		out := captureOutput(func() {
			handleTag(opts, schema)
		})
		for _, substr := range tc.wantContains {
			if !strings.Contains(out, substr) {
				t.Errorf("handleTag(%q, col=%v) missing %q; got %q",
					tc.tagValue, tc.column, substr, out)
			}
		}
	}
}

// TestHandleComponentCases covers the various -component branches.
func TestHandleComponentCases(t *testing.T) {
	schema := makeTestSchema()

	cases := []struct {
		compValue    string
		column       bool
		wantContains []string
	}{
		{"", false, []string{"Usage:"}},                  // explicit -component=
		{"true", false, []string{"Comp1", "Comp2"}},      // bare -component
		{"true", true, []string{"Comp1", "Comp2"}},       // bare -component -column
		{"X", false, []string{"Component not found: X"}}, // not found
		{"Comp2", false, []string{"Component: Comp2"}},   // exact match
	}

	for _, tc := range cases {
		opts := CLIOptions{
			Component:    componentFlag{value: tc.compValue, isSet: true},
			Verbose:      false,
			ColumnOutput: tc.column,
		}
		out := captureOutput(func() {
			handleComponent(opts, schema)
		})
		for _, substr := range tc.wantContains {
			if !strings.Contains(out, substr) {
				t.Errorf("handleComponent(%q, col=%v) missing %q; got %q",
					tc.compValue, tc.column, substr, out)
			}
		}
	}
}

func TestListAllMessages(t *testing.T) {
	schema := SchemaTree{
		Messages: map[string]MessageNode{
			"A": {Name: "MsgA", MsgType: "A", MsgCat: "cat1"},
			"B": {Name: "MsgB", MsgType: "B", MsgCat: "cat2"},
			"C": {Name: "MsgC", MsgType: "C", MsgCat: "cat3"},
		},
	}

	// Capture stdout
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listAllMessages(schema)

	w.Close()
	out, _ := io.ReadAll(r)
	os.Stdout = orig
	gotLines := strings.Split(strings.TrimSpace(string(out)), "\n")
	wantLines := []string{
		"A: MsgA (cat1)",
		" B: MsgB (cat2)",
		" C: MsgC (cat3)",
	}
	if len(gotLines) != len(wantLines) {
		t.Fatalf("got %d lines, want %d", len(gotLines), len(wantLines))
	}
	for i := range wantLines {
		if gotLines[i] != wantLines[i] {
			t.Errorf("Line %d: got %q, want %q", i, gotLines[i], wantLines[i])
		}
	}
}

// Optionally, test with no messages
func TestListAllMessagesEmpty(t *testing.T) {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	listAllMessages(SchemaTree{Messages: map[string]MessageNode{}})
	w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = origStdout
	if got := buf.String(); strings.TrimSpace(got) != "" {
		t.Errorf("Expected no output, got %q", got)
	}
}
