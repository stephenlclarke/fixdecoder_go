// main.go
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"bitbucket.org/edgewater/fixdecoder/fix40"
	"bitbucket.org/edgewater/fixdecoder/fix41"
	"bitbucket.org/edgewater/fixdecoder/fix42"
	"bitbucket.org/edgewater/fixdecoder/fix43"
	"bitbucket.org/edgewater/fixdecoder/fix44"
	"bitbucket.org/edgewater/fixdecoder/fix50"
	"bitbucket.org/edgewater/fixdecoder/fix50SP1"
	"bitbucket.org/edgewater/fixdecoder/fix50SP2"
	"bitbucket.org/edgewater/fixdecoder/fixT11"
)

// tagFlag supports optional string arg; bare -tag lists all, explicit -tag= shows usage, and -tag=NN selects a tag.
type tagFlag struct {
	value string
	isSet bool
}

func (t *tagFlag) String() string     { return t.value }
func (t *tagFlag) Set(s string) error { t.value, t.isSet = s, true; return nil }
func (t *tagFlag) IsBoolFlag() bool   { return true }

// componentFlag supports optional string arg; bare -component lists all, explicit -component= shows usage, and -component=NAME selects it.
type componentFlag struct {
	value string
	isSet bool
}

func (c *componentFlag) String() string     { return c.value }
func (c *componentFlag) Set(s string) error { c.value, c.isSet = s, true; return nil }
func (c *componentFlag) IsBoolFlag() bool   { return true }

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
	Component      componentFlag
	Verbose        bool
	IncludeHeader  bool
	IncludeTrailer bool
	ColumnOutput   bool
	Message        messageFlag
	Tag            tagFlag
	Info           bool
}

// parseFlags parses command-line flags into CLIOptions.
func parseFlags() CLIOptions {
	xmlPath := flag.String("xml", "", "Path to alternative FIX XML file")
	fixVersion := flag.String("fix", "44", "FIX version to use (40,41,42,43,44,50)")
	var component componentFlag
	var tag tagFlag
	verbose := flag.Bool("verbose", false, "Show full message structure with enums")
	includeHeader := flag.Bool("header", false, "Include Header block")
	includeTrailer := flag.Bool("trailer", false, "Include Trailer block")
	var message messageFlag
	flag.Var(&message, "message", "Message ID to display details for (omit value to list all messages)")
	flag.Var(&component, "component", "Component to display (omit to list all components)")
	flag.Var(&tag, "tag", "Tag number to display details for (omit to list all tags)")
	columnOutput := flag.Bool("column", false, "Display enums in columns")
	info := flag.Bool("info", false, "Show XML schema summary (fields, components, messages, version counts)")

	flag.Parse()

	return CLIOptions{
		XMLPath:        *xmlPath,
		FixVersion:     *fixVersion,
		Component:      component,
		Verbose:        *verbose,
		IncludeHeader:  *includeHeader,
		IncludeTrailer: *includeTrailer,
		ColumnOutput:   *columnOutput,
		Message:        message,
		Tag:            tag,
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
	fmt.Printf("  Service Pack: %s\n", schema.ServicePack)
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
	switch opts.Message.value {
	case "true": // bare -message
		if opts.ColumnOutput {
			// Collect messages in a slice for column output
			msgs := make([]string, 0, len(schema.Messages))
			for _, m := range schema.Messages {
				var msg = fmt.Sprintf("%s: %s (%s)", m.MsgType, m.Name, m.MsgCat)
				msgs = append(msgs, msg)
			}

			sort.Strings(msgs)
			printStringColumns(msgs)
		} else {
			listAllMessages(schema)
		}

	case "": // explicit -message=
		printUsage()
	default:
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
	return true
}

// handleTag processes the -tag flag. Returns true if handled.
func handleTag(opts CLIOptions, schema SchemaTree) bool {
	if !opts.Tag.isSet {
		return false
	}
	switch opts.Tag.value {
	case "true": // bare -tag
		if opts.ColumnOutput {
			nums := make([]string, 0, len(schema.Fields))
			for _, f := range schema.Fields {
				nums = append(nums, strconv.Itoa(f.Number))
			}
			sort.Strings(nums)
			printStringColumns(nums)
		} else {
			listAllTags(schema)
		}
	case "": // explicit -tag=
		printUsage()
	default:
		id, err := strconv.Atoi(opts.Tag.value)
		if err != nil {
			fmt.Printf("Invalid tag: %s\n", opts.Tag.value)
		} else {
			field, found := findField(schema, id)
			if !found {
				fmt.Printf("Tag not found: %d\n", id)
			} else {
				printTagDetails(field, opts.Verbose, opts.ColumnOutput)
			}
		}
	}
	return true
}

// handleComponent processes the -component flag. Returns true if handled.
func handleComponent(opts CLIOptions, schema SchemaTree) bool {
	if !opts.Component.isSet {
		return false
	}
	switch opts.Component.value {
	case "true": // bare -component
		if opts.ColumnOutput {
			names := make([]string, 0, len(schema.Components))
			for name := range schema.Components {
				names = append(names, name)
			}
			sort.Strings(names)
			printStringColumns(names)
		} else {
			listAllComponents(schema)
		}
	case "": // explicit -component=
		printUsage()
	default:
		name := opts.Component.value
		if comp, ok := schema.Components[name]; ok {
			displayComponent(schema, comp, opts.Verbose, opts.ColumnOutput, 0)
		} else {
			fmt.Printf("Component not found: %s\n", name)
		}
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
	case "50SP1":
		return fix50SP1.FIX50SP1XML
	case "50SP2":
		return fix50SP2.FIX50SP2XML
	case "T11":
		return fixT11.FIXT11XML
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
