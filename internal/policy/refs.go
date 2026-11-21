package policy

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/policy/dbq"
)

// validTarget checks the name, subjects and selector every rule of
// subjects on resources has.
func validTarget(name string, s Subjects, sel Selector) error {
	if err := validName(name); err != nil {
		return err
	}
	if s.empty() {
		return invalid("subjects must not be empty")
	}
	return sel.validate()
}

// distinct counts the different ids.
func distinct(ids []uuid.UUID) int {
	return len(slices.Compact(slices.SortedFunc(slices.Values(ids), compareIDs)))
}

// checkRefs refuses users, teams and resources of another organization.
func checkRefs(ctx context.Context, st Store, orgID uuid.UUID, s Subjects, sel Selector) error {
	n, err := st.CountRuleRefs(ctx, dbq.CountRuleRefsParams{OrgID: orgID, ResourceIds: sel.ResourceIDs, UserIds: s.Users, TeamIds: s.Teams})
	if err != nil {
		return err
	}
	if int(n.Users) != distinct(s.Users) || int(n.Teams) != distinct(s.Teams) {
		return invalid("subject is not in organization")
	}
	if int(n.Resources) != distinct(sel.ResourceIDs) {
		return invalid("resource is not in organization")
	}
	return nil
}

// encodeTarget is the stored JSON of subjects and selector.
func encodeTarget(s Subjects, sel Selector) (subjects, selector []byte, err error) {
	if subjects, err = json.Marshal(s); err != nil {
		return nil, nil, err
	}
	selector, err = json.Marshal(sel)
	return subjects, selector, err
}

// decodeTarget reads the stored JSON of subjects and selector.
func decodeTarget(subjects, selector []byte, s *Subjects, sel *Selector) error {
	return errors.Join(json.Unmarshal(subjects, s), json.Unmarshal(selector, sel))
}
