package gateway

import (
	"strings"
)

// PDP decides the operations of a running session against the current
// snapshot; *Policy is one, the session hooks bind it to the session.
type PDP interface {
	Decide(r AccessRequest) Decision
	Version() string
	QueryProfile(r AccessRequest) QueryProfile
}

var _ PDP = (*Policy)(nil)

// QueryProfile tells a database guard how much checking the operations of
// one subject on one resource need.
type QueryProfile struct {
	Rules         bool // the policy may deny an operation of the subject on the resource
	TextDependent bool // the snapshot holds custom Cedar: a decision may depend on the text
}

// allClasses are the seven classes of shellclass, as a query probe.
var allClasses = []string{"destructive", "egress", "opaque", "privilege", "read", "secrets", "write"}

// QueryProfile is the profile of the query request r (Action, User,
// Resource, IP and Kind set). The built-in rules depend only on the subject,
// the resource and whether the classes contain one of a list, so a probe
// with all seven classes that is allowed means no operation is denied.
// Custom Cedar may read the text, tables or time: it always has rules.
func (p *Policy) QueryProfile(r AccessRequest) QueryProfile {
	if p.hasCustom() {
		return QueryProfile{Rules: true, TextDependent: true}
	}
	r.Classes = allClasses
	return QueryProfile{Rules: !p.Decide(r).Allow}
}

// hasCustom reports whether the snapshot holds a custom policy ("custom:*").
func (p *Policy) hasCustom() bool {
	if p == nil {
		return false
	}
	for id := range p.policies.All() {
		if strings.HasPrefix(string(id), "custom:") {
			return true
		}
	}
	return false
}
