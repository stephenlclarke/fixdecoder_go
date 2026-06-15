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

// schema.go
package decoder

import (
	"encoding/xml"
)

type FixDictionary struct {
	XMLName     xml.Name    `xml:"fix"`
	Major       string      `xml:"major,attr"`
	Minor       string      `xml:"minor,attr"`
	ServicePack string      `xml:"servicepack,attr"`
	Fields      []Field     `xml:"fields>field"`
	Messages    []Message   `xml:"messages>message"`
	Components  []Component `xml:"components>component"`
	Header      Component   `xml:"header"`
	Trailer     Component   `xml:"trailer"`
}

type Field struct {
	Name   string  `xml:"name,attr"`
	Number int     `xml:"number,attr"`
	Type   string  `xml:"type,attr"`
	Values []Value `xml:"value"`
}

type Value struct {
	Enum        string `xml:"enum,attr"`
	Description string `xml:"description,attr"`
}

type FieldRef struct {
	Name     string `xml:"name,attr"`
	Required string `xml:"required,attr"`
}

// ContainerEntryKind identifies the kind of dictionary entry preserved from XML order.
type ContainerEntryKind int

const (
	containerField ContainerEntryKind = iota
	containerGroup
	containerComponent
)

// ContainerEntry preserves the ordered field/component/group stream inside a FIX container.
type ContainerEntry struct {
	Kind      ContainerEntryKind
	Field     FieldRef
	Group     Group
	Component ComponentRef
}

type Group struct {
	Name       string           `xml:"name,attr"`
	Required   string           `xml:"required,attr"`
	Fields     []FieldRef       `xml:"field"`
	Groups     []Group          `xml:"group"`
	Components []ComponentRef   `xml:"component"`
	Entries    []ContainerEntry `xml:"-"`
}

type Component struct {
	Name       string           `xml:"name,attr"`
	Fields     []FieldRef       `xml:"field"`
	Groups     []Group          `xml:"group"`
	Components []ComponentRef   `xml:"component"`
	Entries    []ContainerEntry `xml:"-"`
}

type ComponentRef struct {
	Name     string `xml:"name,attr"`
	Required string `xml:"required,attr"`
}

type Message struct {
	Name       string           `xml:"name,attr"`
	MsgType    string           `xml:"msgtype,attr"`
	MsgCat     string           `xml:"msgcat,attr"`
	Fields     []FieldRef       `xml:"field"`
	Groups     []Group          `xml:"group"`
	Components []ComponentRef   `xml:"component"`
	Entries    []ContainerEntry `xml:"-"`
}

type FieldNode struct {
	Ref   FieldRef
	Field Field
}

type ComponentNode struct {
	Name       string
	Fields     []FieldNode
	Components []ComponentNode
	Groups     []GroupNode
	Entries    []ContainerNode
}

type GroupNode struct {
	Name       string
	Required   string
	Fields     []FieldNode
	Components []ComponentNode
	Groups     []GroupNode
	Entries    []ContainerNode
}

type MessageNode struct {
	Name       string
	MsgType    string
	MsgCat     string
	Fields     []FieldNode
	Components []ComponentNode
	Groups     []GroupNode
	Entries    []ContainerNode
}

// ContainerNode is the resolved, ordered display node used by schema renderers.
type ContainerNode struct {
	Kind      ContainerEntryKind
	Field     FieldNode
	Group     GroupNode
	Component ComponentNode
}

type SchemaTree struct {
	Fields      map[string]Field
	Messages    map[string]MessageNode
	Components  map[string]ComponentNode
	Version     string
	ServicePack string
}

// UnmarshalXML preserves the dictionary order of entries inside a component.
func (c *Component) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	c.Name = xmlAttr(start, "name")
	return decodeContainerEntries(dec, start, func(entry ContainerEntry) {
		c.Entries = append(c.Entries, entry)
		switch entry.Kind {
		case containerField:
			c.Fields = append(c.Fields, entry.Field)
		case containerGroup:
			c.Groups = append(c.Groups, entry.Group)
		case containerComponent:
			c.Components = append(c.Components, entry.Component)
		}
	})
}

// UnmarshalXML preserves the dictionary order of entries inside a repeating group.
func (g *Group) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	g.Name = xmlAttr(start, "name")
	g.Required = xmlAttr(start, "required")
	return decodeContainerEntries(dec, start, func(entry ContainerEntry) {
		g.Entries = append(g.Entries, entry)
		switch entry.Kind {
		case containerField:
			g.Fields = append(g.Fields, entry.Field)
		case containerGroup:
			g.Groups = append(g.Groups, entry.Group)
		case containerComponent:
			g.Components = append(g.Components, entry.Component)
		}
	})
}

// UnmarshalXML preserves the dictionary order of entries inside a message.
func (m *Message) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	m.Name = xmlAttr(start, "name")
	m.MsgType = xmlAttr(start, "msgtype")
	m.MsgCat = xmlAttr(start, "msgcat")
	return decodeContainerEntries(dec, start, func(entry ContainerEntry) {
		m.Entries = append(m.Entries, entry)
		switch entry.Kind {
		case containerField:
			m.Fields = append(m.Fields, entry.Field)
		case containerGroup:
			m.Groups = append(m.Groups, entry.Group)
		case containerComponent:
			m.Components = append(m.Components, entry.Component)
		}
	})
}

