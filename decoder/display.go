// display.go
package decoder

import (
	"fmt"
	"os"
	"sort"

	"strings"

	"golang.org/x/term"
)

// findField returns the Field with the given number, or false if not found.
func FindField(schema SchemaTree, tagID int) (Field, bool) {
	for _, f := range schema.Fields {
		if f.Number == tagID {
			return f, true
		}
	}
	return Field{}, false
}

func printField(field FieldNode, indent int) {
	printIndent(indent)
	fmt.Printf("%4d: %s (%s)%s\n",
		field.Field.Number, field.Field.Name, field.Field.Type, formatRequired(field.Ref.Required),
	)
}

// printStringColumns prints a slice of strings in columns based on terminal width.
func PrintStringColumns(items []string) {
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
