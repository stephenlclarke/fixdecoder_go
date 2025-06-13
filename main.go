// main.go
package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
)

// Version, Branch, GitUrl, Sha are injected at build time via -ldflags
var (
	Version = "0.0.0"
	Branch  = "main"
	GitUrl  = "git@bitbucket.org:edgewater/fixdecoder.git"
	Sha     = "0000000"
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

// messageFlag supports an optional string argument (with or without '=').
type messageFlag struct {
	value string
	isSet bool
}

func (m *messageFlag) String() string     { return m.value }
func (m *messageFlag) Set(s string) error { m.value, m.isSet = s, true; return nil }
func (m *messageFlag) IsBoolFlag() bool   { return true }

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

// parseFlagsArgs parses command-line arguments using a fresh FlagSet.
func parseFlagsArgs(args []string) CLIOptions {
	var message messageFlag
	var component componentFlag
	var tag tagFlag

	fs := flag.NewFlagSet("fixdecoder", flag.ContinueOnError)
	xmlPath := fs.String("xml", "", "Path to alternative FIX XML file")
	fixVersion := fs.String("fix", "44", "FIX version to use ("+supportedFixVersions()+")")
	verbose := fs.Bool("verbose", false, "Show full message structure with enums")
	includeHeader := fs.Bool("header", false, "Include Header block")
	includeTrailer := fs.Bool("trailer", false, "Include Trailer block")
	fs.Var(&message, "message", "Message name or MsgType (omit to list all messages)")
	fs.Var(&component, "component", "Component to display (omit to list all components)")
	fs.Var(&tag, "tag", "Tag number to display details for (omit to list all tags)")
	columnOutput := fs.Bool("column", false, "Display enums in columns")
	info := fs.Bool("info", false, "Show XML schema summary (fields, components, messages, version counts)")

	fs.Parse(args)

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

// parseFlags wraps parseFlagsArgs for legacy tests.
func parseFlags() CLIOptions {
	return parseFlagsArgs(os.Args[1:])
}

// printUsage prints the program usage.
func printUsage() {
	fmt.Printf("fixdecoder %s (branch:%s, commit:%s)\n\n", Version, Branch, Sha)
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

// Process is the entry point: parses flags, loads a schema, runs handlers, and returns an exit code.
func Process(args []string, out, errOut io.Writer) int {
	opts := parseFlagsArgs(args)

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

func main() {
	os.Exit(Process(os.Args[1:], os.Stdout, os.Stderr))
}
