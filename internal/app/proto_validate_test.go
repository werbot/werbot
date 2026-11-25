package app

import (
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestRequestValidation(t *testing.T) {
	ok := &werbotv1.SetupRequest{Email: "a@example.com", Login: "alice", Name: "Alice", Password: "correct horse battery", OrganizationName: "Acme"}
	assert.NoError(t, protovalidate.Validate(ok))
	assert.Error(t, protovalidate.Validate(&werbotv1.SetupRequest{Email: "nope", Login: "Al+ce", Name: "A", Password: "short", OrganizationName: "Acme"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.InviteMemberRequest{Email: "a@example.com"})) // ROLE_UNSPECIFIED
	assert.NoError(t, protovalidate.Validate(&werbotv1.AcceptInviteRequest{Token: "t"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.AcceptInviteRequest{Token: "t", Login: "UPPER"}))

	// public, unauthenticated Enroll: bounded token and CSR
	enroll := func(token, csr int) *werbotv1.EnrollRequest {
		return &werbotv1.EnrollRequest{JoinToken: strings.Repeat("a", token), CsrDer: make([]byte, csr), Hostname: "gw"}
	}
	assert.NoError(t, protovalidate.Validate(enroll(256, 4096)))
	assert.Error(t, protovalidate.Validate(enroll(257, 1)))
	assert.Error(t, protovalidate.Validate(enroll(1, 4097)))

	// OAuthConsentService (stage 9)
	id := "0190a000-0000-7000-8000-000000000001"
	assert.NoError(t, protovalidate.Validate(&werbotv1.GetAuthRequestRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.GetAuthRequestRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ApproveAuthRequestRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ApproveAuthRequestRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DenyAuthRequestRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DenyAuthRequestRequest{Id: "x"}))
	long := strings.Repeat("B", 17)
	assert.NoError(t, protovalidate.Validate(&werbotv1.LookupDeviceCodeRequest{UserCode: "BCDF-GHJK"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.LookupDeviceCodeRequest{UserCode: long}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ApproveDeviceCodeRequest{UserCode: "BCDF-GHJK"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ApproveDeviceCodeRequest{UserCode: long}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DenyDeviceCodeRequest{UserCode: "BCDF-GHJK"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DenyDeviceCodeRequest{UserCode: long}))

	// CLISessionService (stage 9)
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListCLISessionsRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListCLISessionsRequest{UserId: &id}))
	bad := "x"
	assert.Error(t, protovalidate.Validate(&werbotv1.ListCLISessionsRequest{UserId: &bad}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.RevokeCLISessionRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.RevokeCLISessionRequest{Id: "x"}))

	// CertService and the tunnel's certificate login (stage 9)
	assert.NoError(t, protovalidate.Validate(&werbotv1.IssueUserCertRequest{PublicKey: "ssh-ed25519 AAAA"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.IssueUserCertRequest{}))
	assert.Error(t, protovalidate.Validate(&werbotv1.IssueUserCertRequest{PublicKey: strings.Repeat("a", 8193)}))
	cert := func(c *gatewayv1.CertIdentity) *gatewayv1.AuthenticateUserRequest {
		return &gatewayv1.AuthenticateUserRequest{By: &gatewayv1.AuthenticateUserRequest_Cert{Cert: c}, ClientIp: "192.0.2.7"}
	}
	assert.NoError(t, protovalidate.Validate(cert(&gatewayv1.CertIdentity{UserId: id, CliSessionId: id, Login: "alice", CertFingerprint: "SHA256:x"})))
	assert.Error(t, protovalidate.Validate(cert(&gatewayv1.CertIdentity{UserId: "x", CliSessionId: id, Login: "alice"})))
	assert.Error(t, protovalidate.Validate(cert(&gatewayv1.CertIdentity{UserId: id, CliSessionId: id})))
	assert.Error(t, protovalidate.Validate(cert(&gatewayv1.CertIdentity{UserId: id, CliSessionId: id, Login: "alice", CertFingerprint: strings.Repeat("a", 129)})))

	// UpdateGateway and ListAccessibleResources (stage 9)
	assert.NoError(t, protovalidate.Validate(&werbotv1.UpdateGatewayRequest{Id: id, SshAddress: "gw.example.com:2222"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.UpdateGatewayRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.UpdateGatewayRequest{Id: "x"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.UpdateGatewayRequest{Id: id, SshAddress: strings.Repeat("a", 301)}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListAccessibleResourcesRequest{}))

	// ApprovalPolicyService (stage 10)
	sel := &werbotv1.ResourceSelector{Labels: map[string]string{"env": "prod"}}
	create := func(mut func(*werbotv1.CreateApprovalPolicyRequest)) *werbotv1.CreateApprovalPolicyRequest {
		r := &werbotv1.CreateApprovalPolicyRequest{Name: "prod", Mode: "manual", ResourceSelector: sel, Requesters: &werbotv1.Subjects{AllUsers: true}}
		mut(r)
		return r
	}
	assert.NoError(t, protovalidate.Validate(create(func(*werbotv1.CreateApprovalPolicyRequest) {})))
	assert.NoError(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.MaxDuration = durationpb.New(14 * 24 * time.Hour) })))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.MaxDuration = durationpb.New(15 * 24 * time.Hour) })))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.MaxDuration = durationpb.New(30 * time.Second) })))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.Name = "" })))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.Name = strings.Repeat("a", 64) })))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.Mode = "x" })))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) {
		r.ResourceSelector = &werbotv1.ResourceSelector{ResourceIds: []string{"x"}}
	})))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) {
		r.ResourceSelector = &werbotv1.ResourceSelector{ResourceIds: make([]string, 101)}
	})))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) {
		r.ResourceSelector = &werbotv1.ResourceSelector{Labels: map[string]string{"Bad Key": "v"}}
	})))
	assert.Error(t, protovalidate.Validate(create(func(r *werbotv1.CreateApprovalPolicyRequest) { r.Approvers = &werbotv1.Subjects{Teams: []string{"x"}} })))
	assert.NoError(t, protovalidate.Validate(&werbotv1.UpdateApprovalPolicyRequest{Id: id, Name: "prod", Mode: "auto"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.UpdateApprovalPolicyRequest{Id: "x", Name: "prod", Mode: "auto"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeleteApprovalPolicyRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeleteApprovalPolicyRequest{Id: "x"}))

	// AccessRequestService (stage 10)
	long1025 := strings.Repeat("я", 1025)
	assert.NoError(t, protovalidate.Validate(&werbotv1.CreateRequestRequest{ResourceId: id, Reason: strings.Repeat("я", 1024)}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.CreateRequestRequest{LabelSelector: map[string]string{"env": "prod"}}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateRequestRequest{ResourceId: "x"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateRequestRequest{ResourceId: id, Reason: long1025}))
	nine := map[string]string{"a": "", "b": "", "c": "", "d": "", "e": "", "f": "", "g": "", "h": "", "i": ""}
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateRequestRequest{LabelSelector: nine}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListRequestsRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListRequestsRequest{Scope: werbotv1.RequestScope_REQUEST_SCOPE_ALL, Status: "pending", PageSize: 200}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ListRequestsRequest{PageSize: 201}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ListRequestsRequest{Status: "x"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ListRequestsRequest{Scope: werbotv1.RequestScope(9)}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.GetRequestRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.GetRequestRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ApproveRequest{Id: id, Comment: "ok"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ApproveRequest{Id: id, Comment: long1025}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DenyRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DenyRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.CancelRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CancelRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListGrantsRequest{UserId: &id, ResourceId: &id, Active: true}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ListGrantsRequest{ResourceId: &bad}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.RevokeGrantRequest{Id: id, Reason: "done"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.RevokeGrantRequest{Id: id, Reason: long1025}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ReviewBreakGlassRequest{GrantId: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.ReviewBreakGlassRequest{GrantId: "x"}))

	// LockService (stage 10)
	assert.NoError(t, protovalidate.Validate(&werbotv1.CreateLockRequest{TargetType: "cli_session", TargetId: id, Reason: "incident"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.CreateLockRequest{TargetType: "all_agents", Reason: "incident"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateLockRequest{TargetType: "x", TargetId: id, Reason: "incident"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateLockRequest{TargetType: "user", TargetId: "x", Reason: "incident"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateLockRequest{TargetType: "user", TargetId: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateLockRequest{TargetType: "user", TargetId: id, Reason: long1025}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeleteLockRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeleteLockRequest{Id: "x"}))

	// OrgSettings JIT fields (stage 10)
	settings := func(s *werbotv1.OrgSettings) *werbotv1.UpdateSettingsRequest {
		return &werbotv1.UpdateSettingsRequest{Settings: s, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"lock_mode"}}}
	}
	assert.NoError(t, protovalidate.Validate(settings(&werbotv1.OrgSettings{})))
	assert.NoError(t, protovalidate.Validate(settings(&werbotv1.OrgSettings{GrantDefaultTtl: durationpb.New(14 * 24 * time.Hour),
		GrantMaxTtl: durationpb.New(time.Minute), RequestPendingTtl: durationpb.New(24 * time.Hour),
		CommandApprovalTtl: durationpb.New(30 * time.Second), LockMode: "strict"})))
	for _, o := range []*werbotv1.OrgSettings{
		{GrantDefaultTtl: durationpb.New(0)}, {GrantDefaultTtl: durationpb.New(15 * 24 * time.Hour)},
		{GrantMaxTtl: durationpb.New(59 * time.Second)}, {GrantMaxTtl: durationpb.New(15 * 24 * time.Hour)},
		{RequestPendingTtl: durationpb.New(59 * time.Second)}, {RequestPendingTtl: durationpb.New(25 * time.Hour)},
		{CommandApprovalTtl: durationpb.New(29 * time.Second)}, {CommandApprovalTtl: durationpb.New(61 * time.Minute)},
		{LockMode: "x"},
	} {
		assert.Error(t, protovalidate.Validate(settings(o)), o.String())
	}

	// TunnelService command approvals and end reasons (stage 10)
	assert.NoError(t, protovalidate.Validate(&gatewayv1.RequestApprovalRequest{UserId: id, ResourceId: id, Command: "ls", Classes: []string{"destructive"}}))
	assert.Error(t, protovalidate.Validate(&gatewayv1.RequestApprovalRequest{UserId: id, ResourceId: id, Command: "ls", Reasons: []string{strings.Repeat("a", 257)}}))
	assert.NoError(t, protovalidate.Validate(&gatewayv1.GetApprovalRequest{ApprovalId: id}))
	assert.Error(t, protovalidate.Validate(&gatewayv1.GetApprovalRequest{ApprovalId: "x"}))
	assert.NoError(t, protovalidate.Validate(&gatewayv1.EndSessionRequest{SessionId: id, Reason: "policy_stale"}))

	// PolicyService command rules, class rules, custom policies and TestCommand (stage 11)
	rule := func(mut func(*werbotv1.CreateCommandRuleRequest)) *werbotv1.CreateCommandRuleRequest {
		r := &werbotv1.CreateCommandRuleRequest{Name: "no-rm", Subjects: &werbotv1.Subjects{AllUsers: true}, ResourceSelector: sel,
			Classes: []string{"destructive"}, Effect: "deny", Message: strings.Repeat("я", 256)}
		mut(r)
		return r
	}
	assert.NoError(t, protovalidate.Validate(rule(func(*werbotv1.CreateCommandRuleRequest) {})))
	for _, mut := range []func(*werbotv1.CreateCommandRuleRequest){
		func(r *werbotv1.CreateCommandRuleRequest) { r.Name = strings.Repeat("a", 64) },
		func(r *werbotv1.CreateCommandRuleRequest) { r.Classes = nil },
		func(r *werbotv1.CreateCommandRuleRequest) { r.Classes = []string{"bogus"} },
		func(r *werbotv1.CreateCommandRuleRequest) { r.Classes = []string{"read", "read"} },
		func(r *werbotv1.CreateCommandRuleRequest) { r.Effect = "maybe" },
		func(r *werbotv1.CreateCommandRuleRequest) { r.Message = strings.Repeat("я", 257) },
		func(r *werbotv1.CreateCommandRuleRequest) { r.Subjects = &werbotv1.Subjects{Users: []string{"x"}} },
	} {
		assert.Error(t, protovalidate.Validate(rule(mut)))
	}
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListCommandRulesRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.UpdateCommandRuleRequest{Id: id, Name: "no-rm", Classes: []string{"read"}, Effect: "allow"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.UpdateCommandRuleRequest{Id: "x", Name: "no-rm", Classes: []string{"read"}, Effect: "allow"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeleteCommandRuleRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeleteCommandRuleRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListClassRulesRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.CreateClassRuleRequest{Pattern: strings.Repeat("a", 255), Classes: []string{"write"}}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateClassRuleRequest{Pattern: strings.Repeat("a", 256), Classes: []string{"write"}}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateClassRuleRequest{Pattern: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeleteClassRuleRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeleteClassRuleRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListCustomPoliciesRequest{}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.CreateCustomPolicyRequest{Name: "p", Cedar: strings.Repeat("a", 65536)}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateCustomPolicyRequest{Name: "p", Cedar: strings.Repeat("a", 65537)}))
	assert.Error(t, protovalidate.Validate(&werbotv1.CreateCustomPolicyRequest{Name: "p"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.UpdateCustomPolicyRequest{Id: id, Name: "p", Cedar: "permit (principal, action, resource);"}))
	assert.Error(t, protovalidate.Validate(&werbotv1.UpdateCustomPolicyRequest{Id: id, Name: "", Cedar: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeleteCustomPolicyRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeleteCustomPolicyRequest{Id: "x"}))
	user := &werbotv1.TestCommandRequest_UserId{UserId: id}
	assert.NoError(t, protovalidate.Validate(&werbotv1.TestCommandRequest{Subject: user, ResourceId: id, Command: strings.Repeat("a", 65536)}))
	assert.Error(t, protovalidate.Validate(&werbotv1.TestCommandRequest{Subject: user, ResourceId: id, Command: strings.Repeat("a", 65537)}))
	assert.Error(t, protovalidate.Validate(&werbotv1.TestCommandRequest{Subject: user, ResourceId: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.TestCommandRequest{ResourceId: id, Command: "ls"}), "subject is required")
	assert.Error(t, protovalidate.Validate(&werbotv1.TestCommandRequest{Subject: &werbotv1.TestCommandRequest_AgentId{AgentId: "x"}, ResourceId: id, Command: "ls"}))
}
