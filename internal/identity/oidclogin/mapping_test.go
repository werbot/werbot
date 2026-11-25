package oidclogin

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestApplyMapping(t *testing.T) {
	ops, dev, sec := uuid.New(), uuid.New(), uuid.New()
	claims := map[string]any{"groups": []any{"ops", "dev"}, "dept": "eng"}
	teamRules := []MappingRule{
		{Claim: "groups", Regex: "ops", TeamID: &ops},
		{Claim: "groups", Regex: "op", TeamID: &dev}, // anchored: "op" does not match "ops"
		{Claim: "dept", Regex: "sec|security", TeamID: &sec},
	}

	teams, role := ApplyMapping(teamRules, claims)
	assert.Equal(t, map[uuid.UUID]bool{ops: true, dev: false, sec: false}, teams)
	assert.Empty(t, role, "no role rules: the role stays")

	roles := []MappingRule{{Claim: "groups", Regex: "dev", Role: "member"}, {Claim: "groups", Regex: "o.s", Role: "admin"}}
	_, role = ApplyMapping(roles, claims)
	assert.Equal(t, "admin", role, "the highest matching role")

	_, role = ApplyMapping(roles, map[string]any{"groups": "sales"})
	assert.Equal(t, "member", role, "role rules without a match give member")

	_, role = ApplyMapping([]MappingRule{{Claim: "groups", Regex: "(", Role: "admin"}}, claims)
	assert.Equal(t, "member", role, "a regex that does not compile matches nothing")

	_, role = ApplyMapping([]MappingRule{{Claim: "groups", Regex: ".*", Role: "admin"}}, map[string]any{"groups": 7.0})
	assert.Equal(t, "member", role, "only strings and lists of strings match")
}
