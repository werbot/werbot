package dbproxy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/werbot/werbot/internal/gateway"
)

// maxApprovalText is the longest text an approval can carry
// (RequestApprovalRequest.command).
const maxApprovalText = 16384

// ApprovalText is what the approvers of op read and what its approval
// covers (op_sha256 is its SHA-256): the text, and for an execute with
// parameters a line with them as the querylog shows them. An approval of one
// set of parameters never covers another.
func ApprovalText(op Op) string {
	if len(op.Params) == 0 {
		return op.Text
	}
	params, _ := json.Marshal(LogParams(op.Params)) // a []string always marshals
	return op.Text + "\n-- params: " + string(params)
}

// Approve asks for the approval v obliges op to and waits for it: notify
// tells the client the ticket before a wait (an approval granted before is
// not waited for). Once approved, op is decided again with the approval: a
// rule that came meanwhile may still deny. A denied or expired approval, or
// a text the approvers cannot be shown whole, denies with its message; a gateway
// that drains or a client that leaves is the error. An approved statement
// covers its executes with the same parameters.
func (g *Guard) Approve(ctx context.Context, op Op, v Verdict, notify func(id, url string, timeout time.Duration) error) (Verdict, error) {
	text := ApprovalText(op)
	if len(text) > maxApprovalText || cutParams(op.Params) {
		return refused(v, "query too long to approve (16384 bytes)"), nil
	}
	st, ok := g.approvable(op)
	if !ok {
		return refused(v, denyMessage("", v.Classes)), nil
	}
	err := g.h.Approve(ctx, gateway.ApprovalInput{Command: text, Classes: v.Classes, Reasons: v.PolicyIDs},
		func(id, url string, timeout time.Duration) error {
			v.ApprovalID, v.ApprovalURL = id, url
			if timeout == 0 {
				return nil
			}
			return notify(id, url, timeout)
		})
	switch {
	case errors.Is(err, gateway.ErrApprovalDenied):
		return refused(v, "approval denied"), nil
	case errors.Is(err, gateway.ErrApprovalTimeout):
		return refused(v, "approval timed out"), nil
	case err != nil:
		return v, err
	}
	return g.approved(op, st, text, v), nil
}

// cutParams reports parameters LogParams cut: their display is not the whole
// value, so no approval may cover them.
func cutParams(ps []string) bool { return len(ps) > 0 && ps[len(ps)-1] == Truncated }

// approvable is the statement of a parse or execute op, which must be the
// parsed text; a query has none.
func (g *Guard) approvable(op Op) (*statement, bool) {
	if op.Kind != "parse" && op.Kind != "execute" {
		return nil, true
	}
	st := g.stmts[op.Statement]
	return st, st != nil && st.sha == g.sha(op)
}

// approved decides op again with its approval granted and, once allowed,
// remembers the approved text of its statement.
func (g *Guard) approved(op Op, st *statement, text string, v Verdict) Verdict {
	d := g.h.PDP.Decide(g.request(op.DB, op.Text, v.Classes, v.Tables, v.WriteTables, true))
	if !d.Allow {
		g.denied(op.Text, v.Classes, d.Reasons)
		v.PolicyIDs = d.Reasons
		return refused(v, denyMessage(d.Message, v.Classes))
	}
	if st != nil {
		st.approved, st.hasApproval = sha256.Sum256([]byte(text)), true
	}
	v.Allow, v.Obligations, v.Message, v.PolicyIDs = true, nil, "", d.Reasons
	return v
}

// refused is v denied with message.
func refused(v Verdict, message string) Verdict {
	v.Allow, v.Obligations, v.Message = false, nil, message
	return v
}
