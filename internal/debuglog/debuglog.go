//go:build !debug

package debuglog

// Printf writes a formatted diagnostic log line. In release builds this is a
// no-op; the compiler eliminates the call and its arguments entirely.
func Printf(format string, args ...interface{}) {}
