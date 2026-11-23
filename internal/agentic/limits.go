package agentic

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Limits bound the runs of an agent; a zero field means its default.
type Limits struct {
	MaxActions    int           // 0 or 1–10000
	MaxDuration   time.Duration // 0 or 1m–1h, whole seconds
	MaxConcurrent int           // 0 or 1–32
}

// limitsJSON is agent.limits: {"max_actions", "max_duration" (seconds), "max_concurrent"}.
type limitsJSON struct {
	MaxActions    int   `json:"max_actions"`
	MaxDuration   int64 `json:"max_duration"`
	MaxConcurrent int   `json:"max_concurrent"`
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
	l := Limits{MaxActions: j.MaxActions, MaxDuration: time.Duration(j.MaxDuration) * time.Second,
		MaxConcurrent: j.MaxConcurrent}
	if err := l.Validate(); err != nil {
		return Limits{}, fmt.Errorf("stored limits out of range: %v", err)
	}
	return l, nil
}
