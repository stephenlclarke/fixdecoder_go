package main

import (
	"testing"
)

// Helper for test fixtures
func makeTestMaps() (map[string]Field, map[string]Component) {
	fieldMap := map[string]Field{
		"F1": {Name: "F1", Number: 1, Type: "STRING"},
		"F2": {Name: "F2", Number: 2, Type: "INT"},
	}
	compMap := map[string]Component{
		"C1": {Name: "C1", Fields: []FieldRef{{Name: "F2", Required: "Y"}}},
	}
	return fieldMap, compMap
}

func TestBuildMessageNodeBasic(t *testing.T) {
	fieldMap, compMap := makeTestMaps()
	msg := Message{
		Name:       "Msg",
		MsgType:    "X",
		MsgCat:     "app",
		Fields:     []FieldRef{{Name: "F1", Required: "N"}},
		Components: []ComponentRef{{Name: "C1", Required: "N"}},
		Groups: []Group{
			{Name: "G1", Fields: []FieldRef{{Name: "F2", Required: "Y"}}},
		},
	}

	node := buildMessageNode(msg, fieldMap, compMap)

	// Message header fields
	if node.Name != "Msg" || node.MsgType != "X" || node.MsgCat != "app" {
		t.Errorf("Header mismatch: got %+v", node)
	}
	// Fields
	if len(node.Fields) != 1 || node.Fields[0].Field.Name != "F1" {
		t.Errorf("Fields mismatch: got %+v", node.Fields)
	}
	// Components
	if len(node.Components) != 1 || node.Components[0].Name != "C1" {
		t.Errorf("Components mismatch: got %+v", node.Components)
	}
	// Groups (covers last for-loop)
	if len(node.Groups) != 1 || node.Groups[0].Name != "G1" {
		t.Errorf("Groups mismatch: got %+v", node.Groups)
	}
}

func TestBuildMessageNodeUnknownComponent(t *testing.T) {
	fieldMap, compMap := makeTestMaps()
	msg := Message{
		Name:       "NoComp",
		MsgType:    "Z",
		Components: []ComponentRef{{Name: "NotThere", Required: "N"}},
	}
	node := buildMessageNode(msg, fieldMap, compMap)
	// Should skip unknown component
	if len(node.Components) != 0 {
		t.Errorf("Expected no components, got %+v", node.Components)
	}
}

func TestBuildMessageNodeNoGroups(t *testing.T) {
	fieldMap, compMap := makeTestMaps()
	msg := Message{
		Name:    "NoGroups",
		MsgType: "Y",
	}
	node := buildMessageNode(msg, fieldMap, compMap)
	if len(node.Groups) != 0 {
		t.Errorf("Expected no groups, got %+v", node.Groups)
	}
}

func TestBuildMessageNodeEmpty(t *testing.T) {
	fieldMap := map[string]Field{}
	compMap := map[string]Component{}
	msg := Message{}
	node := buildMessageNode(msg, fieldMap, compMap)
	// Expect empty slices, not nil
	if node.Name != "" || node.MsgType != "" || node.MsgCat != "" {
		t.Errorf("Expected empty strings for Name, MsgType, MsgCat, got: %+v", node)
	}
	if len(node.Fields) != 0 {
		t.Errorf("Expected zero Fields, got: %+v", node.Fields)
	}
	if len(node.Components) != 0 {
		t.Errorf("Expected zero Components, got: %+v", node.Components)
	}
	if len(node.Groups) != 0 {
		t.Errorf("Expected zero Groups, got: %+v", node.Groups)
	}
}

// This one checks nested groups for full coverage of the group for-loop.
func TestBuildMessageNodeNestedGroups(t *testing.T) {
	fieldMap, compMap := makeTestMaps()
	msg := Message{
		Name:    "NestGroup",
		MsgType: "N",
		Groups: []Group{
			{Name: "Outer", Groups: []Group{
				{Name: "Inner", Fields: []FieldRef{{Name: "F1", Required: "N"}}},
			}},
		},
	}
	node := buildMessageNode(msg, fieldMap, compMap)
	if len(node.Groups) != 1 || node.Groups[0].Name != "Outer" {
		t.Errorf("Expected Outer group, got %+v", node.Groups)
	}
	if len(node.Groups[0].Groups) != 1 || node.Groups[0].Groups[0].Name != "Inner" {
		t.Errorf("Expected Inner group, got %+v", node.Groups[0].Groups)
	}
}
