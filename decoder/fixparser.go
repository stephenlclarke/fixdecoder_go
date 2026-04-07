// fixparser.go
package decoder

import (
	"strconv"
	"strings"
)

type FieldValue struct {
	Tag   int
	Value string
}

func ParseFix(msg string) []FieldValue {
	// If there's no SOH delimiter, assume no valid fields
	if !strings.Contains(msg, "\x01") {
		return nil
	}

	parts := strings.Split(msg, "\x01")
	out := make([]FieldValue, 0, len(parts))

	for _, p := range parts {
		if p == "" {
			continue
		}

		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}

		tag, err := strconv.Atoi(kv[0])
		if err != nil {
			continue
		}

		out = append(out, FieldValue{Tag: tag, Value: kv[1]})
	}

	return out
}
