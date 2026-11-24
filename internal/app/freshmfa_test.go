package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

func TestFreshMFAProcedures(t *testing.T) {
	want := []string{
		werbotv1connect.AccessRequestServiceApproveProcedure,
		werbotv1connect.AccessRequestServiceDenyProcedure,
		werbotv1connect.AccessRequestServiceRevokeGrantProcedure,
		werbotv1connect.AccessRequestServiceReviewBreakGlassProcedure,
		werbotv1connect.ApprovalPolicyServiceCreateApprovalPolicyProcedure,
		werbotv1connect.ApprovalPolicyServiceUpdateApprovalPolicyProcedure,
		werbotv1connect.ApprovalPolicyServiceDeleteApprovalPolicyProcedure,
		werbotv1connect.LockServiceCreateLockProcedure,
		werbotv1connect.LockServiceDeleteLockProcedure,
		werbotv1connect.PolicyServiceSetResourceRulesProcedure,
		werbotv1connect.PolicyServiceCreateCommandRuleProcedure,
		werbotv1connect.PolicyServiceUpdateCommandRuleProcedure,
		werbotv1connect.PolicyServiceDeleteCommandRuleProcedure,
		werbotv1connect.PolicyServiceCreateClassRuleProcedure,
		werbotv1connect.PolicyServiceDeleteClassRuleProcedure,
		werbotv1connect.PolicyServiceCreateCustomPolicyProcedure,
		werbotv1connect.PolicyServiceUpdateCustomPolicyProcedure,
		werbotv1connect.PolicyServiceDeleteCustomPolicyProcedure,
		werbotv1connect.PolicyServiceCreatePathRuleProcedure,
		werbotv1connect.PolicyServiceUpdatePathRuleProcedure,
		werbotv1connect.PolicyServiceDeletePathRuleProcedure,
		werbotv1connect.PolicyServiceCreateTerminalRuleProcedure,
		werbotv1connect.PolicyServiceDeleteTerminalRuleProcedure,
		werbotv1connect.PolicyServiceCreateMFARuleProcedure,
		werbotv1connect.PolicyServiceUpdateMFARuleProcedure,
		werbotv1connect.PolicyServiceDeleteMFARuleProcedure,
		werbotv1connect.AccessServiceCreateBindingProcedure,
		werbotv1connect.AccessServiceDeleteBindingProcedure,
		werbotv1connect.GatewayServiceCreateJoinTokenProcedure,
		werbotv1connect.GatewayServiceCreateGatewayProcedure,
		werbotv1connect.GatewayServiceDeleteGatewayProcedure,
		werbotv1connect.MemberServiceUpdateMemberRoleProcedure,
		werbotv1connect.MemberServiceRemoveMemberProcedure,
		werbotv1connect.MemberServiceInviteMemberProcedure,
		werbotv1connect.TeamServiceAddTeamMemberProcedure,
		werbotv1connect.TeamServiceRemoveTeamMemberProcedure,
		werbotv1connect.ResourceServiceSetCredentialProcedure,
		werbotv1connect.ResourceServiceClearCredentialProcedure,
		werbotv1connect.ResourceServiceClearHostKeyProcedure,
		werbotv1connect.SettingsServiceUpdateSettingsProcedure,
		werbotv1connect.IdentityProviderServiceCreateIdentityProviderProcedure,
		werbotv1connect.IdentityProviderServiceUpdateIdentityProviderProcedure,
		werbotv1connect.IdentityProviderServiceDeleteIdentityProviderProcedure,
	}
	got := freshMFAProcedures()
	assert.Len(t, got, len(want))
	for _, p := range want {
		assert.True(t, got[p], p)
	}

	// Every changing method of these services needs a fresh MFA; reads and dry runs do not.
	for _, svc := range []protoreflect.ServiceDescriptor{
		werbotv1.File_werbot_v1_approval_policy_proto.Services().ByName("ApprovalPolicyService"),
		werbotv1.File_werbot_v1_policy_proto.Services().ByName("PolicyService"),
		werbotv1.File_werbot_v1_identity_provider_proto.Services().ByName("IdentityProviderService"),
	} {
		methods := svc.Methods()
		for i := range methods.Len() {
			name := string(methods.Get(i).Name())
			proc := "/" + string(svc.FullName()) + "/" + name
			changes := false
			for _, prefix := range []string{"Create", "Update", "Delete", "Set"} {
				changes = changes || strings.HasPrefix(name, prefix)
			}
			assert.Equal(t, changes, got[proc], proc)
		}
	}
}

func assertDenied(t *testing.T, err error, text, name string) {
	t.Helper()
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), name)
	assert.ErrorContains(t, err, text, name)
}

// TestFreshMFAAfterAgentRestriction: served, a run token keeps the stage 12
// answer; the owner's CLI token and browser session need a fresh MFA, and a
// step-up of the CLI token opens the gate for that token.
func TestFreshMFAAfterAgentRestriction(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	run := r.runToken(t, "claude")
	// enrolled through the CLI token: an enrollment in the browser would make that session fresh
	secret := enrollTOTP(t, newMFAClients(&http.Client{Transport: headerTransport{"Authorization", "Bearer " + r.token}}, r.base))
	hc := http.DefaultClient
	reqs := werbotv1connect.NewAccessRequestServiceClient(hc, r.base)
	approve := func(token string) error {
		_, err := reqs.Approve(ctx, withBearer(&werbotv1.ApproveRequest{Id: uuid.NewString()}, token))
		return err
	}

	assertDenied(t, approve(run), "not allowed for agent tokens", "run token")
	assertDenied(t, approve(r.token), "mfa required", "CLI token")
	_, err := werbotv1connect.NewLockServiceClient(r.web, r.base).CreateLock(ctx,
		connect.NewRequest(&werbotv1.CreateLockRequest{TargetType: "all_agents", Reason: "incident"}))
	assertDenied(t, err, "mfa required", "browser session")

	_, err = werbotv1connect.NewAuthServiceClient(hc, r.base).CompleteStepUp(ctx,
		withBearer(&werbotv1.CompleteStepUpRequest{Proof: &werbotv1.CompleteStepUpRequest_TotpCode{TotpCode: totpAt(t, secret, 30*time.Second)}}, r.token))
	require.NoError(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(approve(r.token)), "the gate passes after the step-up")
	assertDenied(t, approve(run), "not allowed for agent tokens", "run token after the sponsor's step-up")
}

// TestEnrollmentGateAgentToken: with require_mfa all and a sponsor without a
// factor, a run token's allowed procedures answer "mfa enrollment required".
func TestEnrollmentGateAgentToken(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	run := r.runToken(t, "claude")
	_, err := r.pool.Exec(ctx, "UPDATE org_setting SET require_mfa = 'all'")
	require.NoError(t, err)

	_, err = werbotv1connect.NewResourceServiceClient(http.DefaultClient, r.base).ListAccessibleResources(ctx,
		withBearer(&werbotv1.ListAccessibleResourcesRequest{}, run))
	assertDenied(t, err, "mfa enrollment required", "ListAccessibleResources")
	assertDenied(t, r.issueCert(t, run), "mfa enrollment required", "IssueUserCert")
}
