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

package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/stephenlclarke/fixdecoder_go/decoder"
	"github.com/stephenlclarke/fixdecoder_go/fix"
)

func withCapturedStdout(out io.Writer, fn func()) {
	if out == nil {
		fn()
		return
	}

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		fn()
		return
	}

	os.Stdout = writer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(out, reader)
		_ = reader.Close()
		close(done)
	}()

	defer func() {
		_ = writer.Close()
		os.Stdout = originalStdout
		<-done
	}()

	fn()
}

// handleInfo prints a summary of the schema. Returns true if handled.
func handleInfo(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) bool {
	if !opts.Info {
		return false
	}

	if opts.XMLPath != "" {
		fmt.Fprintf(out, "Dictionary loaded from: %s%s%s\n\n", decoder.ColourError, opts.XMLPath, decoder.ColourReset)
	}

	fmt.Fprintf(out, "Available FIX Dictionaries: %s\n", fix.SupportedFixVersions())
	fmt.Fprintln(out, "Current Schema:")
	fmt.Fprintf(out, "  FIX Version:  %s\n", schema.Version)
	fmt.Fprintf(out, "  Service Pack: %s\n", schema.ServicePack)
	fmt.Fprintf(out, "  Messages:     %d\n", len(schema.Messages))
	fmt.Fprintf(out, "  Components:   %d\n", len(schema.Components))
	fmt.Fprintf(out, "  Fields:       %d\n", len(schema.Fields))
	return true
}

// handleMessage processes the --message flag. Returns true if handled.
func handleMessage(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) bool {
	if !opts.Message.isSet {
		return false
	}

	switch opts.Message.value {
	case "true": // bare --message
		handleBareMessage(schema, opts.ColumnOutput, out)
	case "": // explicit --message=
		PrintUsage(out)
	default:
		handleSpecificMessage(opts, schema, out)
	}

	return true
}

func handleBareMessage(schema decoder.SchemaTree, columnOutput bool, out io.Writer) {
	withCapturedStdout(out, func() {
		if columnOutput {
			decoder.PrintStringColumns(sortedMessageSummaries(schema))
		} else {
			decoder.ListAllMessages(schema)
		}
	})
}

func handleSpecificMessage(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) {
	message, found := findMessage(schema, opts.Message.value)
	if !found {
		fmt.Fprintf(out, "Message not found: %s\n", opts.Message.value)
		return
	}

	withCapturedStdout(out, func() {
		decoder.DisplayMessageStructureWithOptions(schema, message, opts.Verbose, opts.IncludeHeader, opts.IncludeTrailer, opts.ColumnOutput, 4)
	})
}

func sortedMessageSummaries(schema decoder.SchemaTree) []string {
	msgs := make([]string, 0, len(schema.Messages))
	for _, m := range schema.Messages {
		msgs = append(msgs, fmt.Sprintf("%2s: %s (%s)", m.MsgType, m.Name, m.MsgCat))
	}

	sort.Strings(msgs)
	return msgs
}

func findMessage(schema decoder.SchemaTree, query string) (decoder.MessageNode, bool) {
	for _, m := range schema.Messages {
		if m.Name == query || m.MsgType == query {
			return m, true
		}
	}

	return decoder.MessageNode{}, false
}

// handleTag processes the --tag flag. Returns true if handled.
func handleTag(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) bool {
	if !opts.Tag.isSet {
		return false
	}

	switch opts.Tag.value {
	case "true": // bare --tag
		handleBareTag(opts, schema, out)
	case "": // explicit --tag=
		PrintUsage(out)
	default:
		handleSpecificTag(opts, schema, out)
	}
	return true
}

func handleBareTag(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) {
	withCapturedStdout(out, func() {
		if opts.ColumnOutput {
			decoder.PrintTagsInColumns(schema)
		} else {
			decoder.ListAllTags(schema)
		}
	})
}

func handleSpecificTag(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) {
	id, err := strconv.Atoi(opts.Tag.value)
	if err != nil {
		fmt.Fprintf(out, "Invalid tag: %s\n", opts.Tag.value)
		return
	}

	field, found := decoder.FindField(schema, id)
	if !found {
		fmt.Fprintf(out, "Tag not found: %d\n", id)
		return
	}

	withCapturedStdout(out, func() {
		decoder.PrintTagDetails(field, opts.Verbose, opts.ColumnOutput)
	})
}

// handleComponent processes the --component flag. Returns true if handled.
func handleComponent(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) bool {
	if !opts.Component.isSet {
		return false
	}

	switch opts.Component.value {
	case "true": // bare --component
		handleBareComponent(opts, schema, out)
	case "": // explicit --component=
		PrintUsage(out)
	default:
		handleSpecificComponent(opts, schema, out)
	}
	return true
}

func handleBareComponent(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) {
	withCapturedStdout(out, func() {
		if opts.ColumnOutput {
			names := make([]string, 0, len(schema.Components))
			for name := range schema.Components {
				names = append(names, name)
			}
			sort.Strings(names)
			decoder.PrintStringColumns(names)
		} else {
			decoder.ListAllComponents(schema)
		}
	})
}

func handleSpecificComponent(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) {
	name := opts.Component.value

	if comp, ok := schema.Components[name]; ok {
		withCapturedStdout(out, func() {
			decoder.DisplayComponent(schema, decoder.MessageNode{}, comp, opts.Verbose, opts.ColumnOutput, 0)
		})
	} else {
		fmt.Fprintf(out, "Component not found: %s\n", name)
	}
}

// runHandlers invokes each of the "--info", "--message", "--tag", and "--component" handlers.
// It returns true if any handler succeeded.
func runHandlers(opts CLIOptions, schema decoder.SchemaTree, out io.Writer) bool {
	handled := false

	if handleInfo(opts, schema, out) {
		handled = true
	}

	if handleMessage(opts, schema, out) {
		handled = true
	}

	if handleTag(opts, schema, out) {
		handled = true
	}

	if handleComponent(opts, schema, out) {
		handled = true
	}

	return handled
}
