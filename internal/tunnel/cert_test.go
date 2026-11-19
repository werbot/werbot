package tunnel

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// ByCert finds users under "cert:<org> <user> <session> <login>".
func (f *fake) ByCert(_ context.Context, orgID uuid.UUID, id sshca.Identity) (User, error) {
	return f.lookup(orgID, fmt.Sprintf("cert:%s %s %s %s", id.OrgID, id.UserID, id.CLISessionID, id.Login))
}

func TestAuthenticateUserByCert(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	alice := User{ID: uuid.New(), Login: "alice"}
	session := uuid.New()
	b.f.with(func(f *fake) { f.users[fmt.Sprintf("cert:%s %s %s alice", b.org, alice.ID, session)] = alice })
	auth := func(c *gatewayv1.CertIdentity) error {
		_, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Cert{Cert: c}, ClientIp: "203.0.113.7"}))
		return err
	}

	resp, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_Cert{Cert: &gatewayv1.CertIdentity{
			UserId: alice.ID.String(), CliSessionId: session.String(), Login: "alice", CertFingerprint: "SHA256:cert"}},
		ClientIp: "203.0.113.7"}))
	if err != nil {
		t.Fatal(err)
	}
	if m := resp.Msg; m.GetUserId() != alice.ID.String() || m.GetLogin() != "alice" || m.GetCountry() != "NL" {
		t.Fatalf("response %+v", m)
	}
	b.f.with(func(f *fake) {
		if len(f.userOrgs) != 1 || f.userOrgs[0] != b.org {
			t.Fatalf("lookups used orgs %v, want the instance org %v", f.userOrgs, b.org)
		}
	})

	for _, c := range []*gatewayv1.CertIdentity{
		{UserId: "x", CliSessionId: session.String(), Login: "alice"},
		{UserId: alice.ID.String(), CliSessionId: "x", Login: "alice"},
		{UserId: alice.ID.String(), CliSessionId: session.String()}, // no login
	} {
		wantCode(t, auth(c), connect.CodeInvalidArgument, "")
	}
	wantCode(t, auth(&gatewayv1.CertIdentity{UserId: alice.ID.String(), CliSessionId: uuid.NewString(), Login: "alice"}),
		connect.CodeNotFound, "")
}
