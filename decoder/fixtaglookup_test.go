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
	"testing"

	"bitbucket.org/edgewater/fixdecoder/fix"
)

const sampleXML = `
<fix>
  <fields>
    <field name="TestField" number="1000">
      <value enum="A" description="Alpha"/>
      <value enum="B" description="Beta"/>
    </field>
  </fields>
  <messages>
    <message name="Heartbeat" msgtype="0" />
  </messages>
</fix>`

func TestParseDictionary(t *testing.T) {
	d, err := parseDictionary(sampleXML)
	if err != nil {
		t.Fatalf("parseDictionary failed: %v", err)
	}
	if got := d.GetFieldName(1000); got != "TestField" {
		t.Errorf("GetFieldName(1000) = %s, want TestField", got)
	}
	if got := d.GetEnumDescription(1000, "A"); got != "Alpha" {
		t.Errorf("GetEnumDescription(1000, A) = %s, want Alpha", got)
	}
	if got := d.enumMap[35]["0"]; got != "Heartbeat" {
		t.Errorf("MsgType 0 = %s, want Heartbeat", got)
	}
}

func TestGetTagValue(t *testing.T) {
	msg := "8=FIX.4.4\x019=123\x0135=A\x01"
	val, ok := getTagValue(msg, "35")
	if !ok || val != "A" {
		t.Errorf("getTagValue failed, got: %q, ok: %v", val, ok)
	}
	_, ok = getTagValue(msg, "999")
	if ok {
		t.Error("Expected false for missing tag")
	}
}

func TestDetectSchemaKey(t *testing.T) {
	tests := []struct {
		msg      string
		expected string
	}{
		{"8=FIX.4.2\x01", "FIX42"},
		{"8=FIXT.1.1\x011128=6\x01", "FIX44"},
		{"8=FIXT.1.1\x011128=8\x01", "FIX50SP1"},
		{"8=FIXT.1.1\x011128=999\x01", "FIX50"},
		{"", "FIX44"},
	}
	for _, tt := range tests {
		if got := detectSchemaKey(tt.msg); got != tt.expected {
			t.Errorf("detectSchemaKey(%q) = %q, want %q", tt.msg, got, tt.expected)
		}
	}
}

func TestMergeLookups(t *testing.T) {
	dst := &FixTagLookup{
		tagToName: map[int]string{1: "A"},
		enumMap:   map[int]map[string]string{1: {"A": "Alpha"}},
	}
	src := &FixTagLookup{
		tagToName: map[int]string{2: "B"},
		enumMap:   map[int]map[string]string{2: {"B": "Beta"}},
	}
	mergeLookups(dst, src)

	if dst.tagToName[2] != "B" {
		t.Error("mergeLookups failed to add tag name")
	}
	if dst.enumMap[2]["B"] != "Beta" {
		t.Error("mergeLookups failed to add enum description")
	}
}

func TestFixTagLookupGetFieldName(t *testing.T) {
	d := &FixTagLookup{tagToName: map[int]string{55: "Symbol"}}
	if d.GetFieldName(55) != "Symbol" {
		t.Error("GetFieldName failed for known tag")
	}
	if d.GetFieldName(9999) != "9999" {
		t.Error("GetFieldName fallback failed")
	}
}

func TestFixTagLookupGetEnumDescription(t *testing.T) {
	d := &FixTagLookup{
		enumMap: map[int]map[string]string{
			40: {"1": "Market", "2": "Limit"},
		},
	}
	if got := d.GetEnumDescription(40, "2"); got != "Limit" {
		t.Errorf("unexpected enum desc: %s", got)
	}
	if got := d.GetEnumDescription(40, "999"); got != "" {
		t.Error("expected empty string for missing enum")
	}
	if got := d.GetEnumDescription(999, "1"); got != "" {
		t.Error("expected empty string for missing tag")
	}
}

func TestParseDictionaryInvalidXML(t *testing.T) {
	_, err := parseDictionary("<invalid><xml>")
	if err == nil {
		t.Error("Expected error for malformed XML, got nil")
	}
}

func TestParseDictionaryValuesWrapper(t *testing.T) {
	xml := `
	<fix>
	  <fields>
	    <field name="TestField" number="1001">
	      <values>
	        <value enum="X" description="Extra"/>
	      </values>
	    </field>
	  </fields>
	</fix>`
	d, err := parseDictionary(xml)
	if err != nil {
		t.Fatalf("parseDictionary failed: %v", err)
	}
	got := d.GetEnumDescription(1001, "X")
	if got != "Extra" {
		t.Errorf("Expected enum description 'Extra', got %q", got)
	}
}

