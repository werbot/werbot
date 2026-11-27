package gateway

import (
	"encoding/json"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// roPolicies permit everything; an agent reaches "db1" only while its
// agent_readonly credential is verified (the stage 18 db_readonly baseline).
const roPolicies = `permit (principal, action, resource);
@id("agent:baseline:db_readonly")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"rocheck.connect", Werbot::Action::"rocheck.query"], resource)
unless { resource has agent_readonly && resource.agent_readonly };`

// roPolicy is the snapshot of version v with "db1" (rocheck on 10.0.1.1),
// whose entity has agent_readonly verified, and the active agent a1 of u1.
func roPolicy(t *testing.T, m ids, v string, verified bool) *Policy {
	t.Helper()
	s := testSnapshot(t, m, roPolicies, nil, res{"rdb", "db1", "rocheck"})
	s.Version, s.Resources[0].ConfigJson = v, []byte(`"10.0.1.1"`)
	var ents cedar.EntityMap
	require.NoError(t, json.Unmarshal(s.GetEntitiesJson(), &ents))
	uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(m["rdb"].String()))
	e := ents[uid]
	e.Attributes = cedar.NewRecord(cedar.RecordMap{"name": cedar.String("db1"), "type": cedar.String("rocheck"), "agent_readonly": cedar.Boolean(verified)})
	ents[uid] = e
	b, err := json.Marshal(ents)
	require.NoError(t, err)
	s.EntitiesJson = b
	withAgents(t, s, m, testAgent{"a1", "u1", "active"})
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// A snapshot in which the server stored has_write (agent_readonly false)
// ends the open session of an agent within 1s with access_revoked; a
// person's session on the same database stays.
func TestAgentSessionClosedOnHasWrite(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.m["rdb"], e.m["a1"], e.m["run1"] = uuid.New(), uuid.New(), uuid.New()
		e.f.setPolicy(roPolicy(t, e.m, "v1", true))
		startSweeper(t, e.p)
		open := func() *Session {
			r := e.req("db1", KindStream)
			r.Target = "10.0.1.1:5432"
			s, err := e.p.Open(t.Context(), r)
			require.NoError(t, err)
			e.runRO(t, s)
			return s
		}
		person := open()
		e.f.with(func(f *fakeControl) { f.user.Agent = &Agent{ID: e.m["a1"], RunID: e.m["run1"], AuthAt: time.Now()} })
		agent := open()

		e.f.setPolicy(roPolicy(t, e.m, "v2", false))
		time.Sleep(time.Second)
		require.Equal(t, []string{"access_revoked"}, e.endedWith(agent.ID), "the agent's session ends within a second")
		require.Empty(t, e.endedWith(person.ID), "a person's session stays")
	})
}

// runRO runs s on a rocheck stream connector that blocks until the session ends.
func (e *pipeEnv) runRO(t *testing.T, s *Session) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = e.p.Run(t.Context(), s, funcConn{typ: "rocheck", kinds: []Kind{KindStream}, serve: block})
	}()
	t.Cleanup(func() { <-done })
}
