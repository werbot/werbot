// Package access owns access bindings: who (a user or a team) may reach which
// resources, by explicit resource or by label selector.
package access

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
)

const maxSelectorLabels = 8

// The label rules are repeated from internal/inventory: domains never import each other.
var (
	labelKeyRe   = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{0,61}[a-z0-9])?$`)
	labelValueRe = regexp.MustCompile(`^[A-Za-z0-9._-]{0,63}$`)
)

// Binding grants a subject access to one resource or to every resource matching LabelSelector.
type Binding struct {
	ID, OrgID     uuid.UUID
	SubjectType   string // "user" | "team"
	SubjectID     uuid.UUID
	ResourceID    *uuid.UUID
	LabelSelector map[string]string
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
}

// CreateInput describes a new binding: exactly one of ResourceID and a non-empty LabelSelector.
type CreateInput struct {
	SubjectType   string
	SubjectID     uuid.UUID
	ResourceID    *uuid.UUID
	LabelSelector map[string]string
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", apperr.ErrInvalidArgument, fmt.Sprintf(format, a...))
}

func (in CreateInput) validate() error {
	if in.SubjectType != "user" && in.SubjectType != "team" {
		return invalid("subject type must be user or team")
	}
	if (in.ResourceID != nil) == (len(in.LabelSelector) > 0) {
		return invalid("set exactly one of resource and label selector")
	}
	if len(in.LabelSelector) > maxSelectorLabels {
		return invalid("at most %d selector labels", maxSelectorLabels)
	}
	for k, v := range in.LabelSelector {
		if !labelKeyRe.MatchString(k) {
			return invalid("label key %q must match %s", k, labelKeyRe)
		}
		if !labelValueRe.MatchString(v) {
			return invalid("label value of %q must match %s", k, labelValueRe)
		}
	}
	return nil
}

func toBinding(r dbq.AccessBinding) (Binding, error) {
	b := Binding{ID: r.ID, OrgID: r.OrgID, SubjectType: r.SubjectType, SubjectID: r.SubjectID,
		ResourceID: r.ResourceID, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	if r.LabelSelector != nil {
		if err := json.Unmarshal(r.LabelSelector, &b.LabelSelector); err != nil {
			return Binding{}, fmt.Errorf("access: decode selector of %s: %w", r.ID, err)
		}
	}
	return b, nil
}
