package app

import "github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"

// freshMFAProcedures are the admin actions identity.RequireFreshMFA guards
// (contract §3.6, owner decision O4): access decisions, approval policies,
// locks, policy changes, bindings, gateways, membership, team membership,
// resource credentials and settings. A break-glass access request and the
// factor changes of MFAService are gated by the interceptor itself.
func freshMFAProcedures() map[string]bool {
	return map[string]bool{
		werbotv1connect.AccessRequestServiceApproveProcedure:               true,
		werbotv1connect.AccessRequestServiceDenyProcedure:                  true,
		werbotv1connect.AccessRequestServiceRevokeGrantProcedure:           true,
		werbotv1connect.AccessRequestServiceReviewBreakGlassProcedure:      true,
		werbotv1connect.ApprovalPolicyServiceCreateApprovalPolicyProcedure: true,
		werbotv1connect.ApprovalPolicyServiceUpdateApprovalPolicyProcedure: true,
		werbotv1connect.ApprovalPolicyServiceDeleteApprovalPolicyProcedure: true,
		werbotv1connect.LockServiceCreateLockProcedure:                     true,
		werbotv1connect.LockServiceDeleteLockProcedure:                     true,
		werbotv1connect.PolicyServiceSetResourceRulesProcedure:             true,
		werbotv1connect.PolicyServiceCreateCommandRuleProcedure:            true,
		werbotv1connect.PolicyServiceUpdateCommandRuleProcedure:            true,
		werbotv1connect.PolicyServiceDeleteCommandRuleProcedure:            true,
		werbotv1connect.PolicyServiceCreateClassRuleProcedure:              true,
		werbotv1connect.PolicyServiceDeleteClassRuleProcedure:              true,
		werbotv1connect.PolicyServiceCreateCustomPolicyProcedure:           true,
		werbotv1connect.PolicyServiceUpdateCustomPolicyProcedure:           true,
		werbotv1connect.PolicyServiceDeleteCustomPolicyProcedure:           true,
		werbotv1connect.PolicyServiceCreatePathRuleProcedure:               true,
		werbotv1connect.PolicyServiceUpdatePathRuleProcedure:               true,
		werbotv1connect.PolicyServiceDeletePathRuleProcedure:               true,
		werbotv1connect.PolicyServiceCreateTerminalRuleProcedure:           true,
		werbotv1connect.PolicyServiceDeleteTerminalRuleProcedure:           true,
		werbotv1connect.PolicyServiceCreateMFARuleProcedure:                true,
		werbotv1connect.PolicyServiceUpdateMFARuleProcedure:                true,
		werbotv1connect.PolicyServiceDeleteMFARuleProcedure:                true,
		werbotv1connect.AccessServiceCreateBindingProcedure:                true,
		werbotv1connect.AccessServiceDeleteBindingProcedure:                true,
		werbotv1connect.GatewayServiceCreateJoinTokenProcedure:             true,
		werbotv1connect.GatewayServiceCreateGatewayProcedure:               true,
		werbotv1connect.GatewayServiceDeleteGatewayProcedure:               true,
		werbotv1connect.MemberServiceUpdateMemberRoleProcedure:             true,
		werbotv1connect.MemberServiceRemoveMemberProcedure:                 true,
		werbotv1connect.MemberServiceInviteMemberProcedure:                 true,
		werbotv1connect.TeamServiceAddTeamMemberProcedure:                  true,
		werbotv1connect.TeamServiceRemoveTeamMemberProcedure:               true,
		werbotv1connect.ResourceServiceSetCredentialProcedure:              true,
		werbotv1connect.ResourceServiceClearCredentialProcedure:            true,
		werbotv1connect.ResourceServiceClearHostKeyProcedure:               true,
		werbotv1connect.SettingsServiceUpdateSettingsProcedure:             true,
	}
}
