package oidclogin

import (
	"errors"
	"net/http"

	"github.com/zitadel/oidc/v3/pkg/client/rp"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/werbot/werbot/internal/identity"
)

// finish signs in the user the IdP vouched for: the ID token claims, with
// the userinfo claims over them, are mapped and handed to LoginExternal.
func (h *handler) finish(idp IdP) rp.CodeExchangeCallback[*oidc.IDTokenClaims] {
	return func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], _ string, party rp.RelyingParty) {
		claims := tokens.IDTokenClaims
		if party.UserinfoEndpoint() != "" {
			info, err := rp.Userinfo[*oidc.UserInfo](r.Context(), tokens.AccessToken, tokens.TokenType, claims.GetSubject(), party)
			if err != nil {
				h.fail(w, r, idp, "userinfo", err)
				return
			}
			claims.SetUserInfo(info)
		}
		in, err := h.external(idp, claims, tokens.RefreshToken)
		if err != nil {
			h.fail(w, r, idp, "seal", err)
			return
		}
		token, err := h.svc.LoginExternal(r.Context(), in, r.UserAgent())
		if errors.Is(err, identity.ErrNoAccount) {
			http.Redirect(w, r, loginFailed, http.StatusFound)
			return
		}
		if err != nil {
			h.fail(w, r, idp, "login", err)
			return
		}
		next, _ := h.cookies.CheckCookie(r, nextCookie)
		h.cookies.DeleteCookie(w, nextCookie)
		h.cookies.DeleteCookie(w, nonceCookie)
		h.svc.SetSessionCookie(w, token)
		http.Redirect(w, r, safeNext(next), http.StatusFound) //nolint:gosec // safeNext keeps a path of this origin only
	}
}

// external is the ExternalLogin of claims at idp; the refresh token is sealed.
func (h *handler) external(idp IdP, claims *oidc.IDTokenClaims, refresh string) (identity.ExternalLogin, error) {
	teams, role := ApplyMapping(idp.Mapping, claims.Claims)
	in := identity.ExternalLogin{OrgID: idp.OrgID, IdPID: idp.ID, Subject: claims.GetSubject(), Email: claims.Email,
		Name: claims.Name, EmailVerified: bool(claims.EmailVerified), AllowSignup: idp.AllowSignup, Teams: teams, Role: role}
	if refresh == "" {
		return in, nil
	}
	var err error
	in.RefreshCiphertext, in.KeyID, err = h.store.SealRefresh(refresh)
	return in, err
}
