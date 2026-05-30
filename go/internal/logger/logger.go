package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
)

var l *slog.Logger

func init() {
	l = slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

// Init sets up the package-level logger with a JSON handler writing to w at
// the given level. Call once from main before any logging.
func Init(level slog.Level, w io.Writer) {
	l = slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}

// Get returns the package-level logger.
func Get() *slog.Logger {
	return l
}

// Info logs at INFO level.
func Info(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// Warn logs at WARN level.
func Warn(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelWarn, msg, attrs...)
}

// Error logs at ERROR level.
func Error(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}
