// Package strutil provides shared string utility functions used across
// the agent, bridge, and relay layers.
package strutil

import (
	"encoding/base64"
	"encoding/json"
)

// JSONString returns a JSON-quoted string literal for s (e.g. "hello" → "\"hello\"").
// Equivalent to json.Marshal(s) but returns the raw string without surrounding quotes
// stripped — the quotes ARE included in the output.
func JSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}

// JSONStringEscape returns a JSON-escaped string value without the surrounding quotes.
// Useful when building JSON fragments like `{"error":` + JSONStringEscape(msg) + `}`.
func JSONStringEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	// Strip surrounding quotes that json.Marshal adds.
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1])
	}
	return string(b)
}

// Base64Encode encodes data as standard base64 (with padding).
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}