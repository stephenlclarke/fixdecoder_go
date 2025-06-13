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

// TestPrintEnumColumns_EmptyValues covers the first if: len(values)==0
func TestPrintEnumColumnsEmptyValues(t *testing.T) {
	values := []Value{}
	out := captureStdout(func() {
		printEnumColumns(values, 0)
	})
	if out != "" {
		t.Errorf("expected no output for empty values, got %q", out)
	}
}

// TestPrintEnumColumns_TermSizeError covers the third if: term.GetSize error path
func TestPrintEnumColumnsTermSizeError(t *testing.T) {
	// On a pipe, term.GetSize will return an error → width = 80
	values := []Value{
		{Enum: "X", Description: "Y"},
	}
	out := captureStdout(func() {
		printEnumColumns(values, 0)
	})
	if !strings.Contains(out, "X: Y") {
		t.Errorf("expected printed enum \"X: Y\", got %q", out)
	}
}

// TestPrintEnumColumns_ZeroCols covers the fifth if: cols == 0 → cols=1
func TestPrintEnumColumnsZeroCols(t *testing.T) {
	// Create a very long description so maxLen+2 > usableWidth
	longDesc := strings.Repeat("Z", 100)
	values := []Value{
		{Enum: "E", Description: longDesc},
	}
	// Use indent large enough to make usableWidth small
	out := captureStdout(func() {
		printEnumColumns(values, 80) // usableWidth = 80-80 = 0 → reset to 80; maxLen+2 > 80 → cols = 0 → cols=1
	})
	// Should still print our single enum on one line
	if !strings.Contains(out, "E: "+longDesc) {
		t.Errorf("expected printed enum with long description, got %q", out)
	}
	// And exactly one line (plus newline)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected exactly 1 output line, got %d lines: %v", len(lines), lines)
	}
}

func makeTestMessageNode() MessageNode {
	return MessageNode{
		Fields: []FieldNode{
			{
				Field: Field{
					Number: 1,
					Name:   "Field1",
					Type:   "STRING",
					Values: []Value{
						{Enum: "EV1", Description: "Desc1"},
						{Enum: "EV2", Description: "Desc2"},
					},
				},
			}, {
				Field: Field{
					Number: 2,
					Name:   "Field2",
					Type:   "INT",
					Values: []Value{
						{Enum: "EVA", Description: "DescA"},
						{Enum: "EVB", Description: "DescB"},
					},
				},
			},
		},
	}
}

func TestPrintFieldsNoVerbose(t *testing.T) {
	msg := makeTestMessageNode()
	output := captureStdout(func() {
		printFields(msg, false, false, 2)
	})

	// Should not contain any enum values
	if strings.Contains(output, "EV1: Desc1") {
		t.Errorf("unexpected enum output when verbose=false: %q", output)
	}
}

func TestPrintFieldsVerboseNoColumn(t *testing.T) {
	msg := makeTestMessageNode()
	output := captureStdout(func() {
		printFields(msg, true, false, 2)
	})

	// Should list each enum on its own line
	expects := []string{"    EV1: Desc1", "    EV2: Desc2", "    EVA: DescA", "    EVB: DescB"}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected %q in output; got %q", exp, output)
		}
	}
}

func TestPrintFieldsVerboseColumn(t *testing.T) {
	msg := makeTestMessageNode()
	output := captureStdout(func() {
		printFields(msg, true, true, 0)
	})

	// Should contain all enum values in one or more columns
	expects := []string{"EV1: Desc1", "EV2: Desc2", "EVA: DescA", "EVB: DescB"}
	for _, exp := range expects {
		if !strings.Contains(output, exp) {
			t.Errorf("expected column output %q; got %q", exp, output)
		}
	}
}
