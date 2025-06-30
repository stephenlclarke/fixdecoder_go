package decoder

import (
	"testing"
)

func TestBuildSchemaEmptyDictionary(t *testing.T) {
	d := FixDictionary{}
	tree := BuildSchema(d)

	if len(tree.Fields) != 0 {
		t.Errorf("Expected 0 Fields, got %d", len(tree.Fields))
	}
	if len(tree.Messages) != 0 {
		t.Errorf("Expected 0 Messages, got %d", len(tree.Messages))
	}

	// Header and Trailer always exist
	if len(tree.Components) != 2 {
		t.Errorf("Expected 2 Components (Header/Trailer), got %d", len(tree.Components))
	}
	if _, ok := tree.Components["Header"]; !ok {
		t.Error("Missing 'Header' component")
	}
	if _, ok := tree.Components["Trailer"]; !ok {
		t.Error("Missing 'Trailer' component")
	}
}

func TestBuildSchemaWithFields(t *testing.T) {
	d := FixDictionary{
		Fields: []Field{
			{Name: "ClOrdID", Number: 11, Type: "STRING"},
			{Name: "HandlInst", Number: 21, Type: "CHAR"},
		},
	}
	tree := BuildSchema(d)

	if len(tree.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(tree.Fields))
	}
	if tree.Fields["ClOrdID"].Number != 11 {
		t.Error("Incorrect field mapping for ClOrdID")
	}
}

func TestBuildSchemaWithComponents(t *testing.T) {
	d := FixDictionary{
		Fields: []Field{{Name: "Account", Number: 1, Type: "STRING"}},
		Components: []Component{
			{
				Name: "Parties",
				Fields: []FieldRef{
					{Name: "Account", Required: "Y"},
				},
			},
		},
	}
	tree := BuildSchema(d)

	c, ok := tree.Components["Parties"]
	if !ok {
		t.Fatal("Expected component Parties")
	}
	if len(c.Fields) != 1 || c.Fields[0].Field.Name != "Account" {
		t.Error("Incorrect component field")
	}
}

func TestBuildSchemaWithNestedGroups(t *testing.T) {
	d := FixDictionary{
		Fields: []Field{{Name: "PartyID", Number: 448, Type: "STRING"}},
		Components: []Component{
			{
				Name: "Parties",
				Groups: []Group{
					{
						Name: "NoPartyIDs",
						Fields: []FieldRef{
							{Name: "PartyID", Required: "Y"},
						},
					},
				},
			},
		},
	}
	tree := BuildSchema(d)
	comp := tree.Components["Parties"]
	if len(comp.Groups) != 1 {
		t.Errorf("Expected 1 group in Parties")
	}
	if comp.Groups[0].Name != "NoPartyIDs" {
		t.Errorf("Group name mismatch")
	}
}

func TestBuildSchemaWithMessage(t *testing.T) {
	d := FixDictionary{
		Fields: []Field{{Name: "ClOrdID", Number: 11, Type: "STRING"}},
		Messages: []Message{
			{
				Name:    "NewOrderSingle",
				MsgType: "D",
				Fields: []FieldRef{
					{Name: "ClOrdID", Required: "Y"},
				},
			},
		},
	}
	tree := BuildSchema(d)
	msg, ok := tree.Messages["NewOrderSingle"]
	if !ok {
		t.Fatal("Expected message NewOrderSingle")
	}
	if msg.MsgType != "D" {
		t.Error("Incorrect MsgType for NewOrderSingle")
	}
}

func TestNestedComponentsAndGroups(t *testing.T) {
	d := FixDictionary{
		Fields: []Field{
			{Name: "Account", Number: 1, Type: "STRING"},
			{Name: "ClOrdID", Number: 11, Type: "STRING"},
		},
		Components: []Component{
			{
				Name:   "SubComponent",
				Fields: []FieldRef{{Name: "Account"}},
			},
			{
				Name: "MainComponent",
				Components: []ComponentRef{
					{Name: "SubComponent"},
				},
				Groups: []Group{
					{
						Name:   "NoLegs",
						Fields: []FieldRef{{Name: "ClOrdID"}},
					},
				},
			},
		},
		Messages: []Message{
			{
				Name: "TestMessage",
				Components: []ComponentRef{
					{Name: "MainComponent"},
				},
				Groups: []Group{
					{
						Name:   "NoAllocs",
						Fields: []FieldRef{{Name: "Account"}},
					},
				},
			},
		},
	}

	tree := BuildSchema(d)

	m := tree.Messages["TestMessage"]
	if len(m.Components) != 1 || m.Components[0].Name != "MainComponent" {
		t.Error("Expected MainComponent in message")
	}
	if len(m.Components[0].Components) != 1 || m.Components[0].Components[0].Name != "SubComponent" {
		t.Error("Expected nested SubComponent in MainComponent")
	}
	if len(m.Components[0].Groups) != 1 || m.Components[0].Groups[0].Name != "NoLegs" {
		t.Error("Expected group NoLegs inside MainComponent")
	}
	if len(m.Groups) != 1 || m.Groups[0].Name != "NoAllocs" {
		t.Error("Expected top-level group NoAllocs in message")
	}
}

func TestBuildGroupNodeCoversNestedComponentsAndGroups(t *testing.T) {
	fieldMap := map[string]Field{
		"Account": {Name: "Account", Number: 1, Type: "STRING"},
		"PartyID": {Name: "PartyID", Number: 448, Type: "STRING"},
	}

	compMap := map[string]Component{
		"PartyComponent": {
			Name:   "PartyComponent",
			Fields: []FieldRef{{Name: "PartyID"}},
		},
	}

	group := Group{
		Name:     "NoPartyIDs",
		Required: "Y",
		Fields: []FieldRef{
			{Name: "Account"},
		},
		Components: []ComponentRef{
			{Name: "PartyComponent"},
		},
		Groups: []Group{
			{
				Name: "NoNestedParties",
				Fields: []FieldRef{
					{Name: "PartyID"},
				},
			},
		},
	}

	node := buildGroupNode(group, fieldMap, compMap)

	if len(node.Components) != 1 || node.Components[0].Name != "PartyComponent" {
		t.Errorf("Expected nested component 'PartyComponent' to be included")
	}
	if len(node.Groups) != 1 || node.Groups[0].Name != "NoNestedParties" {
		t.Errorf("Expected nested group 'NoNestedParties' to be included")
	}
}
