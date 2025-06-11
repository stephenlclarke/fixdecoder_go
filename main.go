// main.go
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"bitbucket.org/edgewater/fixdecoder/fix40"
	"bitbucket.org/edgewater/fixdecoder/fix41"
	"bitbucket.org/edgewater/fixdecoder/fix42"
	"bitbucket.org/edgewater/fixdecoder/fix43"
	"bitbucket.org/edgewater/fixdecoder/fix44"
	"bitbucket.org/edgewater/fixdecoder/fix50"
)

// Version, Branch, GitUrl, Sha are injected at build time via -ldflags
var (
	Version = "0.0.0"
	Branch  = "main"
	GitUrl  = "git@bitbucket.org:edgewater/fixview.git"
	Sha     = "0000000"
)

// messageFlag supports an optional string argument (with or without '=').
type messageFlag struct {
	value string
	isSet bool
}

func (m *messageFlag) String() string {
	return m.value
}

func (m *messageFlag) Set(s string) error {
	m.value = s
	m.isSet = true
	return nil
}

func (m *messageFlag) IsBoolFlag() bool {
	return true
}

// CLIOptions holds all parsed flag values.
type CLIOptions struct {
	XMLPath        string
	FixVersion     string
	ComponentName  string
	Verbose        bool
	IncludeHeader  bool
	IncludeTrailer bool
	ColumnOutput   bool
	Message        messageFlag
	TagRaw         string
	Info           bool
}

// parseFlags parses command-line flags into CLIOptions.
func parseFlags() CLIOptions {
	xmlPath := flag.String("xml", "", "Path to alternative FIX XML file")
	fixVersion := flag.String("fix", "44", "FIX version to use (40,41,42,43,44,50)")
	componentName := flag.String("component", "", "Show the structure of the specified component")
	verbose := flag.Bool("verbose", false, "Show full message structure with enums")
	includeHeader := flag.Bool("header", false, "Include Header block")
	includeTrailer := flag.Bool("trailer", false, "Include Trailer block")
	var message messageFlag
	flag.Var(&message, "message", "Message ID to display details for (omit value to list all messages)")
	columnOutput := flag.Bool("column", false, "Display enums in columns")
	tagRaw := flag.String("tag", "", "Tag number to display details for (omit value to list all tags)")
	info := flag.Bool("info", false, "Show XML schema summary (fields, components, messages, version counts)")

	flag.Parse()

	return CLIOptions{
		XMLPath:        *xmlPath,
		FixVersion:     *fixVersion,
		ComponentName:  *componentName,
		Verbose:        *verbose,
		IncludeHeader:  *includeHeader,
		IncludeTrailer: *includeTrailer,
		ColumnOutput:   *columnOutput,
		Message:        message,
		TagRaw:         *tagRaw,
		Info:           *info,
	}
}

// printUsage prints the program usage.
func printUsage() {
	fmt.Printf("fixview %s (branch:%s, commit:%s)\n\n", Version, Branch, Sha)
	fmt.Printf("  git clone %s\n\n", GitUrl)
	fmt.Println("Usage: go run main.go [[-fix=44] | [-xml FIX44.xml]] [-message[=MSG] [-verbose] [-column] [-header] [-trailer]]")
	fmt.Println("       go run main.go [[-fix=44] | [-xml FIX44.xml]] [-tag[=TAG] [-verbose] [-column]]")
	fmt.Println("       go run main.go [[-fix=44] | [-xml FIX44.xml]] [-component <name> [-verbose]]")
	fmt.Println("       go run main.go [[-fix=44] | [-xml FIX44.xml]] [-info]")
}

// loadSchema reads and parses the FIX XML into a SchemaTree.
func loadSchema(path string) (SchemaTree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SchemaTree{}, err
	}

	var dict FixDictionary
	if err := xml.Unmarshal(data, &dict); err != nil {
		return SchemaTree{}, err
	}
	return buildSchema(dict), nil
}

// handleInfo prints a summary of the schema. Returns true if handled.
func handleInfo(opts CLIOptions, schema SchemaTree) bool {
	if !opts.Info {
		return false
	}

	fmt.Println("Schema summary:")
	fmt.Printf("  FIX Version: %s\n", schema.Version)
	fmt.Printf("  Messages:    %d\n", len(schema.Messages))
	fmt.Printf("  Components:  %d\n", len(schema.Components))
	fmt.Printf("  Fields:      %d\n", len(schema.Fields))
	return true
}

