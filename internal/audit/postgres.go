package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/audit/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

type postgres struct{ pool *pgxpool.Pool }

// NewPostgres returns the Store over pool; each call runs in dbx.InOrg for the org and the caller in ctx.
func NewPostgres(pool *pgxpool.Pool) Store { return postgres{pool: pool} }

func (s postgres) query(ctx context.Context, orgID uuid.UUID, arg dbq.ListAuditEventsParams) (rows []dbq.ListAuditEventsRow, err error) {
	p, _ := principal.From(ctx) // uuid.Nil without a caller
	err = dbx.InOrg(ctx, s.pool, orgID, p.UserID, func(tx pgx.Tx) (err error) {
		rows, err = dbq.New(tx).ListAuditEvents(ctx, arg)
		return err
	})
	return rows, err
}

func (s postgres) ListEvents(ctx context.Context, orgID uuid.UUID, f Filter) ([]Event, error) {
	rows, err := s.query(ctx, orgID, dbq.ListAuditEventsParams{
		OrgID: &orgID, Action: f.Action, ActorUserID: f.ActorUserID, TargetType: f.TargetType, TargetID: f.TargetID,
		FromTs: f.From, ToTs: f.To,
		RowOffset: int32((f.Page - 1) * f.PageSize), RowLimit: int32(f.PageSize + 1), //nolint:gosec // bounded by the service
	})
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		if out[i], err = event(r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s postgres) GetEvent(ctx context.Context, orgID, id uuid.UUID) (Event, error) {
	rows, err := s.query(ctx, orgID, dbq.ListAuditEventsParams{OrgID: &orgID, ID: &id, RowLimit: 1})
	if err != nil {
		return Event{}, err
	}
	if len(rows) == 0 {
		return Event{}, fmt.Errorf("audit event not found: %w", apperr.ErrNotFound)
	}
	return event(rows[0])
}

func event(r dbq.ListAuditEventsRow) (Event, error) {
	e := Event{ID: r.ID, ActorUserID: r.ActorUserID, ActorLogin: r.ActorLogin, ActorKind: r.ActorKind, Action: r.Action,
		TargetType: r.TargetType, TargetID: r.TargetID, CreatedAt: r.CreatedAt}
	if r.Ip != nil {
		e.IP = *r.Ip
	}
	if err := json.Unmarshal(r.Data, &e.Data); err != nil {
		return Event{}, fmt.Errorf("audit: decode data: %w", err)
	}
	return e, nil
}
