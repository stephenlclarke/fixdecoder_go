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

var printStringColumns = PrintStringColumns

// listAllTags prints every tag number, name, and type.
func ListAllTags(schema SchemaTree) {
	fields := make([]Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fields = append(fields, f)
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Number < fields[j].Number })
	for _, field := range fields {
		fmt.Printf("%-4d: %s (%s)\n", field.Number, field.Name, field.Type)
	}
}

// printTagDetails prints a field's header and, if verbose, its enum values.
func PrintTagDetails(field Field, verbose, column bool) {
	fmt.Printf("%-4d: %s (%s)\n", field.Number, field.Name, field.Type)
	if verbose {
		if column {
			printEnumColumns(field.Values, 4)
		} else {
			for _, v := range field.Values {
				printEnum(v.Enum, v.Description, 4)
			}
		}
	}
}

func PrintTagsInColumns(schema SchemaTree) {
	fs := make([]Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fs = append(fs, f)
	}

	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Number < fs[j].Number
	})

	lines := make([]string, len(fs))
	for i, f := range fs {
		lines[i] = fmt.Sprintf("%-4d: %s (%s)", f.Number, f.Name, f.Type)
	}

	printStringColumns(lines)
}
