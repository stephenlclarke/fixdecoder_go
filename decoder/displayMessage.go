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
	printIndent(indent)
	fmt.Println("Message: Body")
	renderContainerEntries(schema, msg, messageEntries(msg), verbose, column, indent+nestIndent)
	printTrailer(schema, msg, includeTrailer, verbose, column, indent)
}

// messageEntries returns ordered entries, falling back to legacy bucket fields for tests.
func messageEntries(msg MessageNode) []ContainerNode {
	if len(msg.Entries) > 0 {
		return msg.Entries
	}

	entries := make([]ContainerNode, 0, len(msg.Fields)+len(msg.Components)+len(msg.Groups))
	for _, field := range msg.Fields {
		entries = append(entries, ContainerNode{Kind: containerField, Field: field})
	}
	for _, component := range msg.Components {
		entries = append(entries, ContainerNode{Kind: containerComponent, Component: component})
	}
	for _, group := range msg.Groups {
		entries = append(entries, ContainerNode{Kind: containerGroup, Group: group})
	}

	return entries
}
