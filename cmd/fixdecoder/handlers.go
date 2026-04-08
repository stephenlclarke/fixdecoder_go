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
	"os"
	"sort"
	"strconv"

	"bitbucket.org/edgewater/fixdecoder/decoder"
	"bitbucket.org/edgewater/fixdecoder/fix"
)

// handleXML is triggered when the user supplied -xml=FILE.
// It prints a short description of the external dictionary that has just
// been loaded, then returns true so runHandlers knows a handler fired.
func handleXML(opts CLIOptions, schema decoder.SchemaTree) bool {
	// Not our turn if -xml wasn’t given.
	if opts.XMLPath == "" {
		return false
	}

	// Re-use the same “info” formatter the other handlers use so the look
	// & feel stays identical.
	fmt.Printf("Dictionary loaded from: %s%s%s\n\n", decoder.ColourError, opts.XMLPath, decoder.ColourReset)

	decoder.PrintSchemaSummary(schema)

	return true
}

// handleInfo prints a summary of the schema. Returns true if handled.
func handleInfo(opts CLIOptions, schema decoder.SchemaTree) bool {
	if !opts.Info {
		return false
	}

	fmt.Printf("Available FIX Dictionaries: %s\n", fix.SupportedFixVersions())
	fmt.Printf("Current Schema:\n")
	fmt.Printf("  FIX Version:  %s\n", schema.Version)
	fmt.Printf("  Service Pack: %s\n", schema.ServicePack)
	fmt.Printf("  Messages:     %d\n", len(schema.Messages))
	fmt.Printf("  Components:   %d\n", len(schema.Components))
	fmt.Printf("  Fields:       %d\n", len(schema.Fields))
	return true
}

// handleMessage processes the -message flag. Returns true if handled.
func handleMessage(opts CLIOptions, schema decoder.SchemaTree) bool {
	if !opts.Message.isSet {
		return false
	}
	switch opts.Message.value {
	case "true": // bare -message
		if opts.ColumnOutput {
			// Collect messages in a slice for column output
			msgs := make([]string, 0, len(schema.Messages))
			for _, m := range schema.Messages {
				var msg = fmt.Sprintf("%2s: %s (%s)", m.MsgType, m.Name, m.MsgCat)
				msgs = append(msgs, msg)
			}

			sort.Strings(msgs)
			decoder.PrintStringColumns(msgs)
		} else {
			decoder.ListAllMessages(schema)
		}

	case "": // explicit -message=
		PrintUsage(os.Stdout)
	default:
		// specific message
		for _, m := range schema.Messages {
			if m.Name == opts.Message.value || m.MsgType == opts.Message.value {
				decoder.DisplayMessageStructureWithOptions(schema, m, opts.Verbose, opts.IncludeHeader, opts.IncludeTrailer, opts.ColumnOutput, 4)
				return true
			}
		}

		fmt.Printf("Message not found: %s\n", opts.Message.value)
		return true
	}
	return true
}

// handleTag processes the -tag flag. Returns true if handled.
func handleTag(opts CLIOptions, schema decoder.SchemaTree) bool {
	if !opts.Tag.isSet {
		return false
	}

	switch opts.Tag.value {
	case "true": // bare -tag
		handleBareTag(opts, schema)
	case "": // explicit -tag=
		PrintUsage(os.Stdout)
	default:
		handleSpecificTag(opts, schema)
	}
	return true
}

func handleBareTag(opts CLIOptions, schema decoder.SchemaTree) {
	if opts.ColumnOutput {
		decoder.PrintTagsInColumns(schema)
	} else {
		decoder.ListAllTags(schema)
	}
}

func handleSpecificTag(opts CLIOptions, schema decoder.SchemaTree) {
	id, err := strconv.Atoi(opts.Tag.value)
	if err != nil {
		fmt.Printf("Invalid tag: %s\n", opts.Tag.value)
		return
	}

	field, found := decoder.FindField(schema, id)
	if !found {
		fmt.Printf("Tag not found: %d\n", id)
		return
	}

	decoder.PrintTagDetails(field, opts.Verbose, opts.ColumnOutput)
}

// handleComponent processes the -component flag. Returns true if handled.
func handleComponent(opts CLIOptions, schema decoder.SchemaTree) bool {
	if !opts.Component.isSet {
		return false
	}

	switch opts.Component.value {
	case "true": // bare -component
		handleBareComponent(opts, schema)
	case "": // explicit -component=
		PrintUsage(os.Stdout)
	default:
		handleSpecificComponent(opts, schema)
	}
	return true
}

func handleBareComponent(opts CLIOptions, schema decoder.SchemaTree) {
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
}

func handleSpecificComponent(opts CLIOptions, schema decoder.SchemaTree) {
	name := opts.Component.value

	if comp, ok := schema.Components[name]; ok {
		decoder.DisplayComponent(schema, decoder.MessageNode{}, comp, opts.Verbose, opts.ColumnOutput, 0)
	} else {
		fmt.Printf("Component not found: %s\n", name)
	}
}

// runHandlers invokes each of the "-info", "-message", "-tag", and "-component" handlers.
// It returns true if any handler succeeded.
func runHandlers(opts CLIOptions, schema decoder.SchemaTree) bool {
	handled := handleXML(opts, schema)

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
