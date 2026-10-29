// Package logging builds flint's diagnostic logger. Logs are off by default.
// With --verbose, flint writes them to stderr, so they never corrupt the token
// on stdout, which the caller consumes with `eval`.
//
// Secret safety: never log token values, credential secrets, or session
// tokens. Log metadata only, such as the profile name, the region, the expiry,
// and the output mode.
package logging

import (
	"io"
	"log/slog"
	"strings"
)

// Format selects the log output encoding.
type Format string

// Supported log output encodings.
const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// ParseFormat validates a --log-format value.
func ParseFormat(s string) (Format, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "text":
		return FormatText, true
	case "json":
		return FormatJSON, true
	default:
		return "", false
	}
}

// New returns a logger writing to w. When verbose is false it returns a logger
// that discards everything (level set above any emitted record), so callers can
// log unconditionally without guarding on the flag.
func New(w io.Writer, verbose bool, format Format) *slog.Logger {
	if !verbose {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	opts := &slog.HandlerOptions{Level: slog.LevelDebug}

	var h slog.Handler
	switch format {
	case FormatJSON:
		h = slog.NewJSONHandler(w, opts)
	default:
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

// Discard returns a logger that drops all records. Useful as a default in
// constructors and tests.
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
