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

// fixParser_test.go
package decoder

import (
	"reflect"
	"testing"
)

const fixVersion44 = "FIX.4.4"

func TestParseFixValidFields(t *testing.T) {
	msg := "8=" + fixVersion44 + "\x019=112\x0135=A\x01"
	got := ParseFix(msg)

	want := []FieldValue{
		{Tag: 8, Value: fixVersion44},
		{Tag: 9, Value: "112"},
		{Tag: 35, Value: "A"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFix() = %v, want %v", got, want)
	}
}

func TestParseFixNoSOH(t *testing.T) {
	msg := "8=FIX.4.49=11235=A"
	if got := ParseFix(msg); got != nil {
		t.Errorf("Expected nil when no SOH, got %v", got)
	}
}

func TestParseFixEmptyFields(t *testing.T) {
	msg := "\x01\x01\x01" // only delimiters, no data
	got := ParseFix(msg)
	if len(got) != 0 {
		t.Errorf("Expected 0 parsed fields, got %d", len(got))
	}
}

func TestParseFixFieldWithoutEquals(t *testing.T) {
	msg := "8=" + fixVersion44 + "\x01BADFIELD\x0135=A\x01"
	got := ParseFix(msg)

	want := []FieldValue{
		{Tag: 8, Value: fixVersion44},
		{Tag: 35, Value: "A"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expected valid fields only, got %v", got)
	}
}

func TestParseFixInvalidTagNumber(t *testing.T) {
	msg := "abc=value\x018=" + fixVersion44 + "\x01"
	got := ParseFix(msg)

	want := []FieldValue{
		{Tag: 8, Value: fixVersion44},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expected valid numeric tags only, got %v", got)
	}
}
