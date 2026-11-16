package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Member is a user's membership in the caller's organization.
type Member struct {
	User     User
	Role     string
	JoinedAt time.Time
}

// Invite is a pending invitation: not accepted and not expired.
type Invite struct {
	Email, Role string
	ExpiresAt   time.Time
}

// AcceptInput accepts an invite; Login and Name are needed only for a new
// user, Password for anyone without a session.
type AcceptInput struct {
	Token, Login, Name, Password string
}

var roles = []string{principal.RoleOwner, principal.RoleAdmin, principal.RoleMember}

// checkRole rejects an unknown role and lets only an owner grant "owner".
func checkRole(p principal.Principal, role string) error {
	if !slices.Contains(roles, role) {
		return fmt.Errorf("unknown role %q: %w", role, apperr.ErrInvalidArgument)
	}
	if role == principal.RoleOwner && p.Role != principal.RoleOwner {
		return fmt.Errorf("only an owner grants owner: %w", apperr.ErrPermissionDenied)
	}
	return nil
}

// lockTarget returns userID's role in p's organization; only an owner may
// change an owner. Owner rows are locked first, as ensureNotLastOwner does,
// so concurrent role changes take their locks in one order.
func lockTarget(ctx context.Context, st Store, p principal.Principal, userID uuid.UUID) (string, error) {
	if _, err := st.LockOrgOwners(ctx, p.OrgID); err != nil {
		return "", err
	}
	role, err := st.LockMemberRole(ctx, dbq.LockMemberRoleParams{OrgID: p.OrgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("member not found: %w", apperr.ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	if role == principal.RoleOwner && p.Role != principal.RoleOwner {
		return "", fmt.Errorf("only an owner changes an owner: %w", apperr.ErrPermissionDenied)
	}
	return role, nil
}

// writeMemberAudit records action on the user target; uuid.Nil means no target.
func writeMemberAudit(ctx context.Context, tx pgx.Tx, action string, orgID, actorID, target uuid.UUID, data map[string]any) error {
	e := auditlog.Event{OrgID: &orgID, ActorUserID: &actorID, ActorKind: "user", Action: action, IP: clientip.From(ctx), Data: data}
	if target != uuid.Nil {
		e.TargetType, e.TargetID = "user", target.String()
	}
	return auditlog.Write(ctx, tx, e)
}

// ListMembers returns the members and pending invites of the caller's organization.
func (s *Service) ListMembers(ctx context.Context) ([]Member, []Invite, error) {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return nil, nil, err
	}
	var (
		members []Member
		invites []Invite
	)
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		rows, err := st.ListMembers(ctx, p.OrgID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			members = append(members, Member{
				User: User{ID: r.ID, Email: r.Email, Login: r.Login, Name: r.Name, CreatedAt: r.CreatedAt},
				Role: r.Role, JoinedAt: r.JoinedAt,
			})
		}
		inv, err := st.ListInvites(ctx, p.OrgID)
		for _, r := range inv {
			invites = append(invites, Invite(r))
		}
		return err
	})
	return members, invites, err
}

// InviteMember emails an invitation to join the caller's organization with role.
func (s *Service) InviteMember(ctx context.Context, email, role string) error {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return err
	}
	if err := checkRole(p, role); err != nil {
		return err
	}
	email = normalizeEmail(email)
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		switch member, err := st.IsMemberEmail(ctx, dbq.IsMemberEmailParams{OrgID: p.OrgID, Email: email}); {
		case err != nil:
			return err
		case member:
			return fmt.Errorf("already a member: %w", apperr.ErrAlreadyExists)
		}
		inviter, err := st.GetSessionInfo(ctx, dbq.GetSessionInfoParams{OrgID: p.OrgID, UserID: p.UserID})
		if err != nil {
			return err
		}
		raw, err := issueToken(ctx, st, tokenInvite, tokenClaims{OrgID: &p.OrgID, Email: email, Role: role})
		if err != nil {
			return err
		}
		data := map[string]string{"url": s.cfg.WebURL + "/invite/" + raw, "org": inviter.OrgName, "inviter": inviter.Name}
		if err := s.emails.EnqueueTx(ctx, tx, email, "invite", data); err != nil {
			return err
		}
		return writeMemberAudit(ctx, tx, "member.invite", p.OrgID, p.UserID, uuid.Nil, map[string]any{"email": email, "role": role})
	})
}

