package decoder

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrettifyWithEnum(t *testing.T) {
	loadDictionary = func(string) *FixTagLookup {
		return &FixTagLookup{
			tagToName: map[int]string{35: "MsgType"},
			enumMap: map[int]map[string]string{
				35: {"A": "Logon"},
			},
		}
	}

	parseFix = func(string) []FieldValue {
		return []FieldValue{{Tag: 35, Value: "A"}}
	}

	msg := "8=FIX.4.4\x0135=A\x0110=200\x01"
	output := Prettify(msg)
	if !strings.Contains(output, "MsgType") || !strings.Contains(output, "Logon") {
		t.Errorf("Expected decorated output with field name and enum, got: %s", output)
	}
}

func TestStreamLogWithFixMatch(t *testing.T) {
	loadDictionary = func(string) *FixTagLookup {
		return &FixTagLookup{
			tagToName: map[int]string{35: "MsgType"},
			enumMap: map[int]map[string]string{
				35: {"A": "Logon"},
			},
		}
	}

	parseFix = func(string) []FieldValue {
		return []FieldValue{{Tag: 35, Value: "A"}}
	}
	in := strings.NewReader("INFO 8=FIX.4.4\x0135=A\x0110=123\x01 more")
	var out bytes.Buffer
	err := streamLog(in, &out)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !strings.Contains(out.String(), "MsgType") {
		t.Error("Expected prettified FIX content in output")
	}
}

func TestStreamLogNoMatch(t *testing.T) {
	in := strings.NewReader("Just a regular log line")
	var out bytes.Buffer
	err := streamLog(in, &out)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !strings.Contains(out.String(), "Just a regular log line") {
		t.Error("Expected original line echoed")
	}
}

func TestPrettifyFilesStdin(t *testing.T) {
	loadDictionary = func(string) *FixTagLookup {
		return &FixTagLookup{
			tagToName: map[int]string{35: "MsgType"},
			enumMap: map[int]map[string]string{
				35: {"A": "Logon"},
			},
		}
	}

	parseFix = func(string) []FieldValue {
		return []FieldValue{{Tag: 35, Value: "A"}}
	}
	r, w, _ := os.Pipe()
	os.Stdin = r
	w.WriteString("8=FIX.4.4\x0135=A\x0110=123\x01\n")
	w.Close()

	var out, errOut bytes.Buffer
	code := PrettifyFiles([]string{}, &out, &errOut)
	if code != 0 {
		t.Errorf("Expected return code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "MsgType") {
		t.Error("Expected prettified FIX output from stdin")
	}
}

func TestPrettifyFileslsInvalidPath(t *testing.T) {
	var out, errOut bytes.Buffer
	code := PrettifyFiles([]string{"/path/does/not/exist"}, &out, &errOut)
	if code != 1 {
		t.Errorf("Expected return code 1 on error, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Cannot open file") {
		t.Error("Expected error output")
	}
}

func TestPrettifyFilesErrorReadingStdin(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	w.Close() // simulate EOF

	// Force error from streamLogFunc
	original := streamLogFunc
	streamLogFunc = func(in io.Reader, out io.Writer) error {
		return errors.New("mocked streamLog error")
	}
	defer func() { streamLogFunc = original }()

	var out, errOut bytes.Buffer
	code := PrettifyFiles([]string{}, &out, &errOut)

	if code != 1 {
		t.Errorf("Expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "Error reading input") {
		t.Errorf("Expected error message for stdin failure, got: %q", errOut.String())
	}
}

func TestPrettifyFilesReadFromDash(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, _ := os.Pipe()
	os.Stdin = r
	_, _ = w.WriteString("8=FIX.4.4\x0135=A\x01\n")
	w.Close()

	var out, errOut bytes.Buffer
	loadDictionary = func(string) *FixTagLookup {
		return &FixTagLookup{
			tagToName: map[int]string{35: "MsgType"},
		}
	}
	parseFix = func(msg string) []FieldValue {
		return []FieldValue{{Tag: 35, Value: "A"}}
	}

	code := PrettifyFiles([]string{"-"}, &out, &errOut)
	if code != 0 {
		t.Errorf("Expected code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "Processing: (stdin)") {
		t.Errorf("Expected stdin processing message")
	}
}

func TestPrettifyFilesStreamLogErrorOnFile(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "invalid")
	tmpFile.WriteString("not_a_fix_message_but_error_triggers")
	tmpFile.Close()

	// Override streamLog to force an error
	original := streamLogFunc
	streamLogFunc = func(r io.Reader, w io.Writer) error {
		return errors.New("mock error")
	}
	defer func() { streamLogFunc = original }()

	var out, errOut bytes.Buffer
	code := PrettifyFiles([]string{tmpFile.Name()}, &out, &errOut)

	if code != 1 {
		t.Errorf("Expected error code 1, got %d", code)
	}

	if !strings.Contains(errOut.String(), "Error reading file") {
		t.Errorf("Expected error reading file message")
	}
}

func TestPrettifyFilesSuccessCase(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "validfix")
	tmpFile.WriteString("8=FIX.4.4\x0135=A\x0110=123\x01\n")
	tmpFile.Close()

	var out, errOut bytes.Buffer
	loadDictionary = func(string) *FixTagLookup {
		return &FixTagLookup{
			tagToName: map[int]string{35: "MsgType"},
			enumMap: map[int]map[string]string{
				35: {"A": "Logon"},
			},
		}
	}
	parseFix = func(msg string) []FieldValue {
		return []FieldValue{{Tag: 35, Value: "A"}}
	}

	code := PrettifyFiles([]string{tmpFile.Name()}, &out, &errOut)
	if code != 0 {
		t.Errorf("Expected return code 0, got %d", code)
	}
	if !strings.Contains(out.String(), "MsgType") {
		t.Errorf("Expected output to include decoded FIX tag")
	}
}
