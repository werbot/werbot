package gateway

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryProfile(t *testing.T) {
	m := newIDs("u1", "pg")
	query := func(p *Policy) AccessRequest {
		r, ok := p.Resource("pg")
		require.True(t, ok)
		return AccessRequest{User: User{ID: m["u1"]}, Resource: r, IP: netip.MustParseAddr("10.1.2.3"), Action: "postgres.query", Kind: KindStream}
	}
	for name, tc := range map[string]struct {
		policies string
		want     QueryProfile
	}{
		"binding only": {`permit (principal, action, resource);`, QueryProfile{}},
		"command rule": {`permit (principal, action, resource);
@id("command:r1")
forbid (principal, action == Werbot::Action::"postgres.query", resource) when { context.classes.containsAny(["write"]) };`,
			QueryProfile{Rules: true}},
		"approval rule": {`permit (principal, action, resource);
@id("command:r2")
@obligation("approval")
forbid (principal, action == Werbot::Action::"postgres.query", resource) when { context.classes.containsAny(["destructive"]) } unless { context.approval_granted };`,
			QueryProfile{Rules: true}},
		"no query permit": {`permit (principal, action == Werbot::Action::"postgres.connect", resource);`, QueryProfile{Rules: true}},
		"custom cedar":    {"@id(\"custom:p1:0\")\npermit (principal, action, resource);", QueryProfile{Rules: true, TextDependent: true}},
	} {
		p := compileTest(t, m, tc.policies, nil, res{"pg", "pg", "postgres"})
		assert.Equal(t, tc.want, p.QueryProfile(query(p)), name)
	}
	assert.Equal(t, QueryProfile{Rules: true}, (*Policy)(nil).QueryProfile(AccessRequest{}), "no snapshot fails closed")
}
