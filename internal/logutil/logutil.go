package logutil

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"cursorbridge/internal/appdir"
)

var (
	mu      sync.Mutex
	logger  *slog.Logger
	logFile *os.File
)

func init() {
	logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func Init(level string) error {
	mu.Lock()
	defer mu.Unlock()
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	dir, err := appdir.ConfigDir()
	if err != nil {
		return err
	}
	logPath := filepath.Join(dir, "cursorbridge.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	logFile = f
	w := io.MultiWriter(f, os.Stderr)
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})
	logger = slog.New(handler)
	slog.SetDefault(logger)
	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

func Logger() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return logger
}

func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

// GoproxyLogger adapts slog.Logger to goproxy's Logger interface.
type GoproxyLogger struct{}

func (GoproxyLogger) Printf(format string, args ...any) {
	Logger().Info("goproxy", "msg", fmt.Sprintf(format, args...))
}