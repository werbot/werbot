// Package logging builds the process slog logger.
package logging

import (
	"io"
	"log/slog"
)

// New returns a logger writing to w. level and format are already validated by config.Load.
func New(w io.Writer, level, format string) *slog.Logger {
	var lv slog.Level
	_ = lv.UnmarshalText([]byte(level))
	opts := &slog.HandlerOptions{Level: lv}
	if format == "text" {
		return slog.New(slog.NewTextHandler(w, opts))
	}
	return slog.New(slog.NewJSONHandler(w, opts))
}
