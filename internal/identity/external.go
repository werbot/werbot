package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ExternalLogin is a user an IdP vouched for (oidclogin).
type ExternalLogin struct {
	OrgID, IdPID               uuid.UUID //nolint:revive // name fixed by contract
	Subject, Email, Name       string
	EmailVerified, AllowSignup bool
	Teams                      map[uuid.UUID]bool // mapped teams only: true joins, false leaves
	Role                       string             // "" keeps the role; "admin" | "member"; owner never changes
	RefreshCiphertext          []byte             // the sealed IdP refresh token; nil keeps the stored one
	KeyID                      string
}

// ErrNoAccount answers an IdP login that matches no member and may not create one.
var ErrNoAccount = fmt.Errorf("no linked account: %w", apperr.ErrPermissionDenied)

// ProviderRef names an enabled identity provider on the login page.
type ProviderRef struct {
	ID   uuid.UUID
	Name string
}

// LoginExternal opens a session for a user an IdP vouched for: the member
// linked to (IdPID, Subject); else the member with the verified email, who
// gets linked; else, with AllowSignup and a verified email, a new member
// without a password. Otherwise it is ErrNoAccount. Mapped teams and the
// role are synced; no Werbot second factor is asked (owner decision O1).
// auth.login is written with method oidc.
func (s *Service) LoginExternal(ctx context.Context, in ExternalLogin, userAgent string) (token string, err error) {
	err = dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		st := newStore(tx)
		userID, err := externalUser(ctx, st, in)
		if err != nil {
			return err
		}
		err = st.UpsertIdPLink(ctx, dbq.UpsertIdPLinkParams{UserID: userID, IdpID: in.IdPID, OrgID: in.OrgID,
			Subject: in.Subject, RefreshCiphertext: in.RefreshCiphertext, KeyID: nullable(in.KeyID)})
		if isUniqueViolation(err) { // the user is linked to another subject of this IdP
			return ErrNoAccount
		}
		if err != nil {
			return err
		}
		data, err := syncExternal(ctx, st, in, userID)
		if err != nil {
			return err
		}
		if token, err = s.createSession(ctx, st, userID, in.OrgID, userAgent); err != nil {
			return err
		}
		return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &in.OrgID, ActorUserID: &userID, ActorKind: "user",
			Action: "auth.login", IP: clientip.From(ctx), Data: data})
	})
	return token, err
}

// externalUser finds or provisions the member of in.
func externalUser(ctx context.Context, st Store, in ExternalLogin) (uuid.UUID, error) {
	id, err := st.GetIdPLinkUser(ctx, dbq.GetIdPLinkUserParams{IdpID: in.IdPID, Subject: in.Subject, OrgID: in.OrgID})
	if !errors.Is(err, pgx.ErrNoRows) {
		return id, err
	}
	if !in.EmailVerified { // an unverified email never links nor signs up
		return uuid.Nil, ErrNoAccount
	}
	email := normalizeEmail(in.Email)
	id, err = st.GetMemberByEmail(ctx, dbq.GetMemberByEmailParams{OrgID: in.OrgID, Email: email})
	if !errors.Is(err, pgx.ErrNoRows) {
		return id, err
	}
	if !in.AllowSignup {
		return uuid.Nil, ErrNoAccount
	}
	return signupUser(ctx, st, in, email)
}

// signupUser creates a member without a password (owner decision O2).
func signupUser(ctx context.Context, st Store, in ExternalLogin, email string) (uuid.UUID, error) {
	login, err := uniqueLogin(ctx, st, signupLogin(email))
	if err != nil {
		return uuid.Nil, err
	}
	name := in.Name
	if name == "" || utf8.RuneCountInString(name) > 128 {
		name = login
	}
	id, err := st.CreateUser(ctx, dbq.CreateUserParams{Email: email, Login: login, Name: name})
	if isUniqueViolation(err) { // the email belongs to a user outside the organization
		return uuid.Nil, ErrNoAccount
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, st.CreateMembership(ctx, dbq.CreateMembershipParams{OrgID: in.OrgID, UserID: id, Role: principal.RoleMember})
}

// syncExternal applies the mapped teams and role of in to userID and returns
// the auth.login data: the role is listed when it changed.
func syncExternal(ctx context.Context, st Store, in ExternalLogin, userID uuid.UUID) (map[string]any, error) {
	var join, leave []uuid.UUID
	for id, member := range in.Teams {
		if member {
			join = append(join, id)
		} else {
			leave = append(leave, id)
		}
	}
	if err := st.AddExternalTeams(ctx, dbq.AddExternalTeamsParams{UserID: userID, OrgID: in.OrgID, TeamIds: join}); err != nil {
		return nil, err
	}
	if err := st.RemoveExternalTeams(ctx, dbq.RemoveExternalTeamsParams{OrgID: in.OrgID, UserID: userID, TeamIds: leave}); err != nil {
		return nil, err
	}
	data := map[string]any{"method": "oidc", "idp_id": in.IdPID}
	if in.Role != principal.RoleAdmin && in.Role != principal.RoleMember {
		return data, nil
	}
	_, err := st.SetExternalRole(ctx, dbq.SetExternalRoleParams{Role: in.Role, OrgID: in.OrgID, UserID: userID})
	switch {
	case err == nil:
		data["role"] = in.Role
	case !errors.Is(err, pgx.ErrNoRows): // no row: an owner, or the role already
		return nil, err
	}
	return data, nil
}

// SetSessionCookie sets the session cookie of token on w.
func (s *Service) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, newSessionCookie(token, sessionMaxAge, s.cfg.SecureCookies))
}

// RevokeUserSessions ends the cookie sessions of userID in orgID and revokes
// its CLI sessions with reason.
func (s *Service) RevokeUserSessions(ctx context.Context, orgID, userID uuid.UUID, reason string) error {
	return dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := st.DeleteOrgUserSessions(ctx, dbq.DeleteOrgUserSessionsParams{OrgID: orgID, UserID: userID}); err != nil {
			return err
		}
		return st.RevokeOrgUserCLISessions(ctx, dbq.RevokeOrgUserCLISessionsParams{Reason: reason, OrgID: orgID, UserID: userID})
	})
}

// IdentityProviders lists every enabled IdP of the installation for the login page.
func (s *Service) IdentityProviders(ctx context.Context) ([]ProviderRef, error) {
	var out []ProviderRef
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ListEnabledIdPs(ctx)
		for _, r := range rows {
			out = append(out, ProviderRef{ID: r.ID, Name: r.Name})
		}
		return err
	})
	return out, err
}
