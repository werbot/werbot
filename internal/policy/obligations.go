package policy

import (
	"slices"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
)

// ObligationApproval asks for an approval of the command.
const ObligationApproval = "approval"

// obligations are the known @obligation names and how a request meets each.
var obligations = map[string]func(*Request){
	ObligationApproval: func(r *Request) { r.ApprovalGranted = true },
}

// explain returns the @message of the first deciding forbid by ascending
// @id and the obligations of a deny decided by forbids. Obligations are set
// only when every deciding forbid carries a known @obligation and r with
// them met is allowed (a deny without a permit asks for nothing).
// The gateway PDP mirrors this logic in internal/gateway/authz_exec.go:
// change both together.
func (ps *PolicySet) explain(r Request, forbids []string) (string, []string) {
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
	return message, ps.meet(r, names)
}

// meet returns names, sorted and unique, when each is known and r with all
// of them met is allowed; nil otherwise.
func (ps *PolicySet) meet(r Request, names []string) []string {
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
	d, diag := ps.evaluate(r)
	if d != cedar.Allow || len(diag.Errors) > 0 {
		return nil
	}
	return slices.Compact(slices.Sorted(slices.Values(names)))
}
