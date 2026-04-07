package decoder

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestPrintSchemaSummaryPopulated(t *testing.T) {
	schema := SchemaTree{
		Fields: map[string]Field{
			"8": {}, "35": {}, "49": {},
		},
		Components: map[string]ComponentNode{
			"Header": {}, "Instrument": {},
		},
		Messages: map[string]MessageNode{
			"NewOrderSingle": {},
		},
		Version:     "FIX.4.4",
		ServicePack: "2",
	}

	output := captureOutput(func() {
		PrintSchemaSummary(schema)
	})

	expected := "Fields: 3   Components: 2   Messages: 1   Version: FIX.4.4  Service Pack: 2\n"
	if output != expected {
		t.Errorf("Unexpected output.\nGot:\n%q\nWant:\n%q", output, expected)
	}
}

func TestPrintSchemaSummaryEmpty(t *testing.T) {
	schema := SchemaTree{
		Fields:      map[string]Field{},
		Components:  map[string]ComponentNode{},
		Messages:    map[string]MessageNode{},
		Version:     "",
		ServicePack: "",
	}

	output := captureOutput(func() {
		PrintSchemaSummary(schema)
	})

	expected := strings.Fields("Fields: 0 Components: 0 Messages: 0 Version: Service Pack:")
	got := strings.Fields(output)

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Mismatch:\nGot:  %q\nWant: %q", got, expected)
	}
}
