package app

import (
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"

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
}
