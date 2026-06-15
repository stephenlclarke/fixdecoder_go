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
)

// displayGroup displays a GroupNode with its count field and ordered children.
func DisplayGroup(schema SchemaTree, g GroupNode, verbose bool, columnOutput bool, indent int) {
	renderGroup(schema, MessageNode{}, g, verbose, columnOutput, indent)
}

// renderGroup keeps the No* count tag at the current indent and nests only group members.
func renderGroup(schema SchemaTree, msg MessageNode, g GroupNode, verbose bool, columnOutput bool, indent int) {
	if field, ok := schema.Fields[g.Name]; ok {
		renderField(
			FieldNode{Ref: FieldRef{Name: g.Name, Required: g.Required}, Field: field},
			msg,
			verbose,
			columnOutput,
			indent,
		)
	} else {
		printIndent(indent)
		fmt.Printf("Group: %s%s\n", g.Name, formatRequired(g.Required))
	}

	renderContainerEntries(schema, msg, groupEntries(g), verbose, columnOutput, indent+schemaGroupChildIndent)
}

// groupEntries returns ordered entries, falling back to legacy bucket fields for tests.
func groupEntries(g GroupNode) []ContainerNode {
	if len(g.Entries) > 0 {
		return g.Entries
	}

	entries := make([]ContainerNode, 0, len(g.Fields)+len(g.Components)+len(g.Groups))
	for _, field := range g.Fields {
		entries = append(entries, ContainerNode{Kind: containerField, Field: field})
	}
	for _, component := range g.Components {
		entries = append(entries, ContainerNode{Kind: containerComponent, Component: component})
	}
	for _, group := range g.Groups {
		entries = append(entries, ContainerNode{Kind: containerGroup, Group: group})
	}

	return entries
}

// printGroups prints all repeating groups of the message.
func printGroups(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, g := range msg.Groups {
		renderGroup(schema, msg, g, verbose, column, indent)
	}
}
