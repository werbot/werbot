package oidclogin

import (
	"regexp"
	"slices"

	"github.com/google/uuid"
)

// ApplyMapping matches the claims against rules: a claim is a string or a
// list of strings, a regex is anchored at both ends. teams holds every
// mapped team (true: the user belongs, false: not); role is the highest
// matching role ("admin" over "member"), "member" when role rules exist but
// none matches, "" without role rules.
func ApplyMapping(rules []MappingRule, claims map[string]any) (teams map[uuid.UUID]bool, role string) {
	hasRoleRules := false
	for _, r := range rules {
		ok := matches(r.Regex, claims[r.Claim])
		switch {
		case r.TeamID != nil:
			if teams == nil {
				teams = map[uuid.UUID]bool{}
			}
			teams[*r.TeamID] = teams[*r.TeamID] || ok
		case r.Role != "":
			hasRoleRules = true
			if ok && role != "admin" {
				role = r.Role
			}
		}
	}
	if hasRoleRules && role == "" {
		role = "member"
	}
	return teams, role
}

// matches reports whether claim, or any string of it, matches expr in full;
// an expr that does not compile matches nothing.
func matches(expr string, claim any) bool {
	re, err := anchored(expr)
	if err != nil {
		return false
	}
	switch v := claim.(type) {
	case string:
		return re.MatchString(v)
	case []string:
		return slices.ContainsFunc(v, re.MatchString)
	case []any:
		return slices.ContainsFunc(v, func(e any) bool { s, ok := e.(string); return ok && re.MatchString(s) })
	}
	return false
}

func anchored(expr string) (*regexp.Regexp, error) { return regexp.Compile("^(?:" + expr + ")$") }
