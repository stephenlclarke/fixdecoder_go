package main

import (
	"fmt"
	"sort"
	"strconv"
)

// handleInfo prints a summary of the schema. Returns true if handled.
func handleInfo(opts CLIOptions, schema SchemaTree) bool {
	if !opts.Info {
		return false
	}

	fmt.Printf("Available FIX Dictionaries: %s\n", supportedFixVersions())
	fmt.Printf("Current Schema:\n")
	fmt.Printf("  FIX Version:  %s\n", schema.Version)
	fmt.Printf("  Service Pack: %s\n", schema.ServicePack)
	fmt.Printf("  Messages:     %d\n", len(schema.Messages))
	fmt.Printf("  Components:   %d\n", len(schema.Components))
	fmt.Printf("  Fields:       %d\n", len(schema.Fields))
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
				var msg = fmt.Sprintf("%2s: %s (%s)", m.MsgType, m.Name, m.MsgCat)
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
		handleBareTag(opts, schema)
	case "": // explicit -tag=
		printUsage()
	default:
		handleSpecificTag(opts, schema)
	}
	return true
}

func handleBareTag(opts CLIOptions, schema SchemaTree) {
	if opts.ColumnOutput {
		printTagsInColumns(schema)
	} else {
		listAllTags(schema)
	}
}

func handleSpecificTag(opts CLIOptions, schema SchemaTree) {
	id, err := strconv.Atoi(opts.Tag.value)
	if err != nil {
		fmt.Printf("Invalid tag: %s\n", opts.Tag.value)
		return
	}

	field, found := findField(schema, id)
	if !found {
		fmt.Printf("Tag not found: %d\n", id)
		return
	}

	printTagDetails(field, opts.Verbose, opts.ColumnOutput)
}

// handleComponent processes the -component flag. Returns true if handled.
func handleComponent(opts CLIOptions, schema SchemaTree) bool {
	if !opts.Component.isSet {
		return false
	}

	switch opts.Component.value {
	case "true": // bare -component
		handleBareComponent(opts, schema)
	case "": // explicit -component=
		printUsage()
	default:
		handleSpecificComponent(opts, schema)
	}
	return true
}

func handleBareComponent(opts CLIOptions, schema SchemaTree) {
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
}

func handleSpecificComponent(opts CLIOptions, schema SchemaTree) {
	name := opts.Component.value

	if comp, ok := schema.Components[name]; ok {
		displayComponent(schema, comp, opts.Verbose, opts.ColumnOutput, 0)
	} else {
		fmt.Printf("Component not found: %s\n", name)
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
