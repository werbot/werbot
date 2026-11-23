package policy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

// Bounds of an MFA rule's max_age, in seconds.
const (
	minMFAAge = 30
	maxMFAAge = 24 * 60 * 60
)

// mfaStrengths are the strengths of an MFA rule: any factor or a passkey only.
var mfaStrengths = []string{"any", "webauthn"}

// SnapshotMFARule is an MFA rule with its selector expanded.
type SnapshotMFARule struct {
	ID            uuid.UUID   `json:"id"`
	Subjects      Subjects    `json:"subjects"`
	ResourceIDs   []uuid.UUID `json:"resource_ids"`
	MaxAgeSeconds int64       `json:"max_age_seconds"`
	Strength      string      `json:"strength"` // any | webauthn
}

// validMFA checks the max age and strength of an MFA rule.
func validMFA(maxAgeSeconds int64, strength string) error {
	if maxAgeSeconds < minMFAAge || maxAgeSeconds > maxMFAAge {
		return invalid("max_age must be 30s to 24h")
	}
	if !slices.Contains(mfaStrengths, strength) {
		return invalid("strength must be one of %s", strings.Join(mfaStrengths, ", "))
	}
	return nil
}

// writeMFARules emits one mfa obligation forbid per rule, sorted by ID, on
// the connect actions of the types of its resources: it holds unless the
// user's factor is younger than the max age and, for strength webauthn, a
// passkey. Only people are subjects: an agent is denied through its
// sponsor's pass. Rules without people, resources or connect actions emit
// nothing; an invalid max age or strength fails the compile.
func writeMFARules(b *strings.Builder, rules []SnapshotMFARule, resources []SnapshotResource) error {
	for _, r := range slices.SortedFunc(slices.Values(rules), func(x, y SnapshotMFARule) int { return compareIDs(x.ID, y.ID) }) {
		if err := validMFA(r.MaxAgeSeconds, r.Strength); err != nil {
			return fmt.Errorf("policy: mfa rule %s: %w", r.ID, err)
		}
		if t := target(subjectCondition(r.Subjects), r.ResourceIDs); t != "" {
			writeForbid(b, forbid{id: "mfa:" + r.ID.String(), obligation: ObligationMFA, actions: connectActions(resources, r.ResourceIDs), when: t,
				unless: fmt.Sprintf(`context.mfa_age < %d && (%t || context.mfa_method == "webauthn")`, r.MaxAgeSeconds, r.Strength == "any")})
		}
	}
	return nil
}

// connectActions are the "<type>.connect" actions of the registered types of
// the resources ids picks, ascending and unique.
func connectActions(resources []SnapshotResource, ids []uuid.UUID) []string {
	var out []string
	for _, r := range resources {
		t, ok := connector.Lookup(r.Type)
		a := r.Type + ".connect"
		if ok && slices.Contains(ids, r.ID) && slices.Contains(t.Actions, a) {
			out = append(out, fmt.Sprintf("Werbot::Action::%q", a))
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(out)))
}
