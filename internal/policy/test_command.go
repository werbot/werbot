package policy

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// CommandResult is the classes of a command and the decision on it: the exec
// decision, or the connect decision when connect denies.
type CommandResult struct {
	Classes []string
	Decision
}

// TestCommand classifies command with the class rules of orgID and the host of
// the resource and decides it for userID from ip at the time at (zero: now):
// connect first, then the exec action of the resource type.
func (s *Service) TestCommand(ctx context.Context, orgID, userID, resourceID uuid.UUID, command string, ip netip.Addr, at time.Time) (CommandResult, error) {
	if err := s.requireMember(ctx, orgID, userID); err != nil {
		return CommandResult{}, err
	}
	c, err := s.prepare(ctx, orgID, userID, resourceID, ip, at)
	if err != nil {
		return CommandResult{}, err
	}
	exec, ok := c.typ.ExecAction()
	if !ok {
		return CommandResult{}, fmt.Errorf("resource type has no exec action: %w", apperr.ErrFailedPrecondition)
	}
	classes, err := classify(c.snap.ClassRules, command, c.res.Host)
	if err != nil {
		return CommandResult{}, err
	}
	out := CommandResult{Classes: classes, Decision: c.ps.Authorize(c.req)}
	if !out.Allow {
		return out, nil
	}
	c.req.Action, c.req.Command, c.req.Classes = exec, command, classes
	out.Decision = c.ps.Authorize(c.req)
	return out, nil
}

// requireMember returns ErrNotFound unless userID is a member of orgID.
func (s *Service) requireMember(ctx context.Context, orgID, userID uuid.UUID) error {
	var member bool
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) (err error) {
		member, err = newStore(tx).IsMember(ctx, dbq.IsMemberParams{OrgID: orgID, UserID: userID})
		return err
	})
	if err == nil && !member {
		return fmt.Errorf("user not found: %w", apperr.ErrNotFound)
	}
	return err
}

// classify classifies command as the gateway does with the class rules.
func classify(rules []SnapshotClassRule, command, host string) ([]string, error) {
	overrides := make([]shellclass.Override, len(rules))
	for i, r := range rules {
		overrides[i] = shellclass.Override{Pattern: r.Pattern, Classes: classesOf(r.Classes)}
	}
	c, err := shellclass.New(overrides)
	if err != nil {
		return nil, fmt.Errorf("policy: class rules: %w", err)
	}
	classes := c.Classify(command, host)
	out := make([]string, len(classes))
	for i, cl := range classes {
		out[i] = string(cl)
	}
	return out, nil
}

func classesOf(names []string) []shellclass.Class {
	out := make([]shellclass.Class, len(names))
	for i, n := range names {
		out[i] = shellclass.Class(n)
	}
	return out
}
