package logger

import (
	"io"
	"log/slog"
	"os"
)

var L *slog.Logger

func init() {
	// Default logger to standard output until explicitly initialized
	L = slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

// Setup initializes the global logger with a specific level and writer.
func Setup(w io.Writer, level slog.Level) {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	L = slog.New(slog.NewJSONHandler(w, opts))
	slog.SetDefault(L)
}

// Info logs a message at Info level.
func Info(msg string, args ...any) {
	L.Info(msg, args...)
}

// Error logs a message at Error level.
func Error(msg string, args ...any) {
	L.Error(msg, args...)
}

// Debug logs a message at Debug level.
func Debug(msg string, args ...any) {
	L.Debug(msg, args...)
}

// Warn logs a message at Warn level.
func Warn(msg string, args ...any) {
	L.Warn(msg, args...)
}
