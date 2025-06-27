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
