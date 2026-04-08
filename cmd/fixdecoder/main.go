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

// main.go
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/stephenlclarke/fixdecoder_go/decoder"
	"github.com/stephenlclarke/fixdecoder_go/fix"
)

// Version, Branch, GitUrl, Sha are injected at build time via -ldflags
var (
	Version = "0.0.0"
	Branch  = "main"
	GitUrl  = "https://github.com/stephenlclarke/fixdecoder_go.git"
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
	FileArgs       []string
	Component      componentFlag
	Verbose        bool
	IncludeHeader  bool
	IncludeTrailer bool
	ColumnOutput   bool
	Message        messageFlag
	Tag            tagFlag
	Info           bool
}

// validateXMLFlag ensures the user supplied -xml=FILE syntax is correct.
// parseFlagsArgs parses command-line arguments using a fresh FlagSet.
func parseFlagsArgs(args []string, errOut io.Writer) (CLIOptions, error) {
	var message messageFlag
	var component componentFlag
	var tag tagFlag

	fs := flag.NewFlagSet("fixdecoder", flag.ContinueOnError)
	fs.SetOutput(errOut)
	xmlPath := fs.String("xml", "", "Path to alternative FIX XML file")
	fixVersion := fs.String("fix", "44", "FIX version to use ("+fix.SupportedFixVersions()+")")
	verbose := fs.Bool("verbose", false, "Show full message structure with enums")
	includeHeader := fs.Bool("header", false, "Include Header block")
	includeTrailer := fs.Bool("trailer", false, "Include Trailer block")
	fs.Var(&message, "message", "Message name or MsgType (omit to list all messages)")
	fs.Var(&component, "component", "Component to display (omit to list all components)")
	fs.Var(&tag, "tag", "Tag number to display details for (omit to list all tags)")
	columnOutput := fs.Bool("column", false, "Display enums in columns")
	info := fs.Bool("info", false, "Show XML schema summary (fields, components, messages, version counts)")

	fs.Usage = func() {
		PrintUsage(errOut)
		fmt.Fprintln(errOut, "\nFlags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(normalizeOptionalFlagArgs(args)); err != nil {
		return CLIOptions{}, err
	}

	return CLIOptions{
		XMLPath:        *xmlPath,
		FixVersion:     *fixVersion,
		FileArgs:       fileArgsOrStdin(fs.Args()),
		Component:      component,
		Verbose:        *verbose,
		IncludeHeader:  *includeHeader,
		IncludeTrailer: *includeTrailer,
		ColumnOutput:   *columnOutput,
		Message:        message,
		Tag:            tag,
		Info:           *info,
	}, nil
}

// printUsage prints the program usage.
func PrintUsage(out io.Writer) {
	fmt.Fprintf(out, "fixdecoder %s (branch:%s, commit:%s)\n\n", Version, Branch, Sha)
	fmt.Fprintf(out, "  git clone %s\n\n", GitUrl)
	fmt.Fprintln(out, "Usage: fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-message[=MSG] [-verbose] [-column] [-header] [-trailer]]")
	fmt.Fprintln(out, "       fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-tag[=TAG] [-verbose] [-column]]")
	fmt.Fprintln(out, "       fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-component=[NAME] [-verbose]]")
	fmt.Fprintln(out, "       fixdecoder [[-fix=44] | [-xml FIX44.xml]] [-info]")
	fmt.Fprintln(out, "       fixdecoder [file1.log file2.log ...]")
}

// loadSchema reads and parses the FIX XML into a SchemaTree.
func loadSchema(path string) (decoder.SchemaTree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return decoder.SchemaTree{}, err
	}

	var dict decoder.FixDictionary
	if err := xml.Unmarshal(data, &dict); err != nil {
		return decoder.SchemaTree{}, err
	}

	return decoder.BuildSchema(dict), nil
}

// loadLookup reads a FIX XML dictionary and turns it into a tag/enum lookup.
func loadLookup(path string) (*decoder.FixTagLookup, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return decoder.ParseDictionary(string(data))
}

func normalizeOptionalFlagArgs(args []string) []string {
	normalized := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			normalized = append(normalized, args[i:]...)
			break
		}

		if (arg == "-message" || arg == "-tag" || arg == "-component") &&
			i+1 < len(args) &&
			!strings.HasPrefix(args[i+1], "-") {
			normalized = append(normalized, arg+"="+args[i+1])
			i++
			continue
		}

		normalized = append(normalized, arg)
	}

	return normalized
}

func fileArgsOrStdin(args []string) []string {
	if len(args) == 0 {
		return []string{"-"}
	}

	return args
}

// Process is the entry point: parses flags, loads a schema, runs handlers, and returns an exit code.
func Process(args []string, out, errOut io.Writer) int {
	opts, err := parseFlagsArgs(args, errOut)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		return 2
	}

	if opts.XMLPath == "" && !fix.IsSupportedFixVersion(opts.FixVersion) {
		fmt.Fprintf(errOut, "Unsupported FIX version %q; continuing with FIX 4.4 fallback\n", opts.FixVersion)
	}

	var lookupOverride func(string) *decoder.FixTagLookup
	if opts.XMLPath != "" {
		lookup, err := loadLookup(opts.XMLPath)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}

		lookupOverride = func(string) *decoder.FixTagLookup {
			return lookup
		}
	}

	schema, err := loadSchemaFromOpts(opts)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}

	if runHandlers(opts, schema, out) {
		return 0
	}

	return decoder.PrettifyFilesWithDictionaryLoader(opts.FileArgs, out, errOut, lookupOverride)
}

// loadSchemaFromOpts picks between an explicit XML file or an embedded schema.
func loadSchemaFromOpts(opts CLIOptions) (decoder.SchemaTree, error) {
	if opts.XMLPath == "" {
		xmlData := fix.ChooseEmbeddedXML(opts.FixVersion)
		var dict decoder.FixDictionary
		if err := xml.Unmarshal([]byte(xmlData), &dict); err != nil {
			return decoder.SchemaTree{}, fmt.Errorf("failed to parse embedded FIX XML: %w", err)
		}

		return decoder.BuildSchema(dict), nil
	}

	return loadSchema(opts.XMLPath)
}

func main() {
	os.Exit(Process(os.Args[1:], os.Stdout, os.Stderr))
}
