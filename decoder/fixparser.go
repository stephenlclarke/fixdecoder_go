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
