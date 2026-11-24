package oidclogin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/werbot/werbot/internal/identity/oidclogin/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// List returns the IdPs of the organization of p, by name, without secrets.
func (s *Store) List(ctx context.Context, p principal.Principal) ([]IdP, error) {
	var out []IdP
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		rows, err := dbq.New(tx).ListIdPs(ctx, p.OrgID)
		for _, r := range rows {
			idp, err := fromRow(r)
			if err != nil {
				return err
			}
			out = append(out, idp)
		}
		return err
	})
	return out, err
}

// GetInOrg returns IdP id of the organization of p without its secret.
func (s *Store) GetInOrg(ctx context.Context, p principal.Principal, id uuid.UUID) (IdP, error) {
	var out IdP
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		r, err := dbq.New(tx).GetOrgIdP(ctx, dbq.GetOrgIdPParams{ID: id, OrgID: p.OrgID})
		if err != nil {
			return err
		}
		out, err = fromRow(r)
		return err
	})
	return out, crudErr(err)
}

// Create stores in, already validated, in the organization of p (idp.create).
func (s *Store) Create(ctx context.Context, p principal.Principal, in IdP) (IdP, error) {
	sealed, keyID, err := s.box.Seal([]byte(in.ClientSecret))
	if err != nil {
		return IdP{}, err
	}
	mapping, err := json.Marshal(in.Mapping)
	if err != nil {
		return IdP{}, err
	}
	var out IdP
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := checkTeams(ctx, q, p, in.Mapping); err != nil {
			return err
		}
		r, err := q.CreateIdP(ctx, dbq.CreateIdPParams{OrgID: p.OrgID, Name: in.Name, Issuer: in.Issuer, ClientID: in.ClientID,
			ClientSecretCiphertext: sealed, KeyID: keyID, Scopes: in.Scopes, ClaimsMapping: mapping,
			AllowSignup: in.AllowSignup, Enabled: in.Enabled, CreatedBy: &p.UserID})
		if err != nil {
			return err
		}
		if out, err = fromRow(r); err != nil {
			return err
		}
		return audit(ctx, tx, p, "idp.create", out)
	})
	return out, crudErr(err)
}

// Update replaces IdP in.ID of the organization of p with in, already
// validated; an empty ClientSecret keeps the stored one (idp.update).
func (s *Store) Update(ctx context.Context, p principal.Principal, in IdP) (IdP, error) {
	arg := dbq.UpdateIdPParams{ID: in.ID, OrgID: p.OrgID, Name: in.Name, Issuer: in.Issuer, ClientID: in.ClientID,
		Scopes: in.Scopes, AllowSignup: in.AllowSignup, Enabled: in.Enabled}
	if in.ClientSecret != "" {
		sealed, keyID, err := s.box.Seal([]byte(in.ClientSecret))
		if err != nil {
			return IdP{}, err
		}
		arg.ClientSecretCiphertext, arg.KeyID = sealed, &keyID
	}
	var err error
	if arg.ClaimsMapping, err = json.Marshal(in.Mapping); err != nil {
		return IdP{}, err
	}
	var out IdP
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		if err := checkTeams(ctx, q, p, in.Mapping); err != nil {
			return err
		}
		r, err := q.UpdateIdP(ctx, arg)
		if err != nil {
			return err
		}
		if out, err = fromRow(r); err != nil {
			return err
		}
		return audit(ctx, tx, p, "idp.update", out)
	})
	return out, crudErr(err)
}

// Delete removes IdP id of the organization of p with its links (idp.delete).
func (s *Store) Delete(ctx context.Context, p principal.Principal, id uuid.UUID) error {
	return crudErr(dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		name, err := dbq.New(tx).DeleteIdP(ctx, dbq.DeleteIdPParams{ID: id, OrgID: p.OrgID})
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, "idp.delete", IdP{ID: id, Name: name})
	}))
}

// checkTeams refuses mapped teams outside the organization of p.
func checkTeams(ctx context.Context, q *dbq.Queries, p principal.Principal, rules []MappingRule) error {
	var ids []uuid.UUID
	for _, r := range rules {
		if r.TeamID != nil {
			ids = append(ids, *r.TeamID)
		}
	}
	n, err := q.CountOrgTeams(ctx, dbq.CountOrgTeamsParams{OrgID: p.OrgID, Ids: ids})
	if err == nil && n != int64(len(uniq(ids))) {
		err = invalid("claims mapping names a team outside the organization")
	}
	return err
}

func uniq(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// audit writes action on idp; the client secret is never part of it.
func audit(ctx context.Context, tx pgx.Tx, p principal.Principal, action string, idp IdP) error {
	data := map[string]any{"name": idp.Name}
	if action != "idp.delete" {
		data["issuer"], data["enabled"], data["allow_signup"] = idp.Issuer, idp.Enabled, idp.AllowSignup
	}
	return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: action,
		TargetType: "idp", TargetID: idp.ID.String(), IP: clientip.From(ctx), Data: data})
}

// crudErr maps a missing row to ErrNotFound and a taken name to ErrAlreadyExists.
func crudErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == "23505" {
		return fmt.Errorf("an identity provider with this name exists: %w", apperr.ErrAlreadyExists)
	}
	return err
}
