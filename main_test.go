package main

import (
	"bytes"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
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

// TestTagFlag tests the tagFlag type.
func TestTagFlag(t *testing.T) {
	tf := &tagFlag{}

	// Initially, value should be empty and isSet false
	if got := tf.String(); got != "" {
		t.Errorf("initial String() = %q; want empty", got)
	}

	if tf.isSet {
		t.Error("initial isSet = true; want false")
	}

	// IsBoolFlag should return true
	if !tf.IsBoolFlag() {
		t.Error("IsBoolFlag() = false; want true")
	}

	// Set a value
	err := tf.Set("123")

	if err != nil {
		t.Errorf("Set returned error: %v", err)
	}

	// Now, value should be "123" and isSet true
	if tf.value != "123" {
		t.Errorf("value after Set = %q; want \"123\"", tf.value)
	}

	if !tf.isSet {
		t.Error("isSet = false; want true after Set")
	}

	// String should return the value
	if got := tf.String(); got != "123" {
		t.Errorf("String() after Set = %q; want \"123\"", got)
	}
}

// TestComponentFlag tests the componentFlag type.
func TestComponentFlag(t *testing.T) {
	cf := &componentFlag{}

	// Initially empty
	if got := cf.String(); got != "" {
		t.Errorf("initial String() = %q; want empty", got)
	}

	if cf.isSet {
		t.Error("initial isSet = true; want false")
	}

	if !cf.IsBoolFlag() {
		t.Error("IsBoolFlag() = false; want true")
	}

	// Set a component name
	err := cf.Set("CompName")

	if err != nil {
		t.Errorf("Set returned error: %v", err)
	}

	if cf.value != "CompName" {
		t.Errorf("value after Set = %q; want \"CompName\"", cf.value)
	}

	if !cf.isSet {
		t.Error("isSet = false; want true after Set")
	}

	if got := cf.String(); got != "CompName" {
		t.Errorf("String() after Set = %q; want \"CompName\"", got)
	}
}

// TestMessageFlag tests the messageFlag type.
func TestMessageFlag(t *testing.T) {
	mf := &messageFlag{}

	// Initially empty
	if got := mf.String(); got != "" {
		t.Errorf("initial String() = %q; want empty", got)
	}

	if mf.isSet {
		t.Error("initial isSet = true; want false")
	}

	if !mf.IsBoolFlag() {
		t.Error("IsBoolFlag() = false; want true")
	}

	// Set a message id
	err := mf.Set("MSG1")

	if err != nil {
		t.Errorf("Set returned error: %v", err)
	}

	if mf.value != "MSG1" {
		t.Errorf("value after Set = %q; want \"MSG1\"", mf.value)
	}

	if !mf.isSet {
		t.Error("isSet = false; want true after Set")
	}

	if got := mf.String(); got != "MSG1" {
		t.Errorf("String() after Set = %q; want \"MSG1\"", got)
	}
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

// resetFlags resets the global FlagSet and sets os.Args for a fresh parse.
func resetFlags(args []string) {
	// Replace the global FlagSet
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	// Reset os.Args
	os.Args = args
}

// TestParseFlags_Defaults verifies that parseFlags returns defaults when no args given.
func TestParseFlagsDefaults(t *testing.T) {
	resetFlags([]string{"cmd"})
	opts := parseFlags()

	want := CLIOptions{
		XMLPath:        "",
		FixVersion:     "44",
		Component:      componentFlag{value: "", isSet: false},
		Verbose:        false,
		IncludeHeader:  false,
		IncludeTrailer: false,
		ColumnOutput:   false,
		Message:        messageFlag{value: "", isSet: false},
		Tag:            tagFlag{value: "", isSet: false},
		Info:           false,
	}
	if !reflect.DeepEqual(opts, want) {
		t.Errorf("parseFlags() = %+v; want %+v", opts, want)
	}
}

// TestParseFlags_AllFlags verifies parseFlags picks up every flag with an explicit value.
func TestParseFlagsAllFlags(t *testing.T) {
	args := []string{
		"cmd",
		"-xml", "path/to/file.xml",
		"-fix", "50SP1",
		"-verbose",
		"-header",
		"-trailer",
		"-column",
		"-info",
		"-message=MSG",
		"-component=CompName",
		"-tag=123",
	}
	resetFlags(args)
	opts := parseFlags()

	if opts.XMLPath != "path/to/file.xml" {
		t.Errorf("XMLPath = %q; want %q", opts.XMLPath, "path/to/file.xml")
	}
	if opts.FixVersion != "50SP1" {
		t.Errorf("FixVersion = %q; want %q", opts.FixVersion, "50SP1")
	}
	if !opts.Verbose || !opts.IncludeHeader || !opts.IncludeTrailer || !opts.ColumnOutput || !opts.Info {
		t.Errorf("Boolean flags not set correctly: %+v", opts)
	}
	if !opts.Message.isSet || opts.Message.value != "MSG" {
		t.Errorf("Message flag = (%v,%q); want (true,MSG)", opts.Message.isSet, opts.Message.value)
	}
	if !opts.Component.isSet || opts.Component.value != "CompName" {
		t.Errorf("Component flag = (%v,%q); want (true,CompName)", opts.Component.isSet, opts.Component.value)
	}
	if !opts.Tag.isSet || opts.Tag.value != "123" {
		t.Errorf("Tag flag = (%v,%q); want (true,123)", opts.Tag.isSet, opts.Tag.value)
	}
}

// TestParseFlags_BareFlags verifies bare '-message', '-component', '-tag' set isSet and value="true".
func TestParseFlagsBareFlags(t *testing.T) {
	args := []string{"cmd", "-message", "-component", "-tag"}
	resetFlags(args)
	opts := parseFlags()

	if !opts.Message.isSet || opts.Message.value != "true" {
		t.Errorf("bare -message => (%v,%q); want (true,\"true\")", opts.Message.isSet, opts.Message.value)
	}
	if !opts.Component.isSet || opts.Component.value != "true" {
		t.Errorf("bare -component => (%v,%q); want (true,\"true\")", opts.Component.isSet, opts.Component.value)
	}
	if !opts.Tag.isSet || opts.Tag.value != "true" {
		t.Errorf("bare -tag => (%v,%q); want (true,\"true\")", opts.Tag.isSet, opts.Tag.value)
	}
}

func TestLoadSchemaSuccess(t *testing.T) {
	// Create a temporary directory
	dir := t.TempDir()
	// Path to the temp XML file
	path := filepath.Join(dir, "testfix.xml")
	// Write minimal valid FIX XML
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<fix major="1" minor="2">
  <fields>
    <field name="TestField" number="1" type="STRING"/>
  </fields>
  <components/>
  <messages/>
  <header/>
  <trailer/>
</fix>`
	if err := os.WriteFile(path, []byte(xmlContent), 0o644); err != nil {
		t.Fatalf("failed to write temp XML: %v", err)
	}

	schema, err := loadSchema(path)
	if err != nil {
		t.Fatalf("loadSchema returned unexpected error: %v", err)
	}

	// Version should be Major.Minor
	if schema.Version != "1.2" {
		t.Errorf("schema.Version = %q; want %q", schema.Version, "1.2")
	}
	// Fields map should contain our TestField
	f, ok := schema.Fields["TestField"]
	if !ok {
		t.Fatalf("schema.Fields missing TestField")
	}
	if f.Number != 1 || f.Type != "STRING" {
		t.Errorf("schema.Fields[TestField] = %+v; want Number=1, Type=STRING", f)
	}
}

func TestLoadSchemaFileNotFound(t *testing.T) {
	_, err := loadSchema("/nonexistent/path/fix.xml")
	if err == nil {
		t.Fatal("expected an error for nonexistent file, got nil")
	}
	// We don't require a specific error type, just that it's non-nil
}

func TestLoadSchemaInvalidXML(t *testing.T) {
	// Create a temp file with invalid XML
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.xml")
	if err := os.WriteFile(path, []byte("<fix><invalid></fix>"), 0o644); err != nil {
		t.Fatalf("failed to write bad XML: %v", err)
	}

	_, err := loadSchema(path)
	if err == nil {
		t.Fatal("expected unmarshal error for invalid XML, got nil")
	}
	// Optionally, check that it's an XML syntax error
}

// createMinimalFIXXML writes a minimal FIX XML schema to the given path.
func createMinimalFIXXML(t *testing.T, path string) {
	t.Helper()
	xmlContent := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<fix major=\"1\" minor=\"0\">\n" +
		"  <fields/>\n" +
		"  <components/>\n" +
		"  <messages/>\n" +
		"  <header/>\n" +
		"  <trailer/>\n" +
		"</fix>\n"
	if err := os.WriteFile(path, []byte(xmlContent), 0644); err != nil {
		t.Fatalf("failed to write minimal FIX XML: %v", err)
	}
}

// captureProcess runs Process with the given args and captures stdout, stderr, and exit code.
func captureProcess(args []string) (stdout, stderr string, exitCode int) {
	// Reset and configure flag parsing to use our args
	flag.CommandLine = flag.NewFlagSet(args[0], flag.ContinueOnError)
	os.Args = args

	var outBuf, errBuf bytes.Buffer
	// Pass the flag arguments (excluding program name) to Process
	exitCode = Process(args[1:], &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), exitCode
}

func TestProcessLoadSchemaError(t *testing.T) {
	stdout, stderr, code := captureProcess([]string{"cmd", "-xml", "no_such_file.xml"})
	if code != 1 {
		t.Errorf("exit code = %d; want 1 on load error", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q; want empty on load error", stdout)
	}
	if !strings.Contains(stderr, "no_such_file.xml") {
		t.Errorf("stderr = %q; want it to mention the missing file", stderr)
	}
}

// TestProcess_RunHandlersTrue ensures that when a handler (e.g., -info) matches,
// Process returns 0 and does not print usage.
func TestProcessRunHandlersTrue(t *testing.T) {
	// capture real stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Process([]string{"-info"}, nil, nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	stdout := buf.String()
	if code != 0 {
		t.Errorf("exit code = %d; want 0 when -info", code)
	}
	if !strings.Contains(stdout, "Available FIX Dictionaries:") {
		t.Errorf("stdout = %q; want it to contain \"Available FIX Dictionaries:\" when handler runs", stdout)
	}
}

// TestProcess_NoHandlerUsage ensures that with no flags Process prints usage and returns 1.
func TestProcessNoHandlerUsage(t *testing.T) {
	// capture real stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Process([]string{}, nil, nil)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = old

	stdout := buf.String()
	if code != 1 {
		t.Errorf("exit code = %d; want 1 when no handler", code)
	}
	if !strings.Contains(stdout, "fixdecoder") && !strings.Contains(stdout, "fixview") {
		t.Errorf("stdout = %q; want it to contain the program banner \"fixdecoder\" when no handler", stdout)
	}
}
