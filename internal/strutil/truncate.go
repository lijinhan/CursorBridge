// Package strutil provides shared string utility functions used across
// the agent, bridge, and relay layers.
package strutil

// Truncate shortens s to at most max runes, appending "..." if truncated.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

// TruncateErr returns a truncated string representation of err for logging.
// If err is nil, returns "<nil>".
func TruncateErr(err error, max int) string {
	if err == nil {
		return "<nil>"
	}
	return Truncate(err.Error(), max)
}