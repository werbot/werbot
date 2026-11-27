package dbproxy

import (
	"crypto/sha256"
	"strconv"
	"strings"
	"time"

	"github.com/werbot/werbot/internal/gateway"
)

// Bounds of the per-session state of a Guard.
const (
	decisionCacheSize = 1024
	decisionTTL       = 60 * time.Second // time-bound rules (hour, weekday, mfa_age) are seen within it
	maxStatements     = 10000            // the connector keeps at most as many live statements
)

// state is what Check and Approve keep for one session.
type state struct {
	version   string // of the PDP snapshot the caches belong to
	profile   gateway.QueryProfile
	decisions map[decisionKey]cachedDecision
	stmts     map[string]*statement
	queries   int    // operations of an agent counted toward MaxQueries
	hashBuf   []byte // of sum
}

type decisionKey struct{ classes, db string }

type cachedDecision struct {
	d  gateway.Decision
	at time.Time
}

// statement is a prepared statement: the SHA-256 of its text, its
// classification once known and the SHA-256 of its last approved text.
type statement struct {
	sha, approved [32]byte
	hasApproval   bool
	c             *classified
}

func (s *state) start(g *Guard) {
	s.decisions, s.stmts = map[decisionKey]cachedDecision{}, map[string]*statement{}
	if g.h.PDP != nil {
		s.version, s.profile = g.h.PDP.Version(), g.h.PDP.QueryProfile(g.probe())
	}
}

// probe is the query request the profile is asked for.
func (g *Guard) probe() gateway.AccessRequest {
	return gateway.AccessRequest{Action: g.action, Kind: gateway.KindStream}
}

func (g *Guard) check(op Op, now time.Time) Verdict {
	if g.h.PDP == nil {
		return Verdict{Allow: true}
	}
	if g.overLimit(op.Kind) {
		return Verdict{Message: "query limit " + strconv.Itoa(g.h.Limits.MaxQueries) + " reached"}
	}
	g.refresh()
	var st *statement
	if op.Kind == "parse" {
		st = g.parsed(op)
	}
	if g.h.Limits == nil && !g.profile.Rules {
		return Verdict{Allow: true}
	}
	c, granted := g.operation(op, st)
	return g.verdict(op.Text, c, g.decide(op.DB, op.Text, c, granted, now))
}

// overLimit counts an agent's query, execute and request and reports one past MaxQueries.
func (g *Guard) overLimit(kind string) bool {
	if g.h.Limits == nil || (kind != "query" && kind != "execute" && kind != "request") {
		return false
	}
	g.queries++
	return g.queries > g.h.Limits.MaxQueries
}

// refresh drops the decisions and the profile of an older snapshot.
func (g *Guard) refresh() {
	if v := g.h.PDP.Version(); v != g.version {
		g.version, g.profile = v, g.h.PDP.QueryProfile(g.probe())
		clear(g.decisions)
	}
}

// parsed keeps a new prepared statement.
// ponytail: past maxStatements an arbitrary statement is forgotten (its
// executes become opaque); the connector bounds the live ones, the Guard does
// not see Close.
func (g *Guard) parsed(op Op) *statement {
	if _, ok := g.stmts[op.Statement]; !ok && len(g.stmts) >= maxStatements {
		for name := range g.stmts {
			delete(g.stmts, name)
			break
		}
	}
	st := &statement{sha: g.sha(op)}
	g.stmts[op.Statement] = st
	return st
}

// operation classifies op; granted reports an execute whose parameters an
// approval of its statement covers. An Opaque op is [opaque], never granted.
func (g *Guard) operation(op Op, parsed *statement) (c *classified, granted bool) {
	if op.Opaque {
		return opaque, false
	}
	switch op.Kind {
	case "query", "request":
		return g.classify(op.Text, g.sha(op)), false
	case "parse":
		return parsed.remember(g.classify(op.Text, parsed.sha)), false
	case "execute":
		st := g.stmts[op.Statement]
		if st == nil {
			return opaque, false
		}
		return g.executed(st, op), st.hasApproval && st.approved == sha256.Sum256([]byte(ApprovalText(op)))
	}
	return opaque, false // a function call, or a kind that carries no text to classify
}

