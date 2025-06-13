// display.go
package main

import (
	"fmt"
)

// displayGroup displays a GroupNode with its fields, components, and nested groups.
func displayGroup(schema SchemaTree, g GroupNode, verbose bool, columnOutput bool, indent int) {
	printIndent(indent)
	fmt.Printf("Group: %s%s\n", g.Name, formatRequired(g.Required))

	for _, f := range g.Fields {
		printField(f, indent+2)
		if verbose && columnOutput {
			printEnumColumns(f.Field.Values, indent+4)
		} else if verbose {
			for _, val := range f.Field.Values {
				printEnum(val.Enum, val.Description, indent+4)
			}
		}
	}

	for _, c := range g.Components {
		displayComponent(schema, c, verbose, columnOutput, indent+2)
	}

	for _, sg := range g.Groups {
		displayGroup(schema, sg, verbose, columnOutput, indent+2)
	}
}

// printGroups prints all repeating groups of the message.
func printGroups(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, g := range msg.Groups {
		displayGroup(schema, g, verbose, column, indent)
	}
}
