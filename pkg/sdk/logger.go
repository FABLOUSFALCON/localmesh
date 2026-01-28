// Package sdk provides logger implementation.
package sdk

import (
	"log/slog"
)

// slogLogger wraps slog.Logger to implement sdk.Logger
type slogLogger struct {
	logger *slog.Logger
}

// NewLogger creates a new Logger from an slog.Logger
func NewLogger(l *slog.Logger) Logger {
	if l == nil {
		l = slog.Default()
	}
	return &slogLogger{logger: l}
}

func (l *slogLogger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

func (l *slogLogger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

func (l *slogLogger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

func (l *slogLogger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{logger: l.logger.With(args...)}
}
