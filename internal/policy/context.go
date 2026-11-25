package policy

import (
	"math"
	"slices"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

// NoMFA is context.mfa_age of a request without a second factor: no rule's
// max_age is met.
const NoMFA int64 = math.MaxInt64

// mfaAge is r as context.mfa_age: NoMFA without a method or with a negative
// age (fail-closed), whatever MFAAgeSeconds says.
// The gateway PDP mirrors it in internal/gateway/authz_exec.go: change both together.
func mfaAge(r Request) int64 {
	if r.MFAMethod == "" || r.MFAAgeSeconds < 0 {
		return NoMFA
	}
	return r.MFAAgeSeconds
}

// cliSession is id as context.cli_session_id: uuid.Nil (no CLI session) is "",
// which no cli_session lock matches.
func cliSession(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

// typeHasAction reports whether action is one of the actions of the registered
// type of resource id; a resource outside the snapshot has none.
func (ps *PolicySet) typeHasAction(id uuid.UUID, action string) bool {
	e, ok := ps.entities[cedar.NewEntityUID("Werbot::Resource", cedar.String(id.String()))]
	if !ok {
		return false
	}
	v, _ := e.Attributes.Get("type")
	typ, _ := v.(cedar.String)
	t, ok := connector.Lookup(string(typ))
	return ok && slices.Contains(t.Actions, action)
}
