//go:build debug

package debuglog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu      sync.Mutex
	logFile *os.File
)

func init() {
	// Write debug logs to a file next to the executable so they survive
	// Wails GUI mode (which has no console on Windows).
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	f, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	logFile = f
	log.SetOutput(f)
}

// Printf writes a formatted diagnostic log line to both the log file
// and os.Stderr (useful when running from a terminal).
func Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.WriteString(msg + "\n")
	}
	os.Stderr.WriteString(msg + "\n")
}