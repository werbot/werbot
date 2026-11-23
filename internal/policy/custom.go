package policy

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/cedar-policy/cedar-go"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/validate"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

// maxCustomPolicyBytes is the size limit of a custom Cedar text.
const maxCustomPolicyBytes = 65536

// Trial request fixtures: user trialUser in team trialTeam, a resource
// without labels, Monday 10:00 UTC.
var (
	trialUser     = uuid.MustParse("00000000-0000-7000-8000-000000000001")
	trialTeam     = uuid.MustParse("00000000-0000-7000-8000-000000000002")
	trialResource = uuid.MustParse("00000000-0000-7000-8000-000000000003")
	trialTime     = time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
)

// ValidateCustom checks a custom Cedar text before it is stored: size,
// syntax, at least one policy, annotations, the schema and a trial
// evaluation of each policy. The first error is returned with its position.
func ValidateCustom(cedarText string) error {
	if len(cedarText) > maxCustomPolicyBytes {
		return errors.New("custom policy is too large")
	}
	list, err := cedar.NewPolicyListFromBytes("custom", []byte(cedarText))
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return errors.New("no policies")
	}
	v, err := validator()
	if err != nil {
		return err
	}
	for _, p := range list {
		if err := validatePolicy(v, p); err != nil {
			return err
		}
	}
	return nil
}

// validator checks policies against Schema() in strict mode.
func validator() (*validate.Validator, error) {
	var s schema.Schema
	if err := s.UnmarshalCedar([]byte(Schema())); err != nil {
		return nil, fmt.Errorf("policy: schema: %w", err)
	}
	rs, err := s.Resolve()
	if err != nil {
		return nil, fmt.Errorf("policy: schema: %w", err)
	}
	return validate.New(rs), nil
}

func validatePolicy(v *validate.Validator, p *cedar.Policy) error {
	pos := p.Position()
	if err := checkAnnotations(p); err != nil {
		return fmt.Errorf("%d:%d: %w", pos.Line, pos.Column, err)
	}
	if err := v.Policy("custom", (*xast.Policy)(p.AST())); err != nil {
		return fmt.Errorf("%d:%d: %w", pos.Line, pos.Column, err)
	}
	if msg := trialError(p); msg != "" {
		return fmt.Errorf("evaluation error at line %d: %s", pos.Line, msg)
	}
	return nil
}

// checkAnnotations rejects @id (Compile assigns it), a @message that a command
// rule could not carry and an @obligation that is not on a forbid or is not a
// known name.
func checkAnnotations(p *cedar.Policy) error {
	a := p.Annotations()
	if _, ok := a["id"]; ok {
		return errors.New("@id is reserved")
	}
	if m, ok := a["message"]; ok {
		if err := validMessage(string(m)); err != nil {
			return err
		}
	}
	name, ok := a["obligation"]
	if !ok {
		return nil
	}
	if p.Effect() != cedar.Forbid {
		return errors.New("@obligation is allowed only on forbid")
	}
	if _, known := obligations[string(name)]; !known {
		return fmt.Errorf("unknown obligation %q", string(name))
	}
	return nil
}

// trialError evaluates p for every action of Schema() on the trial fixtures
// with an exec and path context and returns the first evaluation error; ""
// if none.
func trialError(p *cedar.Policy) string {
	set := cedar.NewPolicySet()
	set.Add("custom", p)
	for _, t := range connector.Types() {
		ps := &PolicySet{policies: set, entities: resourceEntities([]SnapshotResource{{ID: trialResource, Type: t.Name}})}
		for _, a := range t.Actions {
			_, diag := ps.evaluate(Request{UserID: trialUser, TeamIDs: []uuid.UUID{trialTeam}, ResourceID: trialResource,
				Action: a, IP: netip.MustParseAddr("192.0.2.1"), Time: trialTime, Command: "ls", Classes: []string{"read"}, Paths: []string{"/etc/hosts"}},
				userUID(trialUser))
			if len(diag.Errors) > 0 {
				return diag.Errors[0].Message
			}
		}
	}
	return ""
}

// writeCustomPolicies emits the custom policies by ID, the policies of each
// text in order, each with @id("custom:<policy_id>:<n>"). A text that does
// not parse or carries @id fails the whole compile (fail-closed).
func writeCustomPolicies(b *strings.Builder, custom []SnapshotCustomPolicy) error {
	for _, c := range slices.SortedFunc(slices.Values(custom), func(x, y SnapshotCustomPolicy) int { return compareIDs(x.ID, y.ID) }) {
		list, err := cedar.NewPolicyListFromBytes("custom", []byte(c.Cedar))
		if err != nil {
			return fmt.Errorf("policy: custom policy %s: %w", c.ID, err)
		}
		for n, p := range list {
			if _, ok := p.Annotations()["id"]; ok {
				return fmt.Errorf("policy: custom policy %s: @id is reserved", c.ID)
			}
			fmt.Fprintf(b, "@id(\"custom:%s:%d\")\n%s\n\n", c.ID, n, p.MarshalCedar())
		}
	}
	return nil
}
