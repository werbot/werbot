package pgconnector

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

var errNoAgentUsername = errors.New("postgres: agent_username is not set")

// agentParams are the client start parameters an agent session keeps.
var agentParams = []string{"application_name", "client_encoding", "DateStyle", "TimeZone", "IntervalStyle", "extra_float_digits"}

// writeParams are the parameters of an approved write's connection: the
// allowlisted client parameters, the agent's statement_timeout in
// milliseconds and the lexing the classifier assumes.
func writeParams(client map[string]string, l gateway.AgentLimits) map[string]string {
	out := make(map[string]string, len(agentParams)+3)
	for _, k := range agentParams {
		if v, ok := client[k]; ok {
			out[k] = v
		}
	}
	out["statement_timeout"] = strconv.FormatInt(l.StatementTimeout.Milliseconds(), 10)
	out["standard_conforming_strings"] = "on"
	return out
}

// login connects session s to database db of cfg with the session
// credential: a person as the resource's username with the client's
// parameters, an agent (limits set) as its agent_username, read-only.
func login(ctx context.Context, s *gateway.Session, cfg *postgresv1.Config, client map[string]string, db string, l *gateway.AgentLimits) (*pgconn.HijackedConn, *pgconn.PgConn, error) {
	user, params := cfg.GetUsername(), runtimeParams(client)
	if l != nil {
		if cfg.GetAgentUsername() == "" {
			return nil, nil, errNoAgentUsername
		}
		user, params = cfg.GetAgentUsername(), writeParams(client, *l)
		params["default_transaction_read_only"] = "on"
	}
	pc, err := dialConfig(cfg, s.Secret, user, params, db, s.Dial)
	if err != nil {
		return nil, nil, err
	}
	return connect(ctx, pc)
}

// agentWriter runs the approved writes of an agent session on connections
// of their own, as the resource's username with the primary credential,
// within the limits and the masking of the session's hooks h.
type agentWriter struct {
	h      gateway.SessionHooks
	cfg    *postgresv1.Config
	params map[string]string
	db     string
	dial   pgconn.DialFunc
	name   string
}

// agentOf is the writer of an agent session of h; nil for a person.
func agentOf(s *gateway.Session, h gateway.SessionHooks, cfg *postgresv1.Config, client map[string]string, db string) *agentWriter {
	if h.Limits == nil {
		return nil
	}
	return &agentWriter{h: h, cfg: cfg, params: writeParams(client, *h.Limits), db: db, dial: s.Dial, name: s.Resource.Name}
}

// connect logs in for one approved write with secret; the PgConn sends its
// cancel requests.
func (a *agentWriter) connect(ctx context.Context, secret gateway.Secret) (*pgconn.HijackedConn, *pgconn.PgConn, error) {
	pc, err := dialConfig(a.cfg, secret, a.cfg.GetUsername(), a.params, a.db, a.dial)
	if err != nil {
		return nil, nil, err
	}
	return connect(ctx, pc)
}

// cycle keeps an agent session unpipelined: the first message of a query,
// a function call or an extended batch goes on only once the database
// answered everything before it. A person's waits so only after an
// operation that may change the lexing settings or the prepared statements,
// so its text is classified under the settings the database reports and its
// names are the ones the database holds. A Sync ends the client's cycle.
// ponytail: a query sent inside an unsynced batch is not held back; the
// batch has no ReadyForQuery to wait for.
func (c *clientSide) cycle(typ byte) error {
	if strings.IndexByte("PBEDCHSQF", typ) < 0 { // COPY data, Terminate
		return nil
	}
	if !c.inCycle && (c.agent != nil || c.relex) {
		if _, err := c.waitIdle(); err != nil {
			return err
		}
		c.relex = false
	}
	c.syncNames()
	if typ == 'S' {
		c.cycles++
	}
	c.inCycle = typ != 'S' && typ != 'Q' && typ != 'F'
	return nil
}

// waitIdle sends the database the messages buffered for it and waits until
// it answered all of them; it returns the transaction status the client got
// last.
func (c *clientSide) waitIdle() (byte, error) {
	if err := c.w.Flush(); err != nil {
		return 0, err
	}
	if err := c.q.waitIdle(c.ctx); err != nil {
		return 0, err
	}
	c.out.mu.Lock()
	defer c.out.mu.Unlock()
	return c.out.status, nil
}

// holdAgent handles an agent's operation that needs an approval: a query,
// or a Parse with the rest of its batch, that runs one statement outside a
// transaction is approved and run as an approved write; anything else is
// denied without an approval.
func (c *clientSide) holdAgent(h *held, body []byte) error {
	frames := []frame{{h.kind, body}}
	op, ok := h.op, h.kind == 'Q'
	if h.kind == 'P' {
		c.parseRefused() // the session's database never gets it
		rest, err := c.collect()
		if err != nil {
			return err
		}
		frames = append(frames, rest...)
		op, ok = batchOp(h.op, rest)
		ok = ok && oneStatement(rest)
	}
	v, err := c.agentApproval(h.v, op, ok)
	if err == nil && v.Allow {
		v, err = c.runApprovedWrite(frames, op, v)
	}
	if err != nil || v.Allow {
		return err
	}
	if err := c.deny(h.kind, h.op, v); err != nil {
		return err
	}
	return c.replay(frames[1:])
}

// oneStatement reports whether the frames after a held Parse run its
// statement once: one Bind, one Execute, Describes, and the Sync last.
func oneStatement(frames []frame) bool {
	var n [256]int
	for _, f := range frames {
		n[f.typ]++
	}
	return frames[len(frames)-1].typ == 'S' && n['B'] == 1 && n['E'] == 1 && n['S'] == 1 && n['B']+n['E']+n['S']+n['D'] == len(frames)
}

// agentApproval asks for the approval of an agent's write op once the
// session is idle outside a transaction and op is one statement (ok).
func (c *clientSide) agentApproval(v dbproxy.Verdict, op dbproxy.Op, ok bool) (dbproxy.Verdict, error) {
	status, err := c.waitIdle()
	switch {
	case err != nil:
		return v, err
	case status != 'I':
		return dbproxy.Refused(v, "approved writes run outside a transaction"), nil
	case !ok || sqlDialect.Classify(op.Text, sqlclass.Options{}).Statements != 1:
		return dbproxy.Refused(v, "approved writes run one statement at a time"), nil
	}
	return c.approve(op, v)
}
