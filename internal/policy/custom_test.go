package policy

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	customID  = uuid.MustParse("019a0000-0000-7000-8000-0000000000d1")
	custom2ID = uuid.MustParse("019a0000-0000-7000-8000-0000000000d2")
)

const approvalCedar = `@obligation("approval") @message("needs review")
forbid (principal, action == Werbot::Action::"ssh.exec", resource)
when { context.classes.contains("write") }
unless { context.approval_granted };`

func TestValidateCustomAccepts(t *testing.T) {
	for _, text := range []string{
		`forbid (principal, action == Werbot::Action::"ssh.exec", resource) when { context.classes.contains("egress") };`,
		approvalCedar,
	} {
		assert.NoError(t, ValidateCustom(text), text)
	}
}

func TestValidateCustomRejects(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"parse error", "permit (", "1:"},
		{"empty", "", "no policies"},
		{"comment only", "// nothing\n", "no policies"},
		{"reserved id", `@id("x") permit (principal, action, resource);`, "@id is reserved"},
		{"unknown obligation", `@obligation("mfa") forbid (principal, action, resource);`, `unknown obligation "mfa"`},
		{"obligation on permit", `@obligation("approval") permit (principal, action, resource);`, "only on forbid"},
		{"unknown context attribute", `forbid (principal, action, resource) when { context.clases.contains("write") };`, "attribute `clases`"},
		{"tags as attribute", `forbid (principal, action, resource) when { resource.tags["env"] == "prod" };`, "attribute `tags`"},
		{"tag without hasTag", `forbid (principal, action, resource) when { resource.getTag("env") == "prod" };`, "unable to guarantee safety"},
		{"evaluation error", `forbid (principal, action, resource) when { context.hour + 9223372036854775807 > 0 };`, "evaluation error at line 1"},
		{"second policy is checked", "permit (principal, action, resource);\n@id(\"x\") forbid (principal, action, resource);", "2:1: @id is reserved"},
		{"too large", strings.Repeat(" ", maxCustomPolicyBytes+1), "custom policy is too large"},
		{"message with a control character", `@message("a\nb") forbid (principal, action, resource);`, "message must be printable text"},
		{"message too long", `@message("` + strings.Repeat("é", 257) + `") forbid (principal, action, resource);`, "at most 256 characters"},
	}
	for _, c := range cases {
		err := ValidateCustom(c.text)
		require.Error(t, err, c.name)
		assert.Contains(t, err.Error(), c.want, c.name)
	}
}

func TestCompileCustomPolicies(t *testing.T) {
	s := snap()
	s.CustomPolicies = []SnapshotCustomPolicy{
		{ID: custom2ID, Cedar: `forbid (principal, action, resource) when { context.port == 22 };`},
		{ID: customID, Cedar: "permit (principal, action, resource) when { context.port == 1 };\n" +
			`forbid (principal, action, resource) when { context.port == 2 };`},
	}
	_, text, err := Compile(s)
	require.NoError(t, err)
	first := strings.Index(text, `@id("custom:`+customID.String()+`:0")`)
	second := strings.Index(text, `@id("custom:`+customID.String()+`:1")`)
	third := strings.Index(text, `@id("custom:`+custom2ID.String()+`:0")`)
	assert.True(t, first >= 0 && first < second && second < third, "policies by ID, then by order in the text")

	slices.Reverse(s.CustomPolicies)
	_, text2, err := Compile(s)
	require.NoError(t, err)
	assert.Equal(t, text, text2)

	for name, cedarText := range map[string]string{
		"reserved id": `@id("x") permit (principal, action, resource);`,
		"parse error": "permit (",
	} {
		s.CustomPolicies = []SnapshotCustomPolicy{{ID: customID, Cedar: cedarText}}
		_, _, err := Compile(s)
		assert.Error(t, err, name)
	}
}

func TestCustomPolicyEvalErrorDenies(t *testing.T) {
	s := snap()
	s.Resources[0].Labels = nil
	s.CustomPolicies = []SnapshotCustomPolicy{{ID: customID,
		Cedar: `forbid (principal, action, resource) when { resource.getTag("env") == "prod" };`}}
	d := authorize(t, s, req("10.1.2.3", "DE", ""))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "error:custom:"+customID.String()+":0")
}

func TestCustomForbidWithObligation(t *testing.T) {
	s := snap()
	s.CustomPolicies = []SnapshotCustomPolicy{{ID: customID, Cedar: approvalCedar}}
	d := authorize(t, s, execReq("write"))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{ObligationApproval}, d.Obligations)
	assert.Equal(t, "needs review", d.Message)
	assert.Equal(t, []string{"custom:" + customID.String() + ":0"}, d.Reasons)

	r := execReq("write")
	r.ApprovalGranted = true
	assert.True(t, authorize(t, s, r).Allow)
}
