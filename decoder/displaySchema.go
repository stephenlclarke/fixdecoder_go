// displaySchema.go
package decoder

import "fmt"

// PrintSchemaSummary writes a one-line overview of the dictionary that was
// just loaded.
func PrintSchemaSummary(schema SchemaTree) {
	fields := len(schema.Fields)
	components := len(schema.Components)
	messages := len(schema.Messages)
	version := schema.Version
	servicePack := schema.ServicePack

	fmt.Printf("Fields: %d   Components: %d   Messages: %d   Version: %s  Service Pack: %s\n",
		fields, components, messages, version, servicePack)
}
