package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Device codes (RFC 8628). The device code is stored only as its SHA-256;
// the user code is what the person types on the device page.

var (
	errNoDevice      = fmt.Errorf("device code: %w", apperr.ErrNotFound)
	errUnknownDevice = errors.New("unknown device code") // op answers access_denied
)

// deviceGrant is the approved code a poll found: the token step of the same
// request consumes it.
type deviceGrant struct{ id, orgID uuid.UUID }

// StoreDeviceAuthorization implements op.DeviceAuthorizationStorage; the
// scopes follow the loopback rule (offline_access required).
func (s *storage) StoreDeviceAuthorization(ctx context.Context, clientID, deviceCode, userCode string, expires time.Time, scopes []string) error {
	if err := checkScopes(scopes); err != nil {
		return err
	}
	ri := requestInfo(ctx)
	return dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		return dbq.New(tx).CreateDeviceCode(ctx, dbq.CreateDeviceCodeParams{
			DeviceCodeHash: hashSecret(deviceCode), UserCode: userCode, ClientID: clientID, Scopes: scopes,
			ClientHostname: ri.hostname, ClientIp: ri.ipPtr(), ExpiresAt: expires,
		})
	})
}

// GetDeviceAuthorizatonState implements op.DeviceAuthorizationStorage (op's
// spelling). A used or ended code reads as expired (expired_token); only an
// unknown code or a store failure is an error (access_denied, R11).
func (s *storage) GetDeviceAuthorizatonState(ctx context.Context, clientID, deviceCode string) (*op.DeviceAuthorizationState, error) {
	var r dbq.PollDeviceCodeRow
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		r, err = dbq.New(tx).PollDeviceCode(ctx, dbq.PollDeviceCodeParams{DeviceCodeHash: hashSecret(deviceCode), ClientID: clientID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errUnknownDevice
	}
	if err != nil {
		return nil, err
	}
	st := &op.DeviceAuthorizationState{ClientID: clientID, Scopes: r.Scopes}
	switch {
	case r.Status == "denied":
		st.Denied = true
	case !r.Live || r.Status == "used":
		// zero Expires: already past
	case r.Status == "approved" && r.OrgID != nil: // approve always sets org_id; without it, stay pending
		st.Done, st.Subject, st.Audience = true, r.UserID.String(), []string{Audience}
		requestInfo(ctx).device = &deviceGrant{id: r.ID, orgID: *r.OrgID}
	default: // pending
		st.Expires = r.ExpiresAt
	}
	return st, nil
}

// exchangeDevice consumes the approved code the poll found and opens its
// cli_session: of concurrent polls exactly one wins, the others get invalid_grant.
func (s *storage) exchangeDevice(ctx context.Context) (string, string, time.Time, error) {
	d := requestInfo(ctx).device
	if d == nil {
		return "", "", time.Time{}, oidc.ErrInvalidGrant()
	}
	return s.openSession(ctx, "device", func(q *dbq.Queries) (dbq.ConsumeAuthRequestRow, error) {
		r, err := q.ConsumeDeviceCode(ctx, d.id)
		return dbq.ConsumeAuthRequestRow(r), err
	})
}

// LookupDeviceCode returns the pending (live) code the person typed.
func (p *Provider) LookupDeviceCode(ctx context.Context, pr principal.Principal, userCode string) (ClientInfo, error) {
	code, err := consentUserCode(pr, userCode)
	if err != nil {
		return ClientInfo{}, err
	}
	return p.pending(ctx, errNoDevice, func(q *dbq.Queries) (dbq.GetPendingAuthRequestRow, error) {
		r, err := q.GetPendingDeviceCode(ctx, code)
		return dbq.GetPendingAuthRequestRow(r), err
	})
}

// ApproveDeviceCode binds the pending code to the caller; the next poll gets tokens.
func (p *Provider) ApproveDeviceCode(ctx context.Context, pr principal.Principal, userCode string) error {
	code, err := consentUserCode(pr, userCode)
	if err != nil {
		return err
	}
	if _, err := p.users.Membership(ctx, pr.OrgID, pr.UserID); err != nil {
		return err
	}
	return p.update(ctx, errNoDevice, func(q *dbq.Queries) (int64, error) {
		return q.ApproveDeviceCode(ctx, dbq.ApproveDeviceCodeParams{UserCode: code, UserID: &pr.UserID, OrgID: &pr.OrgID})
	})
}

// DenyDeviceCode refuses the pending code; the next poll gets access_denied.
func (p *Provider) DenyDeviceCode(ctx context.Context, pr principal.Principal, userCode string) error {
	code, err := consentUserCode(pr, userCode)
	if err != nil {
		return err
	}
	return p.update(ctx, errNoDevice, func(q *dbq.Queries) (int64, error) { return q.DenyDeviceCode(ctx, code) })
}

// consentUserCode: a browser session and a well-formed user code.
func consentUserCode(pr principal.Principal, userCode string) (string, error) {
	if err := requireCookie(pr); err != nil {
		return "", err
	}
	return normalizeUserCode(userCode)
}

// normalizeUserCode accepts any case, spaces and dashes; the rest must be
// exactly the 8 letters of op.UserCodeBase20, returned as XXXX-XXXX.
func normalizeUserCode(s string) (string, error) {
	s = strings.ToUpper(strings.Map(func(r rune) rune {
		if r == '-' || unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s))
	if len(s) != 8 || strings.Trim(s, op.CharSetBase20) != "" {
		return "", fmt.Errorf("user code: %w", apperr.ErrInvalidArgument)
	}
	return s[:4] + "-" + s[4:], nil
}
