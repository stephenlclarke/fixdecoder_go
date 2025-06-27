// display.go
package decoder

import (
	"fmt"
	"sort"
)

// listAllMessages prints all messages in sorted order by MsgType.
func ListAllMessages(schema SchemaTree) {
	var msgs []MessageNode
	for _, m := range schema.Messages {
		msgs = append(msgs, m)
	}

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].MsgType < msgs[j].MsgType })
	for _, m := range msgs {
		fmt.Printf("%-4s: %s (%s)\n", m.MsgType, m.Name, m.MsgCat)
	}
}

// printMessageStart prints the “Message: Name (Type)” header.
func printMessageStart(msg MessageNode) {
	fmt.Printf("Message: %s (%s)\n", msg.Name, msg.MsgType)
}

// displayMessageStructureWithOptions orchestrates the above helpers.
func DisplayMessageStructureWithOptions(
	schema SchemaTree,
	msg MessageNode,
	verbose, includeHeader, includeTrailer, column bool,
	indent int,
) {
	printMessageStart(msg)
	printHeader(schema, msg, includeHeader, verbose, column, indent)
	printFields(msg, verbose, column, indent)
	printComponents(schema, msg, verbose, column, indent)
	printGroups(schema, msg, verbose, column, indent)
	printTrailer(schema, msg, includeTrailer, verbose, column, indent)
}
