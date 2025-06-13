package main

import (
	"strings"
	"testing"
)

func TestHandleInfoFlagFalse(t *testing.T) {
	opts := CLIOptions{Info: false}
	schema := SchemaTree{}
	// Should return false and produce no output
	out := captureStdout(func() {
		if handled := handleInfo(opts, schema); handled {
			t.Error("handleInfo returned true when opts.Info=false")
		}
	})
	if out != "" {
		t.Errorf("expected no output when opts.Info=false; got %q", out)
	}
}

func TestHandleInfoFlagTrue(t *testing.T) {
	// Prepare options and a sample schema
	opts := CLIOptions{Info: true}
	// Create sample schema with known counts
	schema := SchemaTree{
		Version:     "1.2",
		ServicePack: "SP1",
		Messages:    map[string]MessageNode{"M1": {}, "M2": {}},
		Components:  map[string]ComponentNode{"C1": {}, "C2": {}, "C3": {}},
		Fields:      map[string]Field{"F1": {}, "F2": {}, "F3": {}, "F4": {}},
	}
	// Capture output
	out := captureOutput(func() {
		if handled := handleInfo(opts, schema); !handled {
			t.Error("handleInfo returned false when opts.Info=true")
		}
	})

	// Expected lines
	expects := []string{
		"Available FIX Dictionaries: " + supportedFixVersions(),
		"Current Schema:",
		"  FIX Version:  1.2",
		"  Service Pack: SP1",
		"  Messages:     2",
		"  Components:   3",
		"  Fields:       4",
	}
	for _, exp := range expects {
		if !strings.Contains(out, exp) {
			t.Errorf("expected output to contain %q; got:\n%s", exp, out)
		}
	}
}

// makeTestSchema builds a schema with two messages for testing.
func makeMessageNodeTestSchema() SchemaTree {
	m1 := MessageNode{Name: "Alpha", MsgType: "A", MsgCat: "Cat1"}
	m2 := MessageNode{Name: "Beta", MsgType: "B", MsgCat: "Cat2"}
	schema := SchemaTree{
		Messages: map[string]MessageNode{"Alpha": m1, "Beta": m2},
	}
	return schema
}

func TestHandleMessageNotSet(t *testing.T) {
	schema := makeMessageNodeTestSchema()
	opts := CLIOptions{Message: messageFlag{isSet: false}}
	out := captureStdout(func() {
		if handled := handleMessage(opts, schema); handled {
			t.Errorf("expected handleMessage to return false when not set")
		}
	})
	if out != "" {
		t.Errorf("expected no output when not set; got %q", out)
	}
}

func TestHandleMessageBareNoColumn(t *testing.T) {
	schema := makeMessageNodeTestSchema()
	opts := CLIOptions{Message: messageFlag{value: "true", isSet: true}, ColumnOutput: false}
	out := captureStdout(func() {
		if !handleMessage(opts, schema) {
			t.Fatal("expected handleMessage to return true for bare flag")
		}
	})
	// Expect listing of two messages
	expects := []string{"A: Alpha (Cat1)", "B: Beta (Cat2)"}
	for _, exp := range expects {
		if !strings.Contains(out, exp) {
			t.Errorf("expected %q in output; got %q", exp, out)
		}
	}
}

func TestHandleMessageBareColumn(t *testing.T) {
	schema := makeMessageNodeTestSchema()
	opts := CLIOptions{Message: messageFlag{value: "true", isSet: true}, ColumnOutput: true}
	out := captureOutput(func() {
		if !handleMessage(opts, schema) {
			t.Fatal("expected handleMessage to return true for bare flag with column")
		}
	})
	// Should include both messages formatted
	expects := []string{"A: Alpha (Cat1)", "B: Beta (Cat2)"}
	for _, exp := range expects {
		if !strings.Contains(out, exp) {
			t.Errorf("expected column output %q; got %q", exp, out)
		}
	}
}

func TestHandleMessageExplicitEmpty(t *testing.T) {
	schema := makeMessageNodeTestSchema()
	opts := CLIOptions{Message: messageFlag{value: "", isSet: true}}
	out := captureOutput(func() {
		if !handleMessage(opts, schema) {
			t.Fatal("expected handleMessage to return true for explicit empty value")
		}
	})
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected usage in output; got %q", out)
	}
}

func TestHandleMessageSpecificFound(t *testing.T) {
	schema := makeMessageNodeTestSchema()
	opts := CLIOptions{Message: messageFlag{value: "Alpha", isSet: true}}
	out := captureOutput(func() {
		if !handleMessage(opts, schema) {
			t.Fatal("expected handleMessage to return true for found message")
		}
	})
	if !strings.Contains(out, "Message: Alpha (A)") {
		t.Errorf("expected detailed output; got %q", out)
	}
}

