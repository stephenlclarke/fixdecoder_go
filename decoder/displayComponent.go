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

// display.go
package decoder

import (
	"fmt"
	"sort"
)

var printEnumFunc = printEnum

// listAllComponents prints all component names in sorted order.
func ListAllComponents(schema SchemaTree) {
	names := make([]string, 0, len(schema.Components))
	for name := range schema.Components {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Println(n)
	}
}

// printMatchingEnum prints only the value whose Enum matches `want`.
// It respects the existing indent conventions used by display helpers.
func printMatchingEnum(values []Value, want string, indent int) {
	for _, v := range values {
		if v.Enum == want {
			printEnumFunc(v.Enum, v.Description, indent)
			break
		}
	}
}

// printComponents prints all nested components of the message.
func printComponents(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, c := range msg.Components {
		DisplayComponent(schema, msg, c, verbose, column, indent)
	}
}

// printHeader prints the Header component if includeHeader is true.
func printHeader(schema SchemaTree, msg MessageNode, includeHeader, verbose, column bool, indent int) {
	if !includeHeader {
		return
	}

	if headerComp, ok := schema.Components["Header"]; ok {
		DisplayComponent(schema, msg, headerComp, verbose, column, indent)
	}
}

// printTrailer prints the Trailer component if includeTrailer is true.
func printTrailer(schema SchemaTree, msg MessageNode, includeTrailer, verbose, column bool, indent int) {
	if !includeTrailer {
		return
	}

	if trailerComp, ok := schema.Components["Trailer"]; ok {
		DisplayComponent(schema, msg, trailerComp, verbose, column, indent)
	}
}

func DisplayComponent(schema SchemaTree, msg MessageNode, comp ComponentNode, verbose bool, columnOutput bool, indent int) {
	renderComponent(schema, msg, comp, verbose, columnOutput, indent, indent+nestIndent)
}

// renderComponent prints a component label at headerIndent and its children at fieldIndent.
func renderComponent(
	schema SchemaTree,
	msg MessageNode,
	comp ComponentNode,
	verbose bool,
	columnOutput bool,
	headerIndent int,
	fieldIndent int,
) {
	printIndent(headerIndent)
	fmt.Printf("Component: %s\n", comp.Name)
	renderContainerEntries(schema, msg, componentEntries(comp), verbose, columnOutput, fieldIndent)
}

// renderContainerEntries renders fields, component headings, and group count fields in dictionary order.
func renderContainerEntries(
	schema SchemaTree,
	msg MessageNode,
	entries []ContainerNode,
	verbose bool,
	columnOutput bool,
	fieldIndent int,
) {
	for _, entry := range entries {
		switch entry.Kind {
		case containerField:
			renderField(entry.Field, msg, verbose, columnOutput, fieldIndent)
		case containerComponent:
			headerIndent := componentHeaderIndent(fieldIndent)
			renderComponent(schema, msg, entry.Component, verbose, columnOutput, headerIndent, fieldIndent)
		case containerGroup:
			renderGroup(schema, msg, entry.Group, verbose, columnOutput, fieldIndent)
		}
	}
}

// renderField prints a field plus optional enum detail using the shared indent rules.
func renderField(field FieldNode, msg MessageNode, verbose bool, columnOutput bool, indent int) {
	printField(field, indent)
	if verbose {
		printEnums(field, msg, columnOutput, indent+2)
	}
}

// componentEntries returns ordered entries, falling back to legacy bucket fields for tests.
func componentEntries(comp ComponentNode) []ContainerNode {
	if len(comp.Entries) > 0 {
		return comp.Entries
	}

	entries := make([]ContainerNode, 0, len(comp.Fields)+len(comp.Components)+len(comp.Groups))
	for _, field := range comp.Fields {
		entries = append(entries, ContainerNode{Kind: containerField, Field: field})
	}
	for _, component := range comp.Components {
		entries = append(entries, ContainerNode{Kind: containerComponent, Component: component})
	}
	for _, group := range comp.Groups {
		entries = append(entries, ContainerNode{Kind: containerGroup, Group: group})
	}

	return entries
}

// Helper to handle enum display logic
func printEnums(f FieldNode, msg MessageNode, columnOutput bool, indent int) {
	if f.Field.Number == 35 {
		// Special case for MsgType
		printMatchingEnum(f.Field.Values, msg.MsgType, indent)
		return
	}

	if columnOutput {
		printEnumColumns(f.Field.Values, indent)
	} else {
		for _, v := range f.Field.Values {
			printEnumFunc(v.Enum, v.Description, indent)
		}
	}
}
