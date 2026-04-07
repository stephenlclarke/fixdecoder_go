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

type Group struct {
	Name       string         `xml:"name,attr"`
	Required   string         `xml:"required,attr"`
	Fields     []FieldRef     `xml:"field"`
	Groups     []Group        `xml:"group"`
	Components []ComponentRef `xml:"component"`
}

type Component struct {
	Name       string         `xml:"name,attr"`
	Fields     []FieldRef     `xml:"field"`
	Groups     []Group        `xml:"group"`
	Components []ComponentRef `xml:"component"`
}

type ComponentRef struct {
	Name     string `xml:"name,attr"`
	Required string `xml:"required,attr"`
}

type Message struct {
	Name       string         `xml:"name,attr"`
	MsgType    string         `xml:"msgtype,attr"`
	MsgCat     string         `xml:"msgcat,attr"`
	Fields     []FieldRef     `xml:"field"`
	Groups     []Group        `xml:"group"`
	Components []ComponentRef `xml:"component"`
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
}

type GroupNode struct {
	Name       string
	Required   string
	Fields     []FieldNode
	Components []ComponentNode
	Groups     []GroupNode
}

type MessageNode struct {
	Name       string
	MsgType    string
	MsgCat     string
	Fields     []FieldNode
	Components []ComponentNode
	Groups     []GroupNode
}

type SchemaTree struct {
	Fields      map[string]Field
	Messages    map[string]MessageNode
	Components  map[string]ComponentNode
	Version     string
	ServicePack string
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

func buildComponentNode(comp Component, fieldMap map[string]Field, compMap map[string]Component) ComponentNode {
	node := ComponentNode{
		Name:   comp.Name,
		Fields: buildFieldNodes(comp.Fields, fieldMap),
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
