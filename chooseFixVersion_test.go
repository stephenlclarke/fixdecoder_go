package main

import (
	"strings"
	"testing"

	fix40 "bitbucket.org/edgewater/fixdecoder/fix40"
	fix41 "bitbucket.org/edgewater/fixdecoder/fix41"
	fix42 "bitbucket.org/edgewater/fixdecoder/fix42"
	fix43 "bitbucket.org/edgewater/fixdecoder/fix43"
	fix44 "bitbucket.org/edgewater/fixdecoder/fix44"
	fix50 "bitbucket.org/edgewater/fixdecoder/fix50"
	fix50SP1 "bitbucket.org/edgewater/fixdecoder/fix50SP1"
	fix50SP2 "bitbucket.org/edgewater/fixdecoder/fix50SP2"
	fixT11 "bitbucket.org/edgewater/fixdecoder/fixT11"
)

func TestChooseEmbeddedXML(t *testing.T) {
	tests := []struct {
		ver  string
		want string
	}{
		{"40", fix40.FIX40XML},
		{"41", fix41.FIX41XML},
		{"42", fix42.FIX42XML},
		{"43", fix43.FIX43XML},
		{"44", fix44.FIX44XML},
		{"50", fix50.FIX50XML},
		{"50SP1", fix50SP1.FIX50SP1XML},
		{"50SP2", fix50SP2.FIX50SP2XML},
		{"T11", fixT11.FIXT11XML},
		{"unknown", fix44.FIX44XML}, // default fallback
	}

	for _, tc := range tests {
		got := chooseEmbeddedXML(tc.ver)
		if got != tc.want {
			t.Errorf("chooseEmbeddedXML(%q) = got length %d, want length %d", tc.ver, len(got), len(tc.want))
		}
	}

}

// TestSupportedFixVersions checks the CSV list and its parsing.
func TestSupportedFixVersions(t *testing.T) {
	got := supportedFixVersions()
	expected := "40,41,42,43,44,50,50SP1,50SP2,T11"
	if got != expected {
		t.Errorf("supportedFixVersions() = %q; want %q", got, expected)
	}

	// Verify splitting and membership
	parts := strings.Split(got, ",")
	if len(parts) != 9 {
		t.Fatalf("supportedFixVersions has %d entries; want 9", len(parts))
	}

	wantSet := map[string]bool{
		"40":    true,
		"41":    true,
		"42":    true,
		"43":    true,
		"44":    true,
		"50":    true,
		"50SP1": true,
		"50SP2": true,
		"T11":   true,
	}

	for _, p := range parts {
		if !wantSet[p] {
			t.Errorf("unsupported version %q in supportedFixVersions", p)
		}
	}
}
