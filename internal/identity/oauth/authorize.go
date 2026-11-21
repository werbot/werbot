package oauth

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// errAuthRequestNotFound is what op shows when the callback id is unknown.
var errAuthRequestNotFound = errors.New("auth request not found")

// authRequest is an oauth_auth_request row as op.AuthRequest. codeHash is
// set when it was found by its code: the exchange consumes only that code.
type authRequest struct {
	dbq.GetAuthRequestRow
	codeHash []byte
}

var _ op.AuthRequest = (*authRequest)(nil)

func (a *authRequest) GetID() string                    { return a.ID.String() }
func (*authRequest) GetACR() string                     { return "" }
func (*authRequest) GetAMR() []string                   { return nil }
func (*authRequest) GetAudience() []string              { return []string{Audience} }
func (*authRequest) GetAuthTime() time.Time             { return time.Time{} }
func (*authRequest) GetClientID() string                { return ClientCLI }
func (*authRequest) GetNonce() string                   { return "" }
func (a *authRequest) GetRedirectURI() string           { return a.RedirectUri }
func (*authRequest) GetResponseType() oidc.ResponseType { return oidc.ResponseTypeCode }
func (*authRequest) GetResponseMode() oidc.ResponseMode { return oidc.ResponseModeQuery }
func (a *authRequest) GetScopes() []string              { return a.Scopes }
func (a *authRequest) GetState() string                 { return a.State }
func (a *authRequest) Done() bool                       { return a.GetAuthRequestRow.Done }

func (a *authRequest) GetCodeChallenge() *oidc.CodeChallenge {
	return &oidc.CodeChallenge{Challenge: a.CodeChallenge, Method: oidc.CodeChallengeMethodS256}
}

func (a *authRequest) GetSubject() string {
	if a.UserID == nil {
		return ""
	}
	return a.UserID.String()
}

// orgID is the organization the request was approved in.
func (a *authRequest) orgID() uuid.UUID {
	if a.OrgID == nil {
		return uuid.Nil
	}
	return *a.OrgID
}

func hashSecret(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// CreateAuthRequest implements op.Storage. op has already checked the client
// and reqInfo the redirect_uri; here: PKCE S256 and the scopes the CLI needs.
// Errors go to the (valid) redirect_uri.
func (s *storage) CreateAuthRequest(ctx context.Context, req *oidc.AuthRequest, _ string) (op.AuthRequest, error) {
	if req.CodeChallenge == "" || req.CodeChallengeMethod != oidc.CodeChallengeMethodS256 {
		return nil, oidc.ErrInvalidRequest().WithDescription("PKCE S256 required")
	}
	if err := checkScopes(req.Scopes); err != nil {
		return nil, err
	}
	ri := requestInfo(ctx)
	a := &authRequest{GetAuthRequestRow: dbq.GetAuthRequestRow{
		RedirectUri: req.RedirectURI, CodeChallenge: req.CodeChallenge, Scopes: req.Scopes, State: req.State,
	}}
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		a.ID, err = dbq.New(tx).CreateAuthRequest(ctx, dbq.CreateAuthRequestParams{
			ClientID: ClientCLI, RedirectUri: req.RedirectURI, CodeChallenge: req.CodeChallenge, Scopes: req.Scopes,
			State: req.State, ClientHostname: ri.hostname, ClientIp: ri.ipPtr(),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

// checkScopes: offline_access (the CLI lives on its refresh token), at most openid besides.
func checkScopes(scopes []string) error {
	if !slices.Contains(scopes, oidc.ScopeOfflineAccess) ||
		slices.ContainsFunc(scopes, func(sc string) bool { return sc != oidc.ScopeOpenID && sc != oidc.ScopeOfflineAccess }) {
		return oidc.ErrInvalidScope().WithDescription("scope must be offline_access, optionally with openid")
	}
	return nil
}

// AuthRequestByID implements op.Storage. Ended requests are found too: op
// needs their redirect_uri to report the end.
func (s *storage) AuthRequestByID(ctx context.Context, id string) (op.AuthRequest, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errAuthRequestNotFound
	}
	return s.findAuthRequest(ctx, nil, func(q *dbq.Queries) (dbq.GetAuthRequestRow, error) { return q.GetAuthRequest(ctx, uid) })
}

// AuthRequestByCode implements op.Storage: a live request by its code's hash.
func (s *storage) AuthRequestByCode(ctx context.Context, code string) (op.AuthRequest, error) {
	h := hashSecret(code)
	return s.findAuthRequest(ctx, h, func(q *dbq.Queries) (dbq.GetAuthRequestRow, error) {
		r, err := q.GetAuthRequestByCode(ctx, h)
		return dbq.GetAuthRequestRow(r), err
	})
}

func (s *storage) findAuthRequest(ctx context.Context, codeHash []byte, get func(*dbq.Queries) (dbq.GetAuthRequestRow, error)) (op.AuthRequest, error) {
	a := &authRequest{codeHash: codeHash}
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		a.GetAuthRequestRow, err = get(dbq.New(tx))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errAuthRequestNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// SaveAuthCode implements op.Storage: only an approved, live request gets a
// code; a second callback replaces the first code.
func (s *storage) SaveAuthCode(ctx context.Context, id, code string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return errAuthRequestNotFound
	}
	return dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		n, err := dbq.New(tx).SaveAuthCode(ctx, dbq.SaveAuthCodeParams{ID: uid, CodeHash: hashSecret(code)})
		if err == nil && n == 0 {
			return oidc.ErrInteractionRequired().WithDescription("the request is not approved or has ended")
		}
		return err
	})
}

// DeleteAuthRequest implements op.Storage. op calls it after the exchange,
// which already consumed the row: a missing row is not an error.
func (s *storage) DeleteAuthRequest(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil
	}
	return dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		return dbq.New(tx).DeleteAuthRequest(ctx, uid)
	})
}