func TestHandleMessageSpecificNotFound(t *testing.T) {
	schema := makeMessageNodeTestSchema()
	opts := CLIOptions{Message: messageFlag{value: "Gamma", isSet: true}}
	out := captureOutput(func() {
		if !handleMessage(opts, schema) {
			t.Fatal("expected handleMessage to return true for not found message")
		}
	})
	if !strings.Contains(out, "Message not found: Gamma") {
		t.Errorf("expected not found output; got %q", out)
	}
}

func TestHandleTagNotSet(t *testing.T) {
	// Tag.isSet = false should return false and print nothing
	opts := CLIOptions{Tag: tagFlag{value: "", isSet: false}}
	schema := SchemaTree{}
	output := captureStdout(func() {
		ret := handleTag(opts, schema)
		if ret {
			t.Errorf("handleTag returned true when Tag.isSet=false")
		}
	})
	if output != "" {
		t.Errorf("expected no output when Tag.isSet=false, got %q", output)
	}
}

// TestHandleComponent_NotSet ensures that when opts.Component.isSet=false,
// handleComponent returns false and emits no output.
func TestHandleComponentNotSet(t *testing.T) {
	opts := CLIOptions{Component: componentFlag{value: "", isSet: false}}
	schema := SchemaTree{
		Fields:     make(map[string]Field),
		Messages:   make(map[string]MessageNode),
		Components: make(map[string]ComponentNode),
		Version:    "",
	}

	output := captureStdout(func() {
		ret := handleComponent(opts, schema)
		if ret {
			t.Errorf("handleComponent returned true when Component.isSet=false")
		}
	})

	if output != "" {
		t.Errorf("expected no output when Component.isSet=false, got %q", output)
	}
}

// makeSchemaWithMessages constructs a schema with dummy messages.
func makeSchemaWithMessages() SchemaTree {
	return SchemaTree{
		Messages: map[string]MessageNode{
			"Msg1": {Name: "Msg1", MsgType: "1", MsgCat: "Cat"},
		},
	}
}

// makeSchemaWithComponents constructs a schema with dummy components.
func makeSchemaWithComponents() SchemaTree {
	return SchemaTree{
		Components: map[string]ComponentNode{
			"Comp1": {Name: "Comp1"},
		},
	}
}

func TestRunHandlersNone(t *testing.T) {
	opts := CLIOptions{}
	schema := SchemaTree{}
	handled := runHandlers(opts, schema)
	if handled {
		t.Error("runHandlers returned true; want false when no flags set")
	}
}

func TestRunHandlersInfoOnly(t *testing.T) {
	opts := CLIOptions{Info: true}
	schema := SchemaTree{Version: "v", ServicePack: "sp", Messages: nil, Components: nil, Fields: nil}
	handled := runHandlers(opts, schema)
	if !handled {
		t.Error("runHandlers returned false; want true when Info=true")
	}
}

func TestRunHandlersMessageOnly(t *testing.T) {
	opts := CLIOptions{Message: messageFlag{value: "true", isSet: true}}
	schema := makeSchemaWithMessages()
	handled := runHandlers(opts, schema)
	if !handled {
		t.Error("runHandlers returned false; want true when Message.isSet=true")
	}
}

func TestRunHandlersComponentOnly(t *testing.T) {
	opts := CLIOptions{Component: componentFlag{value: "true", isSet: true}}
	schema := makeSchemaWithComponents()
	handled := runHandlers(opts, schema)
	if !handled {
		t.Error("runHandlers returned false; want true when Component.isSet=true")
	}
}

func TestRunHandlersMultiple(t *testing.T) {
	opts := CLIOptions{
		Info:      true,
		Message:   messageFlag{value: "true", isSet: true},
		Component: componentFlag{value: "true", isSet: true},
	}
	// Combine schemas
	schema := SchemaTree{
		Version:     "v",
		ServicePack: "sp",
		Messages:    makeSchemaWithMessages().Messages,
		Components:  makeSchemaWithComponents().Components,
	}
	handled := runHandlers(opts, schema)
	if !handled {
		t.Error("runHandlers returned false; want true when multiple flags set")
	}
}

// minimal schema with one field so handleTag can run
func makeSchemaWithOneField() SchemaTree {
	return SchemaTree{
		Fields: map[string]Field{
			"X": {Name: "X", Number: 1, Type: "STRING"},
		},
	}
}

func TestRunHandlersTagOnlyBranch(t *testing.T) {
	// Simulate only the tag flag being set
	opts := CLIOptions{
		Tag: tagFlag{value: "1", isSet: true},
	}
	schema := makeSchemaWithOneField()

	handled := runHandlers(opts, schema)
	if !handled {
		t.Errorf("runHandlers returned false; want true when Tag.isSet=true")
	}
}
