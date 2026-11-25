package policy

import (
	"slices"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
)

// Obligations a deny can carry.
const (
	ObligationApproval = "approval" // an approval of the command
	ObligationConsole  = "console"  // the shell is served by the command console
	ObligationMFA      = "mfa"      // a fresh second factor of the user (mfa_rule)
)

// obligations are the known @obligation names and how a request meets each;
// the gateway mirrors them in internal/gateway/authz_exec.go.
var obligations = map[string]func(*Request){
	ObligationApproval: func(r *Request) { r.ApprovalGranted = true },
	ObligationConsole:  func(r *Request) { r.Console = true },
	ObligationMFA:      func(r *Request) { r.MFAAgeSeconds, r.MFAMethod = 0, "webauthn" },
}

// explain returns the @message of the first deciding forbid by ascending
// @id and the obligations of a deny decided by forbids. Obligations are set
// only when every deciding forbid carries a known @obligation and r with
// them met is allowed for principal p, the principal of the pass (a deny
// without a permit asks for nothing).
// The gateway PDP mirrors this logic in internal/gateway/authz_exec.go:
// change both together.
func (ps *PolicySet) explain(r Request, p cedar.EntityUID, forbids []string) (string, []string) {
	ids := slices.Sorted(slices.Values(forbids))
	var message string
	var names []string
	for _, id := range ids {
		a := ps.policies.Get(cedar.PolicyID(id)).Annotations()
		if message == "" {
			message = string(a[types.AnnotationKey("message")])
		}
		names = append(names, string(a[types.AnnotationKey("obligation")]))
	}
	return message, ps.meet(r, p, names)
}

// meet returns names, sorted and unique, when each is known and r with all
// of them met is allowed for principal p; nil otherwise.
func (ps *PolicySet) meet(r Request, p cedar.EntityUID, names []string) []string {
	if len(names) == 0 {
		return nil
	}
	for _, n := range names {
		fulfil, ok := obligations[n]
		if !ok {
			return nil
		}
		fulfil(&r)
	}
	d, diag := ps.evaluate(r, p)
	if d != cedar.Allow || len(diag.Errors) > 0 {
		return nil
	}
	return slices.Compact(slices.Sorted(slices.Values(names)))
}

// MFAStrength is the factor that meets the mfa obligation of r: "any" when
// a TOTP code confirmed now would allow r, "webauthn" otherwise.
// The gateway PDP mirrors it in internal/gateway/authz_exec.go: change both together.
func MFAStrength(ps *PolicySet, r Request) string {
	r.MFAAgeSeconds, r.MFAMethod = 0, "totp"
	if ps.Authorize(r).Allow {
		return "any"
	}
	return "webauthn"
}

// reachable reports whether d allows, or denies only until the user confirms
// a second factor; an agent's decision never carries mfa (mergeAgent).
func reachable(d Decision) bool {
	return d.Allow || slices.Equal(d.Obligations, []string{ObligationMFA})
}
