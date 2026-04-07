// fixParser_test.go
package decoder

import (
	"reflect"
	"testing"
)

func TestParseFixValidFields(t *testing.T) {
	msg := "8=FIX.4.4\x019=112\x0135=A\x01"
	got := ParseFix(msg)

	want := []FieldValue{
		{Tag: 8, Value: "FIX.4.4"},
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
	msg := "8=FIX.4.4\x01BADFIELD\x0135=A\x01"
	got := ParseFix(msg)

	want := []FieldValue{
		{Tag: 8, Value: "FIX.4.4"},
		{Tag: 35, Value: "A"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expected valid fields only, got %v", got)
	}
}

func TestParseFixInvalidTagNumber(t *testing.T) {
	msg := "abc=value\x018=FIX.4.4\x01"
	got := ParseFix(msg)

	want := []FieldValue{
		{Tag: 8, Value: "FIX.4.4"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expected valid numeric tags only, got %v", got)
	}
}