// UpdateMemberRole sets the role of userID in the caller's organization.
func (s *Service) UpdateMemberRole(ctx context.Context, userID uuid.UUID, role string) error {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return err
	}
	if err := checkRole(p, role); err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		from, err := lockTarget(ctx, st, p, userID)
		if err != nil {
			return err
		}
		if role != principal.RoleOwner {
			if err := ensureNotLastOwner(ctx, st, p.OrgID, userID); err != nil {
				return err
			}
		}
		if err := st.UpdateMemberRole(ctx, dbq.UpdateMemberRoleParams{OrgID: p.OrgID, UserID: userID, Role: role}); err != nil {
			return err
		}
		return writeMemberAudit(ctx, tx, "member.role_update", p.OrgID, p.UserID, userID, map[string]any{"from": from, "to": role})
	})
}

// RemoveMember removes userID from the caller's organization; the user's
// sessions and team memberships there go with it.
func (s *Service) RemoveMember(ctx context.Context, userID uuid.UUID) error {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		if _, err := lockTarget(ctx, st, p, userID); err != nil {
			return err
		}
		if err := ensureNotLastOwner(ctx, st, p.OrgID, userID); err != nil {
			return err
		}
		if err := st.DeleteMembership(ctx, dbq.DeleteMembershipParams{OrgID: p.OrgID, UserID: userID}); err != nil {
			return err
		}
		return writeMemberAudit(ctx, tx, "member.remove", p.OrgID, p.UserID, userID, nil)
	})
}

// AcceptInvite joins the invited organization. A signed-in user must own the
// invited email and keeps the session (token ""); an existing user confirms
// with the password, a new one is created from Login, Name and Password.
// Both get a session in the organization. Any failure leaves the token unspent.
func (s *Service) AcceptInvite(ctx context.Context, in AcceptInput, userAgent string) (string, error) {
	p, signedIn := principal.From(ctx)
	var token string
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		st := newStore(tx)
		c, err := consumeToken(ctx, st, tokenInvite, in.Token)
		if err != nil {
			return err
		}
		if c.OrgID == nil {
			return fmt.Errorf("invalid or expired token: %w", apperr.ErrNotFound)
		}
		userID := p.UserID
		if signedIn {
			u, err := st.GetProfile(ctx, p.UserID)
			if err != nil {
				return err
			}
			if u.Email != c.Email {
				return fmt.Errorf("invite is for another email: %w", apperr.ErrPermissionDenied)
			}
		} else if userID, err = s.inviteeUser(ctx, st, c.Email, in); err != nil {
			return err
		}
		err = st.CreateMembership(ctx, dbq.CreateMembershipParams{OrgID: *c.OrgID, UserID: userID, Role: c.Role})
		if isUniqueViolation(err) {
			return fmt.Errorf("already a member: %w", apperr.ErrAlreadyExists)
		}
		if err != nil {
			return err
		}
		if !signedIn {
			if token, err = s.createSession(ctx, st, userID, *c.OrgID, userAgent); err != nil {
				return err
			}
		}
		return writeMemberAudit(ctx, tx, "member.accept", *c.OrgID, userID, userID, nil)
	})
	return token, err
}

// inviteeUser returns the user with email after checking in.Password, or
// creates one from in when no user has that email.
func (s *Service) inviteeUser(ctx context.Context, st Store, email string, in AcceptInput) (uuid.UUID, error) {
	u, err := st.GetLoginUser(ctx, email)
	if err == nil {
		ok, err := verifyPassword(u.PasswordHash, in.Password)
		if err != nil {
			return uuid.Nil, err
		}
		if !ok {
			return uuid.Nil, errInvalidCredentials
		}
		return u.ID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, err
	}
	if in.Login == "" || in.Name == "" {
		return uuid.Nil, fmt.Errorf("login and name are required: %w", apperr.ErrInvalidArgument)
	}
	if err := validatePassword(in.Password); err != nil {
		return uuid.Nil, err
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		return uuid.Nil, err
	}
	id, err := st.CreateUser(ctx, dbq.CreateUserParams{Email: email, Login: in.Login, Name: in.Name, PasswordHash: hash})
	if isUniqueViolation(err) {
		return uuid.Nil, fmt.Errorf("login already taken: %w", apperr.ErrAlreadyExists)
	}
	return id, err
}
