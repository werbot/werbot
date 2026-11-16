package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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

const maxTeamName = 64 // team.name CHECK

// Team is a named group of an organization's members.
type Team struct {
	ID            uuid.UUID
	Name          string
	MemberUserIDs []uuid.UUID
	CreatedAt     time.Time
}

// teamName trims name and checks the 1..maxTeamName rune range of the DB CHECK.
func teamName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxTeamName {
		return "", fmt.Errorf("team name must be 1-%d characters: %w", maxTeamName, apperr.ErrInvalidArgument)
	}
	return name, nil
}

// teamTx runs fn in the caller's organization; only owners and admins manage teams.
func (s *Service) teamTx(ctx context.Context, fn func(p principal.Principal, tx pgx.Tx) error) error {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error { return fn(p, tx) })
}

func writeTeamAudit(ctx context.Context, tx pgx.Tx, action string, p principal.Principal, id uuid.UUID, data map[string]any) error {
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: action,
		TargetType: "team", TargetID: id.String(), IP: clientip.From(ctx), Data: data,
	})
}

// teamErr maps no rows to ErrNotFound and a duplicate name to ErrAlreadyExists.
func teamErr(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("team not found: %w", apperr.ErrNotFound)
	case isUniqueViolation(err):
		return fmt.Errorf("team name already used: %w", apperr.ErrAlreadyExists)
	}
	return err
}

// ListTeams returns the caller's organization's teams with their members.
func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	var teams []Team
	err := s.teamTx(ctx, func(p principal.Principal, tx pgx.Tx) error {
		rows, err := newStore(tx).ListTeams(ctx, p.OrgID)
		for _, r := range rows {
			teams = append(teams, Team{ID: r.ID, Name: r.Name, MemberUserIDs: r.MemberUserIds, CreatedAt: r.CreatedAt})
		}
		return err
	})
	return teams, err
}

// CreateTeam creates an empty team; a name already used is ErrAlreadyExists.
func (s *Service) CreateTeam(ctx context.Context, name string) (Team, error) {
	name, err := teamName(name)
	if err != nil {
		return Team{}, err
	}
	var t Team
	err = s.teamTx(ctx, func(p principal.Principal, tx pgx.Tx) error {
		r, err := newStore(tx).CreateTeam(ctx, dbq.CreateTeamParams{OrgID: p.OrgID, Name: name})
		if err != nil {
			return err
		}
		t = Team{ID: r.ID, Name: name, CreatedAt: r.CreatedAt}
		return writeTeamAudit(ctx, tx, "team.create", p, t.ID, map[string]any{"name": name})
	})
	return t, teamErr(err)
}

// RenameTeam renames a team of the caller's organization.
func (s *Service) RenameTeam(ctx context.Context, id uuid.UUID, name string) (Team, error) {
	name, err := teamName(name)
	if err != nil {
		return Team{}, err
	}
	var t Team
	err = s.teamTx(ctx, func(p principal.Principal, tx pgx.Tx) error {
		st := newStore(tx)
		r, err := st.RenameTeam(ctx, dbq.RenameTeamParams{ID: id, OrgID: p.OrgID, Name: name})
		if err != nil {
			return err
		}
		members, err := st.TeamMemberIDs(ctx, dbq.TeamMemberIDsParams{TeamID: id, OrgID: p.OrgID})
		if err != nil {
			return err
		}
		t = Team{ID: r.ID, Name: r.Name, MemberUserIDs: members, CreatedAt: r.CreatedAt}
		return writeTeamAudit(ctx, tx, "team.rename", p, id, map[string]any{"name": name})
	})
	return t, teamErr(err)
}

// DeleteTeam deletes a team of the caller's organization with its memberships.
func (s *Service) DeleteTeam(ctx context.Context, id uuid.UUID) error {
	return teamErr(s.teamTx(ctx, func(p principal.Principal, tx pgx.Tx) error {
		name, err := newStore(tx).DeleteTeam(ctx, dbq.DeleteTeamParams{ID: id, OrgID: p.OrgID})
		if err != nil {
			return err
		}
		return writeTeamAudit(ctx, tx, "team.delete", p, id, map[string]any{"name": name})
	}))
}

// AddTeamMember adds a member of the caller's organization to a team. An
// unknown team or a user outside the organization is ErrNotFound.
func (s *Service) AddTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	err := s.teamTx(ctx, func(p principal.Principal, tx pgx.Tx) error {
		if err := newStore(tx).AddTeamMember(ctx, dbq.AddTeamMemberParams{TeamID: teamID, OrgID: p.OrgID, UserID: userID}); err != nil {
			return err
		}
		return writeTeamAudit(ctx, tx, "team.member_add", p, teamID, map[string]any{"user_id": userID})
	})
	switch {
	case isForeignKeyViolation(err):
		return fmt.Errorf("team or member not found: %w", apperr.ErrNotFound)
	case isUniqueViolation(err):
		return fmt.Errorf("already in the team: %w", apperr.ErrAlreadyExists)
	}
	return err
}

// RemoveTeamMember removes a user from a team; a missing membership is ErrNotFound.
func (s *Service) RemoveTeamMember(ctx context.Context, teamID, userID uuid.UUID) error {
	return s.teamTx(ctx, func(p principal.Principal, tx pgx.Tx) error {
		n, err := newStore(tx).RemoveTeamMember(ctx, dbq.RemoveTeamMemberParams{TeamID: teamID, OrgID: p.OrgID, UserID: userID})
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("team member not found: %w", apperr.ErrNotFound)
		}
		return writeTeamAudit(ctx, tx, "team.member_remove", p, teamID, map[string]any{"user_id": userID})
	})
}

// UserTeams returns the ids of the teams userID belongs to in orgID, for policy.
func (s *Service) UserTeams(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) (err error) {
		ids, err = newStore(tx).UserTeams(ctx, dbq.UserTeamsParams{OrgID: orgID, UserID: userID})
		return err
	})
	return ids, err
}
