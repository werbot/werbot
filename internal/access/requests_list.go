package access

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/access/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Request scopes of ListRequests.
const (
	ScopeMine      = "mine"
	ScopeToApprove = "to_approve"
	ScopeAll       = "all"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
	maxPage         = 10_000_000 // as session.ListSessions; (maxPage-1)*maxPageSize fits int32
)

// RequestView is an approval with its requester, resource and whether the caller may decide it.
type RequestView struct {
	Approval
	RequesterLogin, ResourceName string
	CanDecide                    bool
}

// RequestFilter narrows ListRequests. Page 0 is 1; PageSize 0 is 50, above 200 is 200.
type RequestFilter struct {
	Scope          string // ScopeMine | ScopeToApprove | ScopeAll
	Status         string // "" or an approval status
	Page, PageSize int
}

// views converts rows and keeps those keep accepts. Only a pending request before its deadline can be decided.
func views(e evaluator, rows []dbq.ListApprovalsRow, user uuid.UUID, keep func(RequestView) bool) ([]RequestView, error) {
	var out []RequestView
	now := time.Now()
	for _, r := range rows {
		a, err := toApproval(r.Approval)
		if err != nil {
			return nil, err
		}
		v := RequestView{Approval: a, RequesterLogin: r.RequesterLogin, ResourceName: r.ResourceName,
			CanDecide: a.Status == StatusPending && a.ExpiresAt.After(now) && e.canDecide(a, user)}
		if keep(v) {
			out = append(out, v)
		}
	}
	return out, nil
}

// GetRequest returns a request the requester, a possible approver, an owner or an admin sees; to others it is not found.
// A run token (p.RunID set) sees only the requests of its run.
func (s *Service) GetRequest(ctx context.Context, p principal.Principal, id uuid.UUID) (RequestView, error) {
	var out []RequestView
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		e, err := loadEvaluator(ctx, st, p.OrgID)
		if err != nil {
			return err
		}
		rows, err := st.ListApprovals(ctx, dbq.ListApprovalsParams{OrgID: p.OrgID, ID: &id, AgentRunID: p.RunID, RowLimit: new(int32(1))})
		if err != nil {
			return err
		}
		out, err = views(e, rows, p.UserID, func(v RequestView) bool { return e.visible(v.Approval, p.UserID) })
		return err
	})
	if err != nil {
		return RequestView{}, err
	}
	if len(out) == 0 {
		return RequestView{}, fmt.Errorf("request not found: %w", apperr.ErrNotFound)
	}
	return out[0], nil
}

// ListRequests returns one page of requests, newest first, and the next page
// number (0 after the last): ScopeMine those of p, ScopeToApprove the pending
// ones p may decide, ScopeAll every request (owner and admin only). A run
// token (p.RunID set) lists only the requests of its run, in every scope.
func (s *Service) ListRequests(ctx context.Context, p principal.Principal, f RequestFilter) ([]RequestView, int, error) {
	statuses := []string{"", StatusPending, StatusApproved, StatusDenied, StatusExpired, StatusCanceled}
	if f.Page < 0 || f.Page > maxPage || f.PageSize < 0 || !slices.Contains(statuses, f.Status) {
		return nil, 0, invalid("invalid request filter")
	}
	page, size := max(f.Page, 1), min(f.PageSize, maxPageSize)
	if size == 0 {
		size = defaultPageSize
	}
	arg := dbq.ListApprovalsParams{OrgID: p.OrgID, AgentRunID: p.RunID, Status: f.Status,
		RowOffset: int32((page - 1) * size), RowLimit: new(int32(size + 1))} //nolint:gosec // page <= maxPage, size <= maxPageSize
	switch f.Scope {
	case ScopeMine:
		arg.RequesterUserID = &p.UserID
	case ScopeAll:
		if !isManager(p.Role) {
			return nil, 0, fmt.Errorf("only owner and admin list all requests: %w", apperr.ErrPermissionDenied)
		}
	case ScopeToApprove:
		// ponytail: filtered and paged in memory; pending rows live at most request_pending_ttl (≤ 24h)
		arg.Status, arg.RowOffset, arg.RowLimit = StatusPending, 0, nil
	default:
		return nil, 0, invalid("scope must be %s, %s or %s", ScopeMine, ScopeToApprove, ScopeAll)
	}
	var out []RequestView
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		e, err := loadEvaluator(ctx, st, p.OrgID)
		if err != nil {
			return err
		}
		rows, err := st.ListApprovals(ctx, arg)
		if err != nil {
			return err
		}
		out, err = views(e, rows, p.UserID, func(v RequestView) bool { return f.Scope != ScopeToApprove || v.CanDecide })
		return err
	})
	if err != nil {
		return nil, 0, err
	}
	if f.Scope == ScopeToApprove {
		out = out[min((page-1)*size, len(out)):]
	}
	next := 0
	if len(out) > size { // the extra row proves a next page
		out, next = out[:size], page+1
	}
	return out, next, nil
}

// Requestable reports for each of resourceIDs whether a policy lets p request
// access to it; an agent token (p.RunID set) only through policies of all agents.
func (s *Service) Requestable(ctx context.Context, p principal.Principal, resourceIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(resourceIDs))
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		e, err := loadEvaluator(ctx, st, p.OrgID)
		if err != nil {
			return err
		}
		for _, id := range resourceIDs {
			_, ok := e.terms(Approval{RequesterID: p.UserID, AgentRunID: p.RunID, Kind: KindAccess, ResourceID: &id})
			out[id] = ok
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
