package identity

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/identity/dbq"
)

// Store is the persistence the use cases need.
type Store interface {
	LockSetup(ctx context.Context, key int64) error
	HasUsers(ctx context.Context) (bool, error)
	CreateOrganization(ctx context.Context, name string) (uuid.UUID, error)
	CreateUser(ctx context.Context, arg dbq.CreateUserParams) (uuid.UUID, error)
	CreateMembership(ctx context.Context, arg dbq.CreateMembershipParams) error
	GetLoginUser(ctx context.Context, email string) (dbq.GetLoginUserRow, error)
	CreateSession(ctx context.Context, arg dbq.CreateSessionParams) error
	GetSessionByToken(ctx context.Context, tokenHash []byte) (dbq.GetSessionByTokenRow, error)
	ExtendSession(ctx context.Context, arg dbq.ExtendSessionParams) error
	DeleteSession(ctx context.Context, arg dbq.DeleteSessionParams) error
	GetSessionInfo(ctx context.Context, arg dbq.GetSessionInfoParams) (dbq.GetSessionInfoRow, error)
	CreateOneTimeToken(ctx context.Context, arg dbq.CreateOneTimeTokenParams) error
	ConsumeOneTimeToken(ctx context.Context, arg dbq.ConsumeOneTimeTokenParams) (dbq.ConsumeOneTimeTokenRow, error)
	GetResetUser(ctx context.Context, email string) (dbq.GetResetUserRow, error)
	UpdateUserPassword(ctx context.Context, arg dbq.UpdateUserPasswordParams) error
	GetUserOrg(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
	GetProfile(ctx context.Context, id uuid.UUID) (dbq.GetProfileRow, error)
	GetCertSubject(ctx context.Context, arg dbq.GetCertSubjectParams) (dbq.GetCertSubjectRow, error)
	UpdateProfile(ctx context.Context, arg dbq.UpdateProfileParams) error
	GetPasswordHash(ctx context.Context, id uuid.UUID) (string, error)
	DeleteOtherUserSessions(ctx context.Context, arg dbq.DeleteOtherUserSessionsParams) error
	VoidPasswordResetTokens(ctx context.Context, userID uuid.UUID) error
	LockOrgOwners(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error)
	LockUserOwnedOrgs(ctx context.Context, userID uuid.UUID) ([]dbq.LockUserOwnedOrgsRow, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	ListSSHKeys(ctx context.Context, userID uuid.UUID) ([]dbq.ListSSHKeysRow, error)
	CreateSSHKey(ctx context.Context, arg dbq.CreateSSHKeyParams) (dbq.CreateSSHKeyRow, error)
	UpdateSSHKey(ctx context.Context, arg dbq.UpdateSSHKeyParams) (dbq.UpdateSSHKeyRow, error)
	DeleteSSHKey(ctx context.Context, arg dbq.DeleteSSHKeyParams) (string, error)
	LookupSSHKeyUser(ctx context.Context, arg dbq.LookupSSHKeyUserParams) (dbq.LookupSSHKeyUserRow, error)
	LookupLoginUser(ctx context.Context, arg dbq.LookupLoginUserParams) (dbq.LookupLoginUserRow, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]dbq.ListMembersRow, error)
	ListInvites(ctx context.Context, orgID uuid.UUID) ([]dbq.ListInvitesRow, error)
	IsMemberEmail(ctx context.Context, arg dbq.IsMemberEmailParams) (bool, error)
	LockMemberRole(ctx context.Context, arg dbq.LockMemberRoleParams) (string, error)
	UpdateMemberRole(ctx context.Context, arg dbq.UpdateMemberRoleParams) error
	DeleteMembership(ctx context.Context, arg dbq.DeleteMembershipParams) error
	ListTeams(ctx context.Context, orgID uuid.UUID) ([]dbq.ListTeamsRow, error)
	TeamMemberIDs(ctx context.Context, arg dbq.TeamMemberIDsParams) ([]uuid.UUID, error)
	CreateTeam(ctx context.Context, arg dbq.CreateTeamParams) (dbq.CreateTeamRow, error)
	RenameTeam(ctx context.Context, arg dbq.RenameTeamParams) (dbq.RenameTeamRow, error)
	DeleteTeam(ctx context.Context, arg dbq.DeleteTeamParams) (string, error)
	AddTeamMember(ctx context.Context, arg dbq.AddTeamMemberParams) error
	RemoveTeamMember(ctx context.Context, arg dbq.RemoveTeamMemberParams) (int64, error)
	UserTeams(ctx context.Context, arg dbq.UserTeamsParams) ([]uuid.UUID, error)
	ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]dbq.ListOrgMembersRow, error)
	CreateOrgSetting(ctx context.Context, orgID uuid.UUID) error
	GetOrgSettings(ctx context.Context, orgID uuid.UUID) (dbq.GetOrgSettingsRow, error)
	LockOrgSettings(ctx context.Context, orgID uuid.UUID) (dbq.LockOrgSettingsRow, error)
	UpdateOrgSettings(ctx context.Context, arg dbq.UpdateOrgSettingsParams) error
	GetMemberRole(ctx context.Context, arg dbq.GetMemberRoleParams) (string, error)
	GetCLISessionRole(ctx context.Context, arg dbq.GetCLISessionRoleParams) (dbq.GetCLISessionRoleRow, error)
	ListCLISessions(ctx context.Context, arg dbq.ListCLISessionsParams) ([]dbq.ListCLISessionsRow, error)
	RevokeCLISession(ctx context.Context, arg dbq.RevokeCLISessionParams) (int64, error)
	RevokeUserCLISessions(ctx context.Context, arg dbq.RevokeUserCLISessionsParams) error
	GetUserCA(ctx context.Context, orgID uuid.UUID) (dbq.GetUserCARow, error)
	InsertUserCA(ctx context.Context, arg dbq.InsertUserCAParams) error
	LookupCertUser(ctx context.Context, arg dbq.LookupCertUserParams) (dbq.LookupCertUserRow, error)
	DeleteOldMFAChallenges(ctx context.Context, olderSecs float64) (int64, error)
	DeleteOldMFAVerifications(ctx context.Context, olderSecs float64) (int64, error)
	UpsertTOTPSecret(ctx context.Context, arg dbq.UpsertTOTPSecretParams) (int64, error)
	ConfirmTOTPSecret(ctx context.Context, userID uuid.UUID) (int64, error)
	GetTOTPSecret(ctx context.Context, userID uuid.UUID) (dbq.GetTOTPSecretRow, error)
	UseTOTPStep(ctx context.Context, arg dbq.UseTOTPStepParams) (int64, error)
	DeleteTOTPSecret(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteRecoveryCodes(ctx context.Context, userID uuid.UUID) (int64, error)
	InsertRecoveryCodes(ctx context.Context, arg dbq.InsertRecoveryCodesParams) error
	UseRecoveryCode(ctx context.Context, arg dbq.UseRecoveryCodeParams) (int64, error)
	DeleteWebAuthnCredential(ctx context.Context, arg dbq.DeleteWebAuthnCredentialParams) (int64, error)
	HasFactor(ctx context.Context, userID uuid.UUID) (bool, error)
	GetTOTPFactor(ctx context.Context, userID uuid.UUID) (time.Time, error)
	GetRecoveryFactor(ctx context.Context, userID uuid.UUID) (dbq.GetRecoveryFactorRow, error)
	ListWebAuthnFactors(ctx context.Context, userID uuid.UUID) ([]dbq.ListWebAuthnFactorsRow, error)
	GetRequireMFA(ctx context.Context, orgID uuid.UUID) (string, error)
}
