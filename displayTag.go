// display.go
package main

import (
	"fmt"
	"sort"
)

// listAllTags prints every tag number, name, and type.
func listAllTags(schema SchemaTree) {
	fields := make([]Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fields = append(fields, f)
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Number < fields[j].Number })
	for _, field := range fields {
		fmt.Printf("%4d: %s (%s)\n", field.Number, field.Name, field.Type)
	}
}

// printTagDetails prints a field's header and, if verbose, its enum values.
func printTagDetails(field Field, verbose, column bool) {
	fmt.Printf("%d: %s (%s)\n", field.Number, field.Name, field.Type)
	if verbose {
		if column {
			printEnumColumns(field.Values, 2)
		} else {
			for _, v := range field.Values {
				printEnum(v.Enum, v.Description, 2)
			}
		}
	}
}

func printTagsInColumns(schema SchemaTree) {
	fs := make([]Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fs = append(fs, f)
	}

	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Number < fs[j].Number
	})

	lines := make([]string, len(fs))
	for i, f := range fs {
		lines[i] = fmt.Sprintf("%4d: %s (%s)", f.Number, f.Name, f.Type)
	}

	printStringColumns(lines)
}
