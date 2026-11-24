package oidclogin

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func invalid(msg string) error { return fmt.Errorf("%s: %w", msg, apperr.ErrInvalidArgument) }

// validate checks in for the store: name, an https issuer (http with
// allowHTTP), a client, scopes with openid (empty: DefaultScopes) and the
// mapping rules; secretRequired is set for a new IdP.
func validate(in *IdP, allowHTTP, secretRequired bool) error {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Scopes) == 0 {
		in.Scopes = DefaultScopes
	}
	if err := validateClient(*in, secretRequired); err != nil {
		return err
	}
	if err := validateIssuer(in.Issuer, allowHTTP); err != nil {
		return err
	}
	if !slices.Contains(in.Scopes, "openid") {
		return invalid("scopes must contain openid")
	}
	for _, r := range in.Mapping {
		if err := validateRule(r); err != nil {
			return err
		}
	}
	return nil
}

// validateClient checks the name and the client credentials.
func validateClient(in IdP, secretRequired bool) error {
	switch {
	case in.Name == "" || len(in.Name) > 63:
		return invalid("name must be 1 to 63 characters")
	case in.ClientID == "":
		return invalid("client id is required")
	case secretRequired && in.ClientSecret == "":
		return invalid("client secret is required")
	}
	return nil
}

// validateIssuer accepts an https URL without query or fragment; http only
// with allowHTTP (WERBOT_ENV=development).
func validateIssuer(issuer string, allowHTTP bool) error {
	u, err := url.Parse(issuer)
	switch {
	case err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil:
		return invalid("issuer must be a URL without query or fragment")
	case u.Scheme == "https", u.Scheme == "http" && allowHTTP:
		return nil
	}
	return invalid("issuer must use https")
}

// validateRule needs a claim, a regex that compiles and exactly one target:
// a team or the role admin or member (owner is never granted).
func validateRule(r MappingRule) error {
	if _, err := anchored(r.Regex); err != nil || r.Claim == "" {
		return invalid("claims mapping needs a claim and a valid regex")
	}
	if (r.TeamID == nil) == (r.Role == "") {
		return invalid("claims mapping needs a team or a role")
	}
	if r.TeamID == nil && r.Role != principal.RoleAdmin && r.Role != principal.RoleMember {
		return invalid("claims mapping role must be admin or member")
	}
	return nil
}
