package policy

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/policy/dbq"
)

// CommandResult is the classes and paths of a command and the decision on it:
// the exec decision, or the connect decision when connect denies. Paths are
// the shellclass.PathRef texts in analysis order; PathHits and PathOutside
// the ids of the path rules of the whole organization it hits or leaves.
type CommandResult struct {
	Classes                      []string
	Paths, PathHits, PathOutside []string
	Decision
}

// maxCwdBytes bounds the cwd of TestCommand.
const maxCwdBytes = 1024

// TestCommand analyzes command from cwd ("" is "~") with the class rules of
// orgID and the host of the resource and decides it for userID from ip at the
// time at (zero: now): connect first, then the exec action of the resource type.
func (s *Service) TestCommand(ctx context.Context, orgID, userID, resourceID uuid.UUID, command, cwd string, ip netip.Addr, at time.Time) (CommandResult, error) {
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
	a, err := analyze(c.snap, command, c.res.Host, cwd)
	if err != nil {
		return CommandResult{}, err
	}
	out := result(a, c.snap.PathEvalRules())
	out.Decision = c.ps.Authorize(c.req)
	if !out.Allow {
		return out, nil
	}
	c.req.Action, c.req.Command, c.req.Classes = exec, command, out.Classes
	c.req.Paths, c.req.PathHits, c.req.PathOutside = out.Paths, out.PathHits, out.PathOutside
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

// analyze analyzes command from cwd ("" is "~") as the gateway does with the
// class rules of snap; a cwd other than "~", "~/…" or a clean absolute path of
// at most maxCwdBytes without control characters is invalid.
func analyze(snap Snapshot, command, host, cwd string) (shellclass.Analysis, error) {
	cwd = cmp.Or(cwd, "~")
	if !validCwd(cwd) {
		return shellclass.Analysis{}, invalid("invalid cwd")
	}
	overrides := make([]shellclass.Override, len(snap.ClassRules))
	for i, r := range snap.ClassRules {
		overrides[i] = shellclass.Override{Pattern: r.Pattern, Classes: classesOf(r.Classes)}
	}
	c, err := shellclass.New(overrides)
	if err != nil {
		return shellclass.Analysis{}, fmt.Errorf("policy: class rules: %w", err)
	}
	return c.Analyze(command, host, cwd), nil
}

func validCwd(cwd string) bool {
	return len(cwd) <= maxCwdBytes && utf8.ValidString(cwd) && path.Clean(cwd) == cwd &&
		(cwd == "~" || strings.HasPrefix(cwd, "~/") || strings.HasPrefix(cwd, "/")) &&
		strings.IndexFunc(cwd, unicode.IsControl) < 0
}

// result is the classes and paths of a and the path rules it hits or leaves.
func result(a shellclass.Analysis, rules []shellclass.PathRule) CommandResult {
	var out CommandResult
	for _, cl := range a.Classes {
		out.Classes = append(out.Classes, string(cl))
	}
	for _, p := range a.Paths {
		out.Paths = append(out.Paths, p.String())
	}
	out.PathHits, out.PathOutside = shellclass.EvalPaths(rules, a.Paths, a.Unresolved)
	return out
}

func classesOf(names []string) []shellclass.Class {
	out := make([]shellclass.Class, len(names))
	for i, n := range names {
		out[i] = shellclass.Class(n)
	}
	return out
}