// executed is the classification of an execute of st: its text must be the
// parsed one (a cut text is opaque).
func (g *Guard) executed(st *statement, op Op) *classified {
	if st.c != nil {
		return st.c
	}
	if g.sum(op.Text) != st.sha {
		return opaque
	}
	return st.remember(g.classify(op.Text, st.sha))
}

// remember keeps a deterministic classification of st and returns c.
func (st *statement) remember(c *classified) *classified {
	if !c.res.Transient {
		st.c = c
	}
	return c
}

// sha is op.SHA256, else the SHA-256 of op.Text.
func (g *Guard) sha(op Op) [32]byte {
	if op.SHA256 != ([32]byte{}) {
		return op.SHA256
	}
	return g.sum(op.Text)
}

// hashBufMax is the longest text sum hashes through the Guard's buffer.
const hashBufMax = 64 << 10

// sum is the SHA-256 of s; up to hashBufMax it copies s into a buffer it
// keeps, so a repeated text does not allocate.
func (g *Guard) sum(s string) [32]byte {
	if len(s) > hashBufMax {
		return sha256.Sum256([]byte(s))
	}
	g.hashBuf = append(g.hashBuf[:0], s...)
	return sha256.Sum256(g.hashBuf)
}

// decide asks the PDP about the operation, through the decision cache unless
// the snapshot has custom Cedar, the classes are transient or an approval is
// granted.
func (g *Guard) decide(db, text string, c *classified, granted bool, now time.Time) gateway.Decision {
	cached := !granted && !g.profile.TextDependent && !c.res.Transient
	k := decisionKey{c.key, db}
	if e, ok := g.decisions[k]; cached && ok && now.Sub(e.at) < decisionTTL {
		return e.d
	}
	d := g.h.PDP.Decide(g.request(db, text, c.classes, c.tables, c.writes, granted))
	if cached {
		if len(g.decisions) >= decisionCacheSize {
			clear(g.decisions) // ponytail: map instead of an LRU; an LRU if misses show
		}
		g.decisions[k] = cachedDecision{d: d, at: now}
	}
	return d
}

func (g *Guard) request(db, text string, classes, tables, writes []string, granted bool) gateway.AccessRequest {
	return gateway.AccessRequest{Action: g.action, Kind: gateway.KindStream, Command: text, Classes: classes,
		Tables: tables, WriteTables: writes, DBName: db, ApprovalGranted: granted}
}

// verdict is the Verdict of decision d of an operation with text and c; a
// deny that no approval lifts is reported.
func (g *Guard) verdict(text string, c *classified, d gateway.Decision) Verdict {
	v := Verdict{Allow: d.Allow, Classes: c.classes, Tables: c.tables, WriteTables: c.writes, PolicyIDs: d.Reasons}
	switch {
	case d.Allow:
	case len(d.Obligations) == 1 && d.Obligations[0] == gateway.ObligationApproval:
		v.Obligations, v.Message = d.Obligations, d.Message
	default:
		v.Message = denyMessage(d.Message, c.classes)
		g.denied(text, c.classes, d.Reasons)
	}
	return v
}

// denyMessage is the @message of a deny, else "not allowed (<classes>)".
func denyMessage(message string, classes []string) string {
	if message != "" {
		return message
	}
	return "not allowed (" + strings.Join(classes, ", ") + ")"
}

func (g *Guard) denied(text string, classes, reasons []string) {
	if g.h.RecordDenial != nil {
		g.h.RecordDenial(gateway.ApprovalInput{Command: text, Classes: classes, Reasons: reasons})
	}
}
