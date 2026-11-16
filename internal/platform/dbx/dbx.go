// Package dbx runs database work in transactions that carry the caller's
// organization context (InOrg) or an explicit system reason (System).
package dbx

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Reason names why a system (no-organization) transaction is needed.
// Callers are only the packages on the system_callers list.
type Reason string

// System transaction reasons.
const (
	ReasonLogin       Reason = "login"
	ReasonSetup       Reason = "setup"
	ReasonBootstrap   Reason = "bootstrap"
	ReasonTunnel      Reason = "tunnel"
	ReasonPartitions  Reason = "partitions"
	ReasonRetention   Reason = "retention"
	ReasonFanout      Reason = "fanout"
	ReasonInstance    Reason = "instance"
	ReasonDomains     Reason = "domains"
	ReasonSupport     Reason = "support"
	ReasonPurge       Reason = "purge"
	ReasonUsage       Reason = "usage"
	ReasonModule      Reason = "module"
	ReasonSession     Reason = "session"
	ReasonEnroll      Reason = "enroll"
	ReasonMaintenance Reason = "maintenance"
	ReasonRate        Reason = "rate"
	ReasonCluster     Reason = "cluster"
)

// ErrNoOrg means InOrg was called without an organization.
var ErrNoOrg = errors.New("dbx: organization context required")

// InOrg runs fn in a transaction with werbot.org_id and werbot.user_id set
// for the transaction. user may be uuid.Nil.
func InOrg(ctx context.Context, pool *pgxpool.Pool, org, user uuid.UUID, fn func(pgx.Tx) error) error {
	if org == uuid.Nil {
		return ErrNoOrg
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			"SELECT set_config('werbot.org_id', $1, true), set_config('werbot.user_id', $2, true)",
			org.String(), user.String()); err != nil {
			return err
		}
		return fn(tx)
	})
}

// System runs fn in a transaction without organization context.
func System(ctx context.Context, pool *pgxpool.Pool, reason Reason, fn func(pgx.Tx) error) error {
	if reason == "" {
		return errors.New("dbx: system reason required")
	}
	return pgx.BeginFunc(ctx, pool, fn)
}
