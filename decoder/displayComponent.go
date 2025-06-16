// display.go
package decoder

import (
	"fmt"
	"sort"
)

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

func DisplayComponent(schema SchemaTree, comp ComponentNode, verbose bool, columnOutput bool, indent int) {
	printIndent(indent)
	fmt.Printf("Component: %s\n", comp.Name)

	for _, f := range comp.Fields {
		printField(f, indent+2)
		if verbose && columnOutput {
			printEnumColumns(f.Field.Values, indent+4)
		} else if verbose {
			for _, v := range f.Field.Values {
				printEnum(v.Enum, v.Description, indent+4)
			}
		}
	}

	for _, c := range comp.Components {
		DisplayComponent(schema, c, verbose, columnOutput, indent+2)
	}

	for _, g := range comp.Groups {
		DisplayGroup(schema, g, verbose, columnOutput, indent+2)
	}
}

// printComponents prints all nested components of the message.
func printComponents(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, c := range msg.Components {
		DisplayComponent(schema, c, verbose, column, indent)
	}
}

// printHeader prints the Header component if includeHeader is true.
func printHeader(schema SchemaTree, includeHeader, verbose, column bool, indent int) {
	if !includeHeader {
		return
	}

	if headerComp, ok := schema.Components["Header"]; ok {
		DisplayComponent(schema, headerComp, verbose, column, indent)
	}
}

// printTrailer prints the Trailer component if includeTrailer is true.
func printTrailer(schema SchemaTree, includeTrailer, verbose, column bool, indent int) {
	if !includeTrailer {
		return
	}

	if trailerComp, ok := schema.Components["Trailer"]; ok {
		DisplayComponent(schema, trailerComp, verbose, column, indent)
	}
}
