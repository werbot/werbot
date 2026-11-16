package identity

import (
	"context"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/identity/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	LockSetup(ctx context.Context, key int64) error
	HasUsers(ctx context.Context) (bool, error)
	CreateOrganization(ctx context.Context, name string) (uuid.UUID, error)
	CreateUser(ctx context.Context, arg dbq.CreateUserParams) (uuid.UUID, error)
	CreateMembership(ctx context.Context, arg dbq.CreateMembershipParams) error
	GetLoginUser(ctx context.Context, email string) (dbq.GetLoginUserRow, error)
	CreateSession(ctx context.Context, arg dbq.CreateSessionParams) error
	GetSessionByToken(ctx context.Context, tokenHash []byte) (dbq.GetSessionByTokenRow, error)
	ExtendSession(ctx context.Context, arg dbq.ExtendSessionParams) error
	DeleteSession(ctx context.Context, arg dbq.DeleteSessionParams) error
	GetSessionInfo(ctx context.Context, arg dbq.GetSessionInfoParams) (dbq.GetSessionInfoRow, error)
	CreateOneTimeToken(ctx context.Context, arg dbq.CreateOneTimeTokenParams) error
	ConsumeOneTimeToken(ctx context.Context, arg dbq.ConsumeOneTimeTokenParams) (dbq.ConsumeOneTimeTokenRow, error)
	GetResetUser(ctx context.Context, email string) (dbq.GetResetUserRow, error)
	UpdateUserPassword(ctx context.Context, arg dbq.UpdateUserPasswordParams) error
	GetUserOrg(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
	GetProfile(ctx context.Context, id uuid.UUID) (dbq.GetProfileRow, error)
	UpdateProfile(ctx context.Context, arg dbq.UpdateProfileParams) error
	GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error)
	DeleteOtherUserSessions(ctx context.Context, arg dbq.DeleteOtherUserSessionsParams) error
	LockOrgOwners(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}
