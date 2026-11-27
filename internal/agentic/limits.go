package agentic

import (
	"cmp"
	"encoding/json"
	"fmt"
	"time"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Limits bound the runs of an agent and, per session, its database queries;
// a zero field means its default.
type Limits struct {
	MaxActions       int           // 0 or 1–10000
	MaxDuration      time.Duration // 0 or 1m–1h, whole seconds
	MaxConcurrent    int           // 0 or 1–32
	MaxRows          int           // 0 or 1–100000
	MaxResultBytes   int64         // 0 or 1 KiB–64 MiB
	StatementTimeout time.Duration // 0 or 1s–10m, whole seconds
	MaxQueries       int           // 0 or 1–100000
}

// limitsJSON is agent.limits: {"max_actions", "max_duration" (seconds),
// "max_concurrent", "max_rows", "max_result_bytes", "statement_timeout"
// (seconds), "max_queries"}.
type limitsJSON struct {
	MaxActions       int   `json:"max_actions"`
	MaxDuration      int64 `json:"max_duration"`
	MaxConcurrent    int   `json:"max_concurrent"`
	MaxRows          int   `json:"max_rows"`
	MaxResultBytes   int64 `json:"max_result_bytes"`
	StatementTimeout int64 `json:"statement_timeout"`
	MaxQueries       int   `json:"max_queries"`
}

// toLimits converts the stored seconds to durations.
func (j limitsJSON) toLimits() Limits {
	return Limits{MaxActions: j.MaxActions, MaxDuration: time.Duration(j.MaxDuration) * time.Second,
		MaxConcurrent: j.MaxConcurrent, MaxRows: j.MaxRows, MaxResultBytes: j.MaxResultBytes,
		StatementTimeout: time.Duration(j.StatementTimeout) * time.Second, MaxQueries: j.MaxQueries}
}

// toJSON is the inverse of toLimits; Validate has made every duration whole seconds.
func (l Limits) toJSON() limitsJSON {
	return limitsJSON{MaxActions: l.MaxActions, MaxDuration: int64(l.MaxDuration / time.Second),
		MaxConcurrent: l.MaxConcurrent, MaxRows: l.MaxRows, MaxResultBytes: l.MaxResultBytes,
		StatementTimeout: int64(l.StatementTimeout / time.Second), MaxQueries: l.MaxQueries}
}

// Validate checks the ranges; zero fields are valid (defaults).
func (l Limits) Validate() error {
	if l.MaxActions < 0 || l.MaxActions > 10000 {
		return fmt.Errorf("max_actions must be 0 to 10000: %w", apperr.ErrInvalidArgument)
	}
	if d := l.MaxDuration; d != 0 && (d < time.Minute || d > time.Hour || d%time.Second != 0) {
		return fmt.Errorf("max_duration must be 0 or whole seconds from 1m to 1h: %w", apperr.ErrInvalidArgument)
	}
	if l.MaxConcurrent < 0 || l.MaxConcurrent > 32 {
		return fmt.Errorf("max_concurrent must be 0 to 32: %w", apperr.ErrInvalidArgument)
	}
	return l.validateDB()
}

// validateDB checks the database limits; zero fields are valid (defaults).
func (l Limits) validateDB() error {
	for _, r := range []struct {
		name      string
		v, lo, hi int64
	}{
		{"max_rows", int64(l.MaxRows), 1, 100000},
		{"max_result_bytes", l.MaxResultBytes, 1 << 10, 64 << 20},
		{"max_queries", int64(l.MaxQueries), 1, 100000},
	} {
		if r.v != 0 && (r.v < r.lo || r.v > r.hi) {
			return fmt.Errorf("%s must be 0 or %d to %d: %w", r.name, r.lo, r.hi, apperr.ErrInvalidArgument)
		}
	}
	if d := l.StatementTimeout; d != 0 && (d < time.Second || d > 10*time.Minute || d%time.Second != 0) {
		return fmt.Errorf("statement_timeout must be 0 or whole seconds from 1s to 10m: %w", apperr.ErrInvalidArgument)
	}
	return nil
}

// WithDefaults returns l with every zero field set to its default.
func (l Limits) WithDefaults() Limits {
	if l.MaxActions == 0 {
		l.MaxActions = DefaultMaxActions
	}
	if l.MaxDuration == 0 {
		l.MaxDuration = DefaultMaxDuration
	}
	if l.MaxConcurrent == 0 {
		l.MaxConcurrent = DefaultMaxConcurrent
	}
	l.MaxRows = cmp.Or(l.MaxRows, DefaultMaxRows)
	l.MaxResultBytes = cmp.Or(l.MaxResultBytes, DefaultMaxResultBytes)
	l.StatementTimeout = cmp.Or(l.StatementTimeout, DefaultStatementTimeout)
	l.MaxQueries = cmp.Or(l.MaxQueries, DefaultMaxQueries)
	return l
}

// parseLimits reads agent.limits; absent keys are zero. Stored limits out of
// range fail closed with an internal error (not the caller's input), so
// callers may rely on Validate's bounds.
func parseLimits(raw []byte) (Limits, error) {
	var j limitsJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return Limits{}, err
	}
	l := j.toLimits()
	if err := l.Validate(); err != nil {
		return Limits{}, fmt.Errorf("stored limits out of range: %v", err)
	}
	return l, nil
}