// handleMessage processes the -message flag. Returns true if handled.
func handleMessage(opts CLIOptions, schema SchemaTree) bool {
	if !opts.Message.isSet {
		return false
	}

	// list all messages
	if opts.Message.value == "" || opts.Message.value == "true" {
		var msgs []MessageNode
		for _, m := range schema.Messages {
			msgs = append(msgs, m)
		}

		sort.Slice(msgs, func(i, j int) bool { return msgs[i].MsgType < msgs[j].MsgType })
		for _, m := range msgs {
			fmt.Printf("%s: %s (%s)\n", m.MsgType, m.Name, m.MsgCat)
		}
		return true
	}

	// specific message
	for _, m := range schema.Messages {
		if m.Name == opts.Message.value || m.MsgType == opts.Message.value {
			displayMessageStructureWithOptions(schema, m, opts.Verbose, opts.IncludeHeader, opts.IncludeTrailer, opts.ColumnOutput, 2)
			return true
		}
	}

	fmt.Printf("Message not found: %s\n", opts.Message.value)
	return true
}

// handleTag processes the -tag flag. Returns true if handled.
func handleTag(opts CLIOptions, schema SchemaTree) bool {
	if !isTagSet() {
		return false
	}

	if opts.TagRaw == "" {
		listAllTags(schema)
		return true
	}

	tagID, err := parseTagID(opts.TagRaw)
	if err != nil {
		fmt.Printf("Invalid tag: %s\n", opts.TagRaw)
		return true
	}

	field, found := findField(schema, tagID)
	if !found {
		fmt.Printf("Tag not found: %d\n", tagID)
		return true
	}

	printTagDetails(field, opts.Verbose, opts.ColumnOutput)
	return true
}

// handleComponent processes the -component flag. Returns true if handled.
func handleComponent(opts CLIOptions, schema SchemaTree) bool {
	if opts.ComponentName == "" {
		return false
	}

	if comp, ok := schema.Components[opts.ComponentName]; ok {
		displayComponent(schema, comp, opts.Verbose, opts.ColumnOutput, 0)
	} else {
		fmt.Printf("Component not found: %s\n", opts.ComponentName)
	}
	return true
}

// Process is the entry point: parses flags, loads a schema, runs handlers, and returns an exit code.
func Process(args []string, out, errOut io.Writer) int {
	opts := parseFlags()

	schema, err := loadSchemaFromOpts(opts)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}

	if runHandlers(opts, schema) {
		return 0
	}

	printUsage()
	return 1
}

// loadSchemaFromOpts picks between an explicit XML file or an embedded schema.
func loadSchemaFromOpts(opts CLIOptions) (SchemaTree, error) {
	if opts.XMLPath == "" {
		xmlData := chooseEmbeddedXML(opts.FixVersion)
		var dict FixDictionary
		if err := xml.Unmarshal([]byte(xmlData), &dict); err != nil {
			return SchemaTree{}, fmt.Errorf("failed to parse embedded FIX XML: %w", err)
		}

		return buildSchema(dict), nil
	}

	return loadSchema(opts.XMLPath)
}

// chooseEmbeddedXML returns the raw XML constant for a given FIX version.
func chooseEmbeddedXML(ver string) string {
	switch ver {
	case "40":
		return fix40.FIX40XML
	case "41":
		return fix41.FIX41XML
	case "42":
		return fix42.FIX42XML
	case "43":
		return fix43.FIX43XML
	case "44":
		return fix44.FIX44XML
	case "50":
		return fix50.FIX50XML
	default:
		return fix44.FIX44XML
	}
}

// runHandlers invokes each of your "-info", "-message", "-tag", and "-component" handlers.
// It returns true if any handler succeeded.
func runHandlers(opts CLIOptions, schema SchemaTree) bool {
	handled := false

	if handleInfo(opts, schema) {
		handled = true
	}

	if handleMessage(opts, schema) {
		handled = true
	}

	if handleTag(opts, schema) {
		handled = true
	}

	if handleComponent(opts, schema) {
		handled = true
	}

	return handled
}

func main() {
	os.Exit(Process(os.Args[1:], os.Stdout, os.Stderr))
}
