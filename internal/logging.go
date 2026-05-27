package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

const defaultConsoleTimeFormat = "15:04:05"

const (
	ansiReset   = "\x1b[0m"
	ansiDim     = "\x1b[2m"
	ansiRed     = "\x1b[31m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiBlue    = "\x1b[34m"
	ansiMagenta = "\x1b[35m"
	ansiCyan    = "\x1b[36m"
	ansiGray    = "\x1b[90m"
)

var attrKeyColors = map[string]string{
	"stage":       ansiMagenta,
	"output":      ansiYellow,
	"case":        ansiBlue,
	"host":        ansiGreen,
	"run_id":      ansiGray,
	"run_dir":     ansiGray,
	"local_path":  ansiCyan,
	"remote_path": ansiCyan,
	"error":       ansiRed,
	"exit_code":   ansiGray,
	"commit":      ansiBlue,
	"branch":      ansiBlue,
	"dirty":       ansiYellow,
	"pid":         ansiGray,
	"type":        ansiCyan,
	"target":      ansiCyan,
	"index":       ansiGray,
	"total":       ansiGray,
}

type consoleHandler struct {
	w          io.Writer
	level      slog.Leveler
	color      bool
	timeFormat string
	attrs      []slog.Attr
	mu         *sync.Mutex
}

func newConsoleHandler(w io.Writer, level slog.Leveler, color bool, timeFormat string) slog.Handler {
	if strings.TrimSpace(timeFormat) == "" {
		timeFormat = defaultConsoleTimeFormat
	}
	return &consoleHandler{
		w:          w,
		level:      level,
		color:      color,
		timeFormat: timeFormat,
		mu:         &sync.Mutex{},
	}
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
		fields = append(fields, formatAttr(attr, h.color))
	}
	record.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, formatAttr(attr, h.color))
		return true
	})

	timestamp := record.Time.Format(h.timeFormat)
	if h.color {
		timestamp = ansiDim + timestamp + ansiReset
	}

	line := fmt.Sprintf("%s %-5s %s", timestamp, level, record.Message)
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

func (h *consoleHandler) WithGroup(string) slog.Handler {
	next := *h
	return &next
}

func formatAttr(attr slog.Attr, color bool) string {
	attr.Value = attr.Value.Resolve()
	key := attr.Key
	value := fmt.Sprintf("%v", attr.Value.Any())
	if !color {
		return fmt.Sprintf("%s=%s", key, value)
	}
	keyColor, ok := attrKeyColors[key]
	if !ok {
		return fmt.Sprintf("%s=%s", key, value)
	}
	return fmt.Sprintf("%s%s%s%s=%s", keyColor, key, ansiReset, keyColor, value+ansiReset)
}

func colorLevel(level slog.Level, text string) string {
	switch {
	case level >= slog.LevelError:
		return ansiRed + text + ansiReset
	case level >= slog.LevelWarn:
		return ansiYellow + text + ansiReset
	case level <= slog.LevelDebug:
		return ansiCyan + text + ansiReset
	default:
		return ansiGreen + text + ansiReset
	}
}

func logError(logger *slog.Logger, msg string, err error, attrs ...any) {
	args := append(append([]any(nil), attrs...), "error", err)
	logger.Error(msg, args...)
}

// ConfigureDefaultLogger installs a human-readable slog logger on stderr.
func ConfigureDefaultLogger() {
	color := term.IsTerminal(int(os.Stderr.Fd()))
	slog.SetDefault(slog.New(newConsoleHandler(os.Stderr, slog.LevelInfo, color, defaultConsoleTimeFormat)))
}
