// display.go
package main

import (
	"fmt"
	"os"
	"sort"

	"strings"

	"golang.org/x/term"
)

// listAllTags prints every tag number, name, and type.
func listAllTags(schema SchemaTree) {
	fields := make([]Field, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		fields = append(fields, f)
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Number < fields[j].Number })
	for _, field := range fields {
		fmt.Printf("%d: %s (%s)\n", field.Number, field.Name, field.Type)
	}
}

// listAllComponents prints all component names in sorted order.
func listAllComponents(schema SchemaTree) {
	names := make([]string, 0, len(schema.Components))
	for name := range schema.Components {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Println(n)
	}
}

// listAllMessages prints all messages in sorted order by MsgType.
func listAllMessages(schema SchemaTree) {
	var msgs []MessageNode
	for _, m := range schema.Messages {
		msgs = append(msgs, m)
	}

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].MsgType < msgs[j].MsgType })
	for _, m := range msgs {
		fmt.Printf("%s: %s (%s)\n", m.MsgType, m.Name, m.MsgCat)
	}
}

// findField returns the Field with the given number, or false if not found.
func findField(schema SchemaTree, tagID int) (Field, bool) {
	for _, f := range schema.Fields {
		if f.Number == tagID {
			return f, true
		}
	}
	return Field{}, false
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

func displayComponent(schema SchemaTree, comp ComponentNode, verbose bool, columnOutput bool, indent int) {
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
		displayComponent(schema, c, verbose, columnOutput, indent+2)
	}

	for _, g := range comp.Groups {
		displayGroup(schema, g, verbose, columnOutput, indent+2)
	}
}

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

// printMessageStart prints the “Message: Name (Type)” header.
func printMessageStart(msg MessageNode) {
	fmt.Printf("Message: %s (%s)\n", msg.Name, msg.MsgType)
}

// printHeader prints the Header component if includeHeader is true.
func printHeader(schema SchemaTree, includeHeader, verbose, column bool, indent int) {
	if !includeHeader {
		return
	}

	if headerComp, ok := schema.Components["Header"]; ok {
		displayComponent(schema, headerComp, verbose, column, indent)
	}
}

func printField(field FieldNode, indent int) {
	printIndent(indent)
	fmt.Printf("%d: %s (%s)%s\n",
		field.Field.Number, field.Field.Name, field.Field.Type, formatRequired(field.Ref.Required),
	)
}

// printStringColumns prints a slice of strings in columns based on terminal width.
func printStringColumns(items []string) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}
	maxLen := 0
	for _, s := range items {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	cols := width / (maxLen + 2)
	if cols == 0 {
		cols = 1
	}
	rows := (len(items) + cols - 1) / cols
	for r := range make([]int, rows) {
		for c := range make([]int, cols) {
			i := c*rows + r
			if i < len(items) {
				fmt.Printf("%-*s", maxLen+2, items[i])
			}
		}
		fmt.Println()
	}
}

// printFields prints all the simple fields of the message.
func printFields(msg MessageNode, verbose, column bool, indent int) {
	for _, f := range msg.Fields {
		printField(f, indent)

		if verbose && column {
			printEnumColumns(f.Field.Values, indent+4)
		} else if verbose {
			for _, val := range f.Field.Values {
				printEnum(val.Enum, val.Description, indent+4)
			}
		}
	}
}

// printComponents prints all nested components of the message.
func printComponents(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, c := range msg.Components {
		displayComponent(schema, c, verbose, column, indent)
	}
}

// printGroups prints all repeating groups of the message.
func printGroups(schema SchemaTree, msg MessageNode, verbose, column bool, indent int) {
	for _, g := range msg.Groups {
		displayGroup(schema, g, verbose, column, indent)
	}
}

// printFooter prints the Trailer component if includeTrailer is true.
func printFooter(schema SchemaTree, includeTrailer, verbose, column bool, indent int) {
	if !includeTrailer {
		return
	}

	if trailerComp, ok := schema.Components["Trailer"]; ok {
		displayComponent(schema, trailerComp, verbose, column, indent)
	}
}

// displayMessageStructureWithOptions orchestrates the above helpers.
func displayMessageStructureWithOptions(
	schema SchemaTree,
	msg MessageNode,
	verbose, includeHeader, includeTrailer, column bool,
	indent int,
) {
	printMessageStart(msg)
	printHeader(schema, includeHeader, verbose, column, indent)
	printFields(msg, verbose, column, indent)
	printComponents(schema, msg, verbose, column, indent)
	printGroups(schema, msg, verbose, column, indent)
	printFooter(schema, includeTrailer, verbose, column, indent)
}

func printIndent(level int) {
	fmt.Print(strings.Repeat(" ", level))
}

func printEnum(enum string, description string, indent int) {
	printIndent(indent)
	fmt.Printf("%s: %s\n", enum, description)
}

func formatRequired(req string) string {
	if req == "Y" {
		return " - (Y)"
	}

	return ""
}

func printEnumColumns(values []Value, indent int) {
	if len(values) == 0 {
		return
	}

	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width = 80
	}

	usableWidth := width - indent
	if usableWidth <= 0 {
		usableWidth = width
	}

	maxLen := 0
	for _, v := range values {
		l := len(v.Enum) + 2 + len(v.Description)

		if l > maxLen {
			maxLen = l
		}
	}

	cols := usableWidth / (maxLen + 2)
	if cols == 0 {
		cols = 1
	}

	rows := (len(values) + cols - 1) / cols

	sort.Slice(values, func(i, j int) bool {
		return values[i].Enum < values[j].Enum
	})

	for r := range rows {
		printIndent(indent)

		for c := range cols {
			i := c*rows + r

			if i < len(values) {
				s := fmt.Sprintf("%s: %s", values[i].Enum, values[i].Description)
				fmt.Printf("%-*s", maxLen+2, s)
			}
		}

		fmt.Println()
	}
}