// decodeContainerEntries reads container children while retaining XML order.
func decodeContainerEntries(dec *xml.Decoder, start xml.StartElement, accept func(ContainerEntry)) error {
	for {
		token, err := dec.Token()
		if err != nil {
			return err
		}

		switch node := token.(type) {
		case xml.StartElement:
			switch node.Name.Local {
			case "field":
				var field FieldRef
				if err := dec.DecodeElement(&field, &node); err != nil {
					return err
				}
				accept(ContainerEntry{Kind: containerField, Field: field})
			case "group":
				var group Group
				if err := dec.DecodeElement(&group, &node); err != nil {
					return err
				}
				accept(ContainerEntry{Kind: containerGroup, Group: group})
			case "component":
				var component ComponentRef
				if err := dec.DecodeElement(&component, &node); err != nil {
					return err
				}
				accept(ContainerEntry{Kind: containerComponent, Component: component})
			default:
				if err := dec.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if node.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// xmlAttr returns an attribute value from a start element.
func xmlAttr(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}

	return ""
}

func BuildSchema(dict FixDictionary) SchemaTree {
	fieldMap := make(map[string]Field, len(dict.Fields))
	for _, f := range dict.Fields {
		fieldMap[f.Name] = f
	}

	compMap := make(map[string]Component, len(dict.Components))
	for _, c := range dict.Components {
		compMap[c.Name] = c
	}

	schema := SchemaTree{
		Fields:      fieldMap,
		Components:  make(map[string]ComponentNode),
		Messages:    make(map[string]MessageNode),
		Version:     dict.Major + "." + dict.Minor,
		ServicePack: dict.ServicePack,
	}

	if dict.ServicePack == "" {
		schema.ServicePack = "n/a"
	}

	for _, c := range dict.Components {
		schema.Components[c.Name] = buildComponentNode(c, fieldMap, compMap)
	}

	for _, m := range dict.Messages {
		schema.Messages[m.Name] = buildMessageNode(m, fieldMap, compMap)
	}

	// Include Header and Trailer as components
	header := dict.Header
	header.Name = "Header"
	schema.Components["Header"] = buildComponentNode(header, fieldMap, compMap)

	trailer := dict.Trailer
	trailer.Name = "Trailer"
	schema.Components["Trailer"] = buildComponentNode(trailer, fieldMap, compMap)

	return schema
}

func buildFieldNodes(refs []FieldRef, fieldMap map[string]Field) []FieldNode {
	nodes := make([]FieldNode, 0, len(refs))
	for _, ref := range refs {
		if f, ok := fieldMap[ref.Name]; ok {
			nodes = append(nodes, FieldNode{Ref: ref, Field: f})
		}
	}
	return nodes
}

// buildContainerNodes resolves ordered XML entries into display nodes.
func buildContainerNodes(entries []ContainerEntry, fieldMap map[string]Field, compMap map[string]Component) []ContainerNode {
	nodes := make([]ContainerNode, 0, len(entries))
	for _, entry := range entries {
		switch entry.Kind {
		case containerField:
			if field, ok := fieldMap[entry.Field.Name]; ok {
				nodes = append(nodes, ContainerNode{
					Kind:  containerField,
					Field: FieldNode{Ref: entry.Field, Field: field},
				})
			}
		case containerGroup:
			nodes = append(nodes, ContainerNode{
				Kind:  containerGroup,
				Group: buildGroupNode(entry.Group, fieldMap, compMap),
			})
		case containerComponent:
			if component, ok := compMap[entry.Component.Name]; ok {
				nodes = append(nodes, ContainerNode{
					Kind:      containerComponent,
					Component: buildComponentNode(component, fieldMap, compMap),
				})
			}
		}
	}

	return nodes
}

func buildComponentNode(comp Component, fieldMap map[string]Field, compMap map[string]Component) ComponentNode {
	node := ComponentNode{
		Name:    comp.Name,
		Fields:  buildFieldNodes(comp.Fields, fieldMap),
		Entries: buildContainerNodes(comp.Entries, fieldMap, compMap),
	}
	for _, cref := range comp.Components {
		if sub, ok := compMap[cref.Name]; ok {
			node.Components = append(node.Components, buildComponentNode(sub, fieldMap, compMap))
		}
	}
	for _, g := range comp.Groups {
		node.Groups = append(node.Groups, buildGroupNode(g, fieldMap, compMap))
	}
	return node
}

func buildGroupNode(group Group, fieldMap map[string]Field, compMap map[string]Component) GroupNode {
	node := GroupNode{
		Name:     group.Name,
		Required: group.Required,
		Fields:   buildFieldNodes(group.Fields, fieldMap),
		Entries:  buildContainerNodes(group.Entries, fieldMap, compMap),
	}
	for _, cref := range group.Components {
		if sub, ok := compMap[cref.Name]; ok {
			node.Components = append(node.Components, buildComponentNode(sub, fieldMap, compMap))
		}
	}
	for _, sg := range group.Groups {
		node.Groups = append(node.Groups, buildGroupNode(sg, fieldMap, compMap))
	}
	return node
}

func buildMessageNode(msg Message, fieldMap map[string]Field, compMap map[string]Component) MessageNode {
	mnode := MessageNode{
		Name:    msg.Name,
		MsgType: msg.MsgType,
		MsgCat:  msg.MsgCat,
		Fields:  buildFieldNodes(msg.Fields, fieldMap),
		Entries: buildContainerNodes(msg.Entries, fieldMap, compMap),
	}
	for _, cref := range msg.Components {
		if sub, ok := compMap[cref.Name]; ok {
			mnode.Components = append(mnode.Components, buildComponentNode(sub, fieldMap, compMap))
		}
	}
	for _, grp := range msg.Groups {
		mnode.Groups = append(mnode.Groups, buildGroupNode(grp, fieldMap, compMap))
	}
	return mnode
}
