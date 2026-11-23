package policy_test

import (
	"fmt"
	"net/netip"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/resolved"
	"github.com/cedar-policy/cedar-go/x/exp/schema/validate"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/ssh"
	"github.com/werbot/werbot/internal/policy"
)

func TestSchemaHasConnectorActions(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(policy.Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	a, ok := res.Actions[cedar.NewEntityUID("Werbot::Action", "ssh.connect")]
	require.True(t, ok)
	require.NotNil(t, a.AppliesTo)
	assert.Equal(t, resolved.LongType{}, a.AppliesTo.Context["port"].Type)
}

// TestCompiledPoliciesTypecheck validates every compiled policy, the port
// condition included, against the schema.
func TestCompiledPoliciesTypecheck(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(policy.Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	v := validate.New(res)

	r := uuid.New()
	_, text, err := policy.Compile(policy.Snapshot{
		Resources: []policy.SnapshotResource{{ID: r, Name: "db", Type: "ssh"}},
		Bindings: []policy.SnapshotBinding{{ID: uuid.New(), SubjectType: "team", SubjectID: uuid.New(), ResourceIDs: []uuid.UUID{r},
			Ports: []connector.PortRange{{Port: 5432}, {Port: 8000, EndPort: 8100}}}},
		Rules: []policy.ResourceRules{{ResourceID: r,
			Networks:  []policy.NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: policy.EffectAllow}},
			Countries: []policy.CountryRule{{Country: "NL", Effect: policy.EffectAllow}},
			Schedule:  []policy.Slot{{Weekday: 1, Hour: 9}}}},
	})
	require.NoError(t, err)
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	require.Len(t, list, 11) // binding, three rules, blocklist, six agent baseline policies
	for i, p := range list {
		assert.NoError(t, v.Policy(fmt.Sprint(i), (*ast.Policy)(p.AST())), i)
	}
}

// TestSchemaHasNowAndCLISession validates the compiled grants and every
// compiled lock target against the schema.
func TestSchemaHasNowAndCLISession(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(policy.Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	ctx := res.Actions[cedar.NewEntityUID("Werbot::Action", "ssh.connect")].AppliesTo.Context
	assert.Equal(t, resolved.LongType{}, ctx["now"].Type)
	assert.Equal(t, resolved.StringType{}, ctx["cli_session_id"].Type)

	r, g, target := uuid.New(), uuid.New(), uuid.New()
	snap := policy.Snapshot{
		Resources: []policy.SnapshotResource{{ID: r, Name: "db", Type: "ssh"}},
		Grants:    []policy.SnapshotGrant{{ID: g, UserID: uuid.New(), ResourceID: r, ExpiresUnix: 2000}},
		Locks:     []policy.SnapshotLock{{ID: uuid.New(), TargetType: "grant", TargetID: &g, ExpiresUnix: 1000}},
	}
	for _, typ := range []string{"user", "team", "resource", "cli_session"} {
		snap.Locks = append(snap.Locks, policy.SnapshotLock{ID: uuid.New(), TargetType: typ, TargetID: &target})
	}
	_, text, err := policy.Compile(snap)
	require.NoError(t, err)
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	require.Len(t, list, 12) // grant, four locks, blocklist, six agent baseline policies
	v := validate.New(res)
	for i, p := range list {
		assert.NoError(t, v.Policy(fmt.Sprint(i), (*ast.Policy)(p.AST())), i)
	}
}

// TestSchemaHasExecContext checks the exec action and context and validates
// compiled command rules against the schema.
func TestSchemaHasExecContext(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(policy.Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	a, ok := res.Actions[cedar.NewEntityUID("Werbot::Action", "ssh.exec")]
	require.True(t, ok)
	ctx := a.AppliesTo.Context
	assert.Equal(t, resolved.StringType{}, ctx["command"].Type)
	assert.Equal(t, resolved.SetType{Element: resolved.StringType{}}, ctx["classes"].Type)
	assert.Equal(t, resolved.BoolType{}, ctx["approval_granted"].Type)

	r := uuid.New()
	rule := func(effect string, subj policy.Subjects) policy.SnapshotCommandRule {
		return policy.SnapshotCommandRule{ID: uuid.New(), Subjects: subj, ResourceIDs: []uuid.UUID{r},
			Classes: []string{"write"}, Effect: effect, Message: "m"}
	}
	_, text, err := policy.Compile(policy.Snapshot{
		Resources: []policy.SnapshotResource{{ID: r, Name: "db", Type: "ssh"}},
		CommandRules: []policy.SnapshotCommandRule{
			rule("deny", policy.Subjects{AllUsers: true}),
			rule("approval", policy.Subjects{Users: []uuid.UUID{uuid.New()}, Teams: []uuid.UUID{uuid.New()}}),
		},
	})
	require.NoError(t, err)
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	require.Len(t, list, 10) // two rules, the agent permit of the approval rule, blocklist, six agent baseline policies
	v := validate.New(res)
	for i, p := range list {
		assert.NoError(t, v.Policy(fmt.Sprint(i), (*ast.Policy)(p.AST())), i)
	}
}

func TestSchemaHasPathContext(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(policy.Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	_, ok := res.Actions[cedar.NewEntityUID("Werbot::Action", "ssh.shell")]
	assert.True(t, ok)
	ctx := res.Actions[cedar.NewEntityUID("Werbot::Action", "ssh.exec")].AppliesTo.Context
	for _, key := range []string{"paths", "path_hits", "path_outside"} {
		assert.Equal(t, resolved.SetType{Element: resolved.StringType{}}, ctx[cedar.String(key)].Type, key)
	}
	assert.Equal(t, resolved.BoolType{}, ctx["console"].Type)
}