func TestMergeLookupsNil(t *testing.T) {
	// Line 139 coverage
	mergeLookups(nil, nil)             // no panic
	mergeLookups(&FixTagLookup{}, nil) // no panic
	mergeLookups(nil, &FixTagLookup{}) // no panic
}

func TestDetectSchemaKeyAllCases(t *testing.T) {
	// Full ApplVerID cases for FIXT.1.1
	cases := map[string]string{
		"0": "FIX27", "1": "FIX30", "2": "FIX40", "3": "FIX41",
		"4": "FIX42", "5": "FIX43", "6": "FIX44", "7": "FIX50",
		"8": "FIX50SP1", "9": "FIX50SP2", "x": "FIX50",
	}
	for id, want := range cases {
		msg := "8=FIXT.1.1\x01" + "1128=" + id + "\x01"
		got := detectSchemaKey(msg)
		if got != want {
			t.Errorf("ApplVerID %s: got %s, want %s", id, got, want)
		}
	}
}

func TestGetDictionaryInvalidKey(t *testing.T) {
	if getDictionary("NON_EXISTENT_VERSION") != nil {
		t.Error("Expected nil for unknown schema key")
	}
}

func TestGetDictionaryCachedFastPath(t *testing.T) {
	key := "FIX42"
	mock := &FixTagLookup{
		tagToName: map[int]string{11: "ClOrdID"},
		enumMap:   map[int]map[string]string{},
	}

	// Inject test dictionary into global cache
	dictMux.Lock()
	dicts[key] = mock
	dictMux.Unlock()

	got := getDictionary(key)
	if got == nil || got.GetFieldName(11) != "ClOrdID" {
		t.Fatal("Expected cached dictionary to be returned")
	}
}

func TestLoadDictionaryWithPreloadedKey(t *testing.T) {
	mock := &FixTagLookup{
		tagToName: map[int]string{8: "BeginString"},
		enumMap:   map[int]map[string]string{},
	}

	// Preload fallback dictionary
	dictMux.Lock()
	dicts["FIX44"] = mock
	dictMux.Unlock()

	msg := "8=FIX.4.4\x01"
	d := LoadDictionary(msg)
	if d == nil || d.GetFieldName(8) != "BeginString" {
		t.Error("Expected LoadDictionary to return fallback FIX44 dictionary")
	}
}
func TestLoadDictionaryFallbackToFIX44(t *testing.T) {
	// Ensure FIX44 is cached
	dictMux.Lock()
	dicts["FIX44"] = &FixTagLookup{
		tagToName: map[int]string{8: "BeginString"},
	}
	dictMux.Unlock()

	// Load with unknown BeginString → fallback path
	msg := "8=UNKNOWN.0\x01"
	d := LoadDictionary(msg)

	if d == nil || d.GetFieldName(8) != "BeginString" {
		t.Error("Expected LoadDictionary to return fallback FIX44 dictionary")
	}
}

func TestChooseEmbeddedXMLFIX50Parses(t *testing.T) {
	xml := fix.ChooseEmbeddedXML("FIX50")
	_, err := parseDictionary(xml)
	if err != nil {
		t.Fatalf("parseDictionary(FIX50) failed: %v", err)
	}
}

func TestGetDictionaryWithT11Merge(t *testing.T) {
	// Clear cache
	dicts = make(map[string]*FixTagLookup)

	// Manually preload FIXT11 without triggering getDictionary (no locking issue)
	t11 := &FixTagLookup{
		tagToName: map[int]string{1128: "ApplVerID"},
		enumMap:   map[int]map[string]string{},
	}
	dicts["FIXT11"] = t11

	// Now trigger getDictionary("FIX50") → will call mergeLookups
	d := getDictionary("FIX50")
	if d == nil {
		t.Fatal("Expected dictionary for FIX50")
	}

	if name := d.GetFieldName(1128); name != "ApplVerID" {
		t.Errorf("Expected ApplVerID to be merged from FIXT11, got %q", name)
	}
}

func TestGetDictionaryParseError(t *testing.T) {
	// Temporarily override the XML loader
	original := chooseEmbeddedXML
	chooseEmbeddedXML = func(ver string) string {
		return "<invalid><unclosed>" // malformed XML
	}
	defer func() { chooseEmbeddedXML = original }()

	// Clear cache for this key
	dictMux.Lock()
	delete(dicts, "FIX42")
	dictMux.Unlock()

	d := getDictionary("FIX42")
	if d != nil {
		t.Error("Expected nil on parse error")
	}
}
