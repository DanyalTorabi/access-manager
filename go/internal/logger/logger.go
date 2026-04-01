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

// Audit emits a structured audit event at INFO level with audit=true.
// Use for security-relevant mutations (grants, revokes, entity changes).
func Audit(ctx context.Context, action string, attrs ...slog.Attr) {
	all := make([]slog.Attr, 0, len(attrs)+2)
	all = append(all, slog.Bool("audit", true), slog.String("action", action))
	all = append(all, attrs...)
	l.LogAttrs(ctx, slog.LevelInfo, "audit", all...)
}
