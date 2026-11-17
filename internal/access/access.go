// Package access owns access bindings: who (a user or a team) may reach which
// resources, by explicit resource or by label selector.
package access

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/connector"
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
	Ports         []connector.PortRange // target ports of stream and datagram sessions; empty: every port
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
}

// CreateInput describes a new binding: exactly one of ResourceID and a non-empty LabelSelector.
type CreateInput struct {
	SubjectType   string
	SubjectID     uuid.UUID
	ResourceID    *uuid.UUID
	LabelSelector map[string]string
	Ports         []connector.PortRange // empty: every port
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
	if len(in.Ports) > 0 {
		if err := connector.ValidatePorts(in.Ports); err != nil {
			return invalid("%v", err)
		}
	}
	return nil
}

// portArrays splits ranges into the parallel port and end_port arrays of the queries.
func portArrays(rs []connector.PortRange) (ports, ends []int32) {
	ports, ends = make([]int32, len(rs)), make([]int32, len(rs))
	for i, r := range rs {
		ports[i], ends[i] = int32(r.Port), int32(r.EndPort)
	}
	return ports, ends
}

func toBinding(r dbq.ListBindingsRow) (Binding, error) {
	b := Binding{ID: r.ID, OrgID: r.OrgID, SubjectType: r.SubjectType, SubjectID: r.SubjectID,
		ResourceID: r.ResourceID, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt}
	if r.LabelSelector != nil {
		if err := json.Unmarshal(r.LabelSelector, &b.LabelSelector); err != nil {
			return Binding{}, fmt.Errorf("access: decode selector of %s: %w", r.ID, err)
		}
	}
	ports, err := portRanges(r.Ports, r.EndPorts)
	if err != nil {
		return Binding{}, fmt.Errorf("access: ports of %s: %w", r.ID, err)
	}
	b.Ports = ports
	return b, nil
}

// portRanges joins the parallel arrays the queries read back; an end equal
// to its start is the single port (EndPort 0).
func portRanges(starts, ends []int32) ([]connector.PortRange, error) {
	if len(starts) != len(ends) {
		return nil, fmt.Errorf("%d starts, %d ends", len(starts), len(ends))
	}
	var out []connector.PortRange
	for i, p := range starts {
		end := ends[i]
		if p < 1 || p > math.MaxUint16 || end < p || end > math.MaxUint16 {
			return nil, fmt.Errorf("invalid range %d-%d", p, end)
		}
		if end == p {
			end = 0
		}
		out = append(out, connector.PortRange{Port: uint16(p), EndPort: uint16(end)})
	}
	return out, nil
}
