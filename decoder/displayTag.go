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
	"os"
	"sort"
	"strings"
)

var printStringColumns = PrintStringColumns

const tagDetailEnumIndent = tagWidth + 2

// listAllTags prints every tag number, name, and type.
func ListAllTags(schema SchemaTree) {
	fields := make([]Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fields = append(fields, f)
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Number < fields[j].Number })
	for _, field := range fields {
		fmt.Println(tagDetailHeader(field))
	}
}

// printTagDetails prints a field's header and, if verbose, its enum values.
func PrintTagDetails(field Field, verbose, column bool) {
	fmt.Println(tagDetailHeader(field))
	if verbose {
		if column {
			printTagEnumColumns(field.Values, tagDetailEnumIndent)
		} else {
			printTagEnums(field.Values, tagDetailEnumIndent)
		}
	}
}

// tagDetailHeader formats a tag detail row with the decoded-message tag colours.
func tagDetailHeader(field Field) string {
	return fmt.Sprintf("%s%4d%s: %s%s%s (%s%s%s)",
		ColourTag, field.Number, ColourReset,
		ColourName, field.Name, ColourReset,
		ColourValue, field.Type, ColourReset,
	)
}

// printTagEnums prints enum values aligned under the tag detail field-name column.
func printTagEnums(values []Value, indent int) {
	sorted := sortedEnumValues(values)
	enumWidth := maxEnumWidth(sorted)
	for _, value := range sorted {
		printTagEnum(value, indent, enumWidth)
	}
}

// printTagEnum prints one enum value using decoded-message value/enum colours.
func printTagEnum(value Value, indent int, enumWidth int) {
	printIndent(indent)
	fmt.Println(colourTagEnumText(value, enumWidth))
}

// printTagEnumColumns prints enum values in columns while preserving enum-value padding.
func printTagEnumColumns(values []Value, indent int) {
	if len(values) == 0 {
		return
	}

	sorted := sortedEnumValues(values)

	enumWidth := maxEnumWidth(sorted)
	maxLen := 0
	for _, value := range sorted {
		textWidth := len(tagEnumText(value, enumWidth))
		if textWidth > maxLen {
			maxLen = textWidth
		}
	}

	width, _, err := getTerminalSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}

	usableWidth := width - indent
	if usableWidth <= 0 {
		usableWidth = width
	}

	cols := usableWidth / (maxLen + 2)
	if cols == 0 {
		cols = 1
	}

	rows := (len(sorted) + cols - 1) / cols
	for row := range rows {
		printIndent(indent)
		for col := range cols {
			index := col*rows + row
			if index >= len(sorted) {
				continue
			}
			plain := tagEnumText(sorted[index], enumWidth)
			fmt.Print(colourTagEnumText(sorted[index], enumWidth))
			fmt.Print(strings.Repeat(" ", maxLen+2-len(plain)))
		}
		fmt.Println()
	}
}

// sortedEnumValues returns a value-sorted copy so renderers do not mutate schema data.
func sortedEnumValues(values []Value) []Value {
	sorted := append([]Value(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Enum < sorted[j].Enum
	})

	return sorted
}

// tagEnumText formats enum details without ANSI escapes for width calculations.
func tagEnumText(value Value, enumWidth int) string {
	return fmt.Sprintf("%*s : %s", enumWidth, value.Enum, value.Description)
}

// colourTagEnumText formats enum details using decoded-message value and enum colours.
func colourTagEnumText(value Value, enumWidth int) string {
	return fmt.Sprintf("%s%*s%s : %s%s%s",
		ColourValue, enumWidth, value.Enum, ColourReset,
		ColourEnum, value.Description, ColourReset,
	)
}

// maxEnumWidth returns the visible width needed for aligned enum values.
func maxEnumWidth(values []Value) int {
	width := 1
	for _, value := range values {
		if len(value.Enum) > width {
			width = len(value.Enum)
		}
	}

	return width
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
		lines[i] = fmt.Sprintf("%4d: %s (%s)", f.Number, f.Name, f.Type)
	}

	printStringColumns(lines)
}
