package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

type consoleHandler struct {
	w      io.Writer
	level  slog.Leveler
	color  bool
	attrs  []slog.Attr
	groups []string
	mu     *sync.Mutex
}

func newConsoleHandler(w io.Writer, level slog.Leveler, color bool) slog.Handler {
	return &consoleHandler{w: w, level: level, color: color, mu: &sync.Mutex{}}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.level != nil {
		minLevel = h.level.Level()
	}
	return level >= minLevel
}

func (h *consoleHandler) Handle(_ context.Context, record slog.Record) error {
	level := strings.ToUpper(record.Level.String())
	if h.color {
		level = colorLevel(record.Level, level)
	}
	fields := make([]string, 0, len(h.attrs)+record.NumAttrs())
	for _, attr := range h.attrs {
		fields = append(fields, formatAttr(attr))
	}
	record.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, formatAttr(attr))
		return true
	})

	line := fmt.Sprintf("%s %-5s %s", record.Time.Format(time.RFC3339), level, record.Message)
	if len(fields) > 0 {
		line += " " + strings.Join(fields, " ")
	}
	line += "\n"

	h.mu.Lock()
	_, err := io.WriteString(h.w, line)
	h.mu.Unlock()
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &next
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	next := *h
	next.groups = append(append([]string(nil), h.groups...), name)
	return &next
}

func formatAttr(attr slog.Attr) string {
	attr.Value = attr.Value.Resolve()
	return fmt.Sprintf("%s=%v", attr.Key, attr.Value.Any())
}

func colorLevel(level slog.Level, text string) string {
	switch {
	case level >= slog.LevelError:
		return "\x1b[31m" + text + "\x1b[0m"
	case level >= slog.LevelWarn:
		return "\x1b[33m" + text + "\x1b[0m"
	case level <= slog.LevelDebug:
		return "\x1b[36m" + text + "\x1b[0m"
	default:
		return "\x1b[32m" + text + "\x1b[0m"
	}
}

func logInfof(logger *slog.Logger, format string, args ...any) {
	logger.Info(fmt.Sprintf(format, args...))
}

func logWarnf(logger *slog.Logger, format string, args ...any) {
	logger.Warn(fmt.Sprintf(format, args...))
}

func logErrorf(logger *slog.Logger, format string, args ...any) {
	logger.Error(fmt.Sprintf(format, args...))
}
