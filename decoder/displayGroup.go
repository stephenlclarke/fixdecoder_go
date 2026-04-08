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

// displayGroup displays a GroupNode with its fields, components, and nested groups.
func DisplayGroup(schema SchemaTree, g GroupNode, verbose bool, columnOutput bool, indent int) {
	printIndent(indent)
	fmt.Printf("Group: %s%s\n", g.Name, formatRequired(g.Required))

	for _, f := range g.Fields {
		printField(f, indent+4)
		if verbose && columnOutput {
			printEnumColumns(f.Field.Values, indent+6)
		} else if verbose {
			for _, val := range f.Field.Values {
				printEnum(val.Enum, val.Description, indent+6)
			}
		}
	}

	for _, c := range g.Components {
		DisplayComponent(schema, MessageNode{}, c, verbose, columnOutput, indent+4)
	}

	for _, sg := range g.Groups {
		DisplayGroup(schema, sg, verbose, columnOutput, indent+4)
	}
}

// printGroups prints all repeating groups of the message.
func printGroups(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, g := range msg.Groups {
		DisplayGroup(schema, g, verbose, column, indent)
	}
}
