package identity

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// setupLock serializes Setup across replicas: "WERBOT" + 2, next after db.BootstrapLock.
const setupLock int64 = 0x574552424f540002

// Service implements the identity use cases.
type Service struct {
	pool   *pgxpool.Pool
	emails EmailQueue
	cfg    Config
}

// New returns a Service; emails receives invite and reset emails.
func New(pool *pgxpool.Pool, emails EmailQueue, cfg Config) *Service {
	return &Service{pool: pool, emails: emails, cfg: cfg}
}

// NeedsSetup reports whether no user exists yet.
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	var has bool
	err := dbx.System(ctx, s.pool, dbx.ReasonSetup, func(tx pgx.Tx) error {
		var err error
		has, err = newStore(tx).HasUsers(ctx)
		return err
	})
	return !has, err
}

// Setup creates the first organization, its owner and a session, and returns
// the session token. Once any user exists it fails with ErrFailedPrecondition.
func (s *Service) Setup(ctx context.Context, in SetupInput, userAgent string) (string, error) {
	if err := validatePassword(in.Password); err != nil {
		return "", err
	}
	var token string
	err := dbx.System(ctx, s.pool, dbx.ReasonSetup, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := st.LockSetup(ctx, setupLock); err != nil {
			return err
		}
		has, err := st.HasUsers(ctx)
		if err != nil {
			return err
		}
		if has {
			return fmt.Errorf("setup already completed: %w", apperr.ErrFailedPrecondition)
		}
		// Hashed only after the check: a completed setup costs no argon2 work.
		hash, err := hashPassword(in.Password)
		if err != nil {
			return err
		}
		orgID, err := st.CreateOrganization(ctx, in.OrganizationName)
		if err != nil {
			return err
		}
		userID, err := st.CreateUser(ctx, dbq.CreateUserParams{
			Email: normalizeEmail(in.Email), Login: in.Login, Name: in.Name, PasswordHash: hash,
		})
		if err != nil {
			return err
		}
		if err := st.CreateMembership(ctx, dbq.CreateMembershipParams{OrgID: orgID, UserID: userID, Role: principal.RoleOwner}); err != nil {
			return err
		}
		if token, err = s.createSession(ctx, st, userID, orgID, userAgent); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "auth.setup", orgID, userID)
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// writeAudit records action by userID in orgID inside tx.
func writeAudit(ctx context.Context, tx pgx.Tx, action string, orgID, userID uuid.UUID) error {
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &orgID, ActorUserID: &userID, ActorKind: "user", Action: action, IP: clientip.From(ctx),
	})
}
