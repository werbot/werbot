package oidclogin

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// IdentityProviderHandler serves IdentityProviderService; owner and admin only.
type IdentityProviderHandler struct {
	werbotv1connect.UnimplementedIdentityProviderServiceHandler
	store *Store
	cfg   Config
}

// NewIdentityProviderHandler returns the handler; cfg gives the redirect URI
// and, without SecureCookies (development), allows http:// issuers.
func NewIdentityProviderHandler(store *Store, cfg Config) *IdentityProviderHandler {
	return &IdentityProviderHandler{store: store, cfg: cfg}
}

func manager(ctx context.Context) (principal.Principal, error) {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	return p, apperr.ToConnect(err)
}

// ListIdentityProviders lists the organization's IdPs by name.
func (h *IdentityProviderHandler) ListIdentityProviders(ctx context.Context, _ *connect.Request[werbotv1.ListIdentityProvidersRequest]) (*connect.Response[werbotv1.ListIdentityProvidersResponse], error) {
	p, err := manager(ctx)
	if err != nil {
		return nil, err
	}
	idps, err := h.store.List(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListIdentityProvidersResponse{}
	for _, idp := range idps {
		res.IdentityProviders = append(res.IdentityProviders, h.toProto(idp))
	}
	return connect.NewResponse(res), nil
}

// GetIdentityProvider returns one IdP of the organization.
func (h *IdentityProviderHandler) GetIdentityProvider(ctx context.Context, req *connect.Request[werbotv1.GetIdentityProviderRequest]) (*connect.Response[werbotv1.GetIdentityProviderResponse], error) {
	p, err := manager(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	idp, err := h.store.GetInOrg(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetIdentityProviderResponse{IdentityProvider: h.toProto(idp)}), nil
}

// CreateIdentityProvider stores a new IdP with its sealed client secret.
func (h *IdentityProviderHandler) CreateIdentityProvider(ctx context.Context, req *connect.Request[werbotv1.CreateIdentityProviderRequest]) (*connect.Response[werbotv1.CreateIdentityProviderResponse], error) {
	p, err := manager(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in := IdP{Name: m.GetName(), Issuer: m.GetIssuer(), ClientID: m.GetClientId(), ClientSecret: m.GetClientSecret(),
		Scopes: m.GetScopes(), AllowSignup: m.GetAllowSignup(), Enabled: m.GetEnabled()}
	idp, err := h.save(in, m.GetClaimsMapping(), true, func(in IdP) (IdP, error) { return h.store.Create(ctx, p, in) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.CreateIdentityProviderResponse{IdentityProvider: h.toProto(idp)}), nil
}

// UpdateIdentityProvider replaces an IdP; an empty client secret keeps the stored one.
func (h *IdentityProviderHandler) UpdateIdentityProvider(ctx context.Context, req *connect.Request[werbotv1.UpdateIdentityProviderRequest]) (*connect.Response[werbotv1.UpdateIdentityProviderResponse], error) {
	p, err := manager(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := parseID(m.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	in := IdP{ID: id, Name: m.GetName(), Issuer: m.GetIssuer(), ClientID: m.GetClientId(), ClientSecret: m.GetClientSecret(),
		Scopes: m.GetScopes(), AllowSignup: m.GetAllowSignup(), Enabled: m.GetEnabled()}
	idp, err := h.save(in, m.GetClaimsMapping(), false, func(in IdP) (IdP, error) { return h.store.Update(ctx, p, in) })
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.UpdateIdentityProviderResponse{IdentityProvider: h.toProto(idp)}), nil
}

// DeleteIdentityProvider removes an IdP and its links.
func (h *IdentityProviderHandler) DeleteIdentityProvider(ctx context.Context, req *connect.Request[werbotv1.DeleteIdentityProviderRequest]) (*connect.Response[werbotv1.DeleteIdentityProviderResponse], error) {
	p, err := manager(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.store.Delete(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteIdentityProviderResponse{}), nil
}

// save validates in with the mapping of the request and stores it with fn.
func (h *IdentityProviderHandler) save(in IdP, mapping []*werbotv1.ClaimMapping, create bool, fn func(IdP) (IdP, error)) (IdP, error) {
	var err error
	if in.Mapping, err = rulesFromProto(mapping); err == nil {
		err = validate(&in, !h.cfg.SecureCookies, create)
	}
	if err == nil {
		in, err = fn(in)
	}
	return in, apperr.ToConnect(err)
}

func rulesFromProto(ms []*werbotv1.ClaimMapping) ([]MappingRule, error) {
	out := make([]MappingRule, 0, len(ms))
	for _, m := range ms {
		r := MappingRule{Claim: m.GetClaim(), Regex: m.GetRegex(), Role: m.GetRole()}
		if t, ok := m.GetTarget().(*werbotv1.ClaimMapping_TeamId); ok {
			id, err := parseID(t.TeamId)
			if err != nil {
				return nil, err
			}
			r.TeamID = &id
		}
		out = append(out, r)
	}
	return out, nil
}

// toProto is idp without its client secret, with the redirect URI.
func (h *IdentityProviderHandler) toProto(idp IdP) *werbotv1.IdentityProvider {
	out := &werbotv1.IdentityProvider{Id: idp.ID.String(), Name: idp.Name, Issuer: idp.Issuer, ClientId: idp.ClientID,
		Scopes: idp.Scopes, AllowSignup: idp.AllowSignup, Enabled: idp.Enabled, CreatedAt: timestamppb.New(idp.CreatedAt),
		RedirectUri: h.cfg.redirectURI()}
	for _, r := range idp.Mapping {
		m := &werbotv1.ClaimMapping{Claim: r.Claim, Regex: r.Regex, Target: &werbotv1.ClaimMapping_Role{Role: r.Role}}
		if r.TeamID != nil {
			m.Target = &werbotv1.ClaimMapping_TeamId{TeamId: r.TeamID.String()}
		}
		out.ClaimsMapping = append(out.ClaimsMapping, m)
	}
	return out
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, invalid("invalid id")
	}
	return id, nil
}
