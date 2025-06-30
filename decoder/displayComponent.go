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
	printIndent(indent)
	fmt.Printf("Component: %s\n", comp.Name)

	for _, f := range comp.Fields {
		printField(f, indent+4)
		if verbose {
			printEnums(f, msg, columnOutput, indent+6)
		}
	}

	for _, c := range comp.Components {
		DisplayComponent(schema, msg, c, verbose, columnOutput, indent+4)
	}

	for _, g := range comp.Groups {
		DisplayGroup(schema, g, verbose, columnOutput, indent+4)
	}
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
